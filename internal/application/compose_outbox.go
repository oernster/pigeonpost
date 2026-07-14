package application

// The outbox half of ComposeService: the queue of outgoing operations (offline sends and drafts plus
// undo-send holds), its replay paths and the dispatcher's queries. Kept apart from the compose and
// draft flows in compose.go so each file stays within the module-size limit.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// PendingOutbox returns the number of operations currently queued for replay.
func (s *ComposeService) PendingOutbox(ctx context.Context) (int, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return 0, fmt.Errorf("compose: list outbox: %w", err)
	}
	return len(items), nil
}

// OutboxItems returns the queued outgoing operations, oldest first, so the user can review or cancel
// mail waiting to be sent.
func (s *ComposeService) OutboxItems(ctx context.Context) ([]domain.OutboxItem, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return nil, fmt.Errorf("compose: list outbox: %w", err)
	}
	return items, nil
}

// CancelOutbox removes a queued operation before it is sent, discarding it. It reports whether the
// item was still queued: false means it had already been sent (or removed), so an undo that lost the
// race can tell the user the message left rather than pretending it was stopped.
func (s *ComposeService) CancelOutbox(ctx context.Context, id string) (bool, error) {
	cancelled, err := s.outbox.DeleteOutbox(ctx, id)
	if err != nil {
		return false, fmt.Errorf("compose: cancel outbox item %q: %w", id, err)
	}
	return cancelled, nil
}

// ReplayOutbox attempts every queued operation, oldest first. A successful operation is removed from
// the queue. If the server is still unreachable, replay stops and the remaining items stay queued. An
// operation that fails for any other reason (the account is gone, the message is rejected) is kept in
// the queue and stamped with its failure reason, so it surfaces in the outbox for the user to see and
// act on rather than vanishing. An item already marked failed is skipped, not retried, and so is an
// item still inside its undo-send hold: the user may yet cancel it, so no replay may send it early.
// Its error is also collected and returned. It returns how many operations succeeded.
func (s *ComposeService) ReplayOutbox(ctx context.Context) (int, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return 0, fmt.Errorf("compose: list outbox: %w", err)
	}
	replayed := 0
	var failures []error
	for _, item := range items {
		if item.Failed() || item.HeldAt(s.clock.Now()) {
			continue
		}
		err := s.replayItem(ctx, item)
		if errors.Is(err, domain.ErrOffline) {
			return replayed, nil
		}
		if err != nil {
			if markErr := s.outbox.MarkOutboxFailed(ctx, item.ID(), err.Error()); markErr != nil {
				return replayed, fmt.Errorf("compose: mark outbox item %q failed: %w", item.ID(), markErr)
			}
			failures = append(failures, fmt.Errorf("compose: outbox item %q failed: %w", item.ID(), err))
			continue
		}
		replayed++
		if _, delErr := s.outbox.DeleteOutbox(ctx, item.ID()); delErr != nil {
			return replayed, fmt.Errorf("compose: remove replayed item %q: %w", item.ID(), delErr)
		}
	}
	return replayed, errors.Join(failures...)
}

// ReplayDueHeld sends the held items whose undo-send window has elapsed, returning how many were
// sent. It is the dispatcher's entry point, so it touches only held-and-due items: never the plain
// offline queue (which waits for a sync) and never an item still inside its window. A due item whose
// send finds the server unreachable has its hold cleared instead of being retried on every tick,
// degrading it to an ordinary queued item that the next sync replays; any other failure is stamped on
// the item exactly as in ReplayOutbox.
func (s *ComposeService) ReplayDueHeld(ctx context.Context) (int, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return 0, fmt.Errorf("compose: list outbox: %w", err)
	}
	sent := 0
	var failures []error
	for _, item := range items {
		if item.Failed() || item.HoldUntil().IsZero() || item.HeldAt(s.clock.Now()) {
			continue
		}
		err := s.replayItem(ctx, item)
		if errors.Is(err, domain.ErrOffline) {
			if clearErr := s.outbox.ClearOutboxHold(ctx, item.ID()); clearErr != nil {
				return sent, fmt.Errorf("compose: clear hold on %q: %w", item.ID(), clearErr)
			}
			continue
		}
		if err != nil {
			if markErr := s.outbox.MarkOutboxFailed(ctx, item.ID(), err.Error()); markErr != nil {
				return sent, fmt.Errorf("compose: mark outbox item %q failed: %w", item.ID(), markErr)
			}
			failures = append(failures, fmt.Errorf("compose: outbox item %q failed: %w", item.ID(), err))
			continue
		}
		sent++
		if _, delErr := s.outbox.DeleteOutbox(ctx, item.ID()); delErr != nil {
			return sent, fmt.Errorf("compose: remove sent item %q: %w", item.ID(), delErr)
		}
	}
	return sent, errors.Join(failures...)
}

// NextHold returns when the earliest undo-send hold elapses and whether any held item exists, so the
// dispatcher can sleep until something is actually due.
func (s *ComposeService) NextHold(ctx context.Context) (time.Time, bool, error) {
	next, ok, err := s.outbox.NextOutboxHold(ctx)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("compose: next outbox hold: %w", err)
	}
	return next, ok, nil
}

// replayItem performs one queued operation against the server, dispatching on its kind. A delivered
// send also gets its best-effort Sent copy here, so a queued message keeps the same record a direct
// send leaves.
func (s *ComposeService) replayItem(ctx context.Context, item domain.OutboxItem) error {
	account, err := s.accounts.GetAccount(ctx, item.AccountID())
	if err != nil {
		return fmt.Errorf("load account %q: %w", item.AccountID(), err)
	}
	switch item.Kind() {
	case domain.OutboxDraft:
		draftsPath, err := s.draftsPath(ctx, item.AccountID())
		if err != nil {
			return err
		}
		return s.drafts.SaveDraft(ctx, account, draftsPath, item.Message())
	default:
		if err := s.transport.Send(ctx, account, item.Message()); err != nil {
			return err
		}
		s.saveToSent(ctx, account, item.Message())
		return nil
	}
}

// enqueue records an outgoing operation in the outbox, stamped with a fresh id and the current time.
func (s *ComposeService) enqueue(ctx context.Context, accountID string, kind domain.OutboxKind, msg domain.OutgoingMessage) error {
	item, err := domain.NewOutboxItem(s.newID(), accountID, kind, msg, s.clock.Now())
	if err != nil {
		return fmt.Errorf("compose: build outbox item: %w", err)
	}
	if err := s.outbox.EnqueueOutbox(ctx, item); err != nil {
		return fmt.Errorf("compose: queue outbox item: %w", err)
	}
	return nil
}
