package application

// The outbox half of ComposeService: the queue of outgoing operations (offline sends and drafts plus
// send-later holds), its replay paths and the dispatcher's queries. Kept apart from the compose and
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

// CancelOutbox discards a queued operation before it is sent. It reports whether the item was still
// queued: false means a replay had already claimed it (the message is on its way or has left) or it was
// already gone, so the caller tells the user the message could not be stopped rather than pretending
// it was. The queued test and the delete are one store operation, so no send can slip between them.
func (s *ComposeService) CancelOutbox(ctx context.Context, id string) (bool, error) {
	cancelled, err := s.outbox.CancelQueuedOutbox(ctx, id)
	if err != nil {
		return false, fmt.Errorf("compose: cancel outbox item %q: %w", id, err)
	}
	return cancelled, nil
}

// ReplayOutbox attempts every queued operation, oldest first, each only after winning its claim, so a
// replay racing another (a second account's sync, the dispatcher) skips what the other is sending. A
// successful operation is removed from the queue. If the server is still unreachable, replay stops and
// the remaining items stay queued. An operation that fails for any other reason (the account is gone,
// the message is rejected) is kept in the queue and stamped with its failure reason, so it surfaces in
// the outbox for the user to see and act on rather than vanishing; its error is also collected and
// returned. An item already marked failed is skipped rather than retried; so is an item still inside its
// hold: the user may yet cancel it, so no replay may send it early. It returns how many succeeded.
func (s *ComposeService) ReplayOutbox(ctx context.Context) (int, error) {
	items, err := s.OutboxItems(ctx)
	if err != nil {
		return 0, err
	}
	replayed := 0
	var failures []error
	for _, item := range items {
		if item.Failed() || item.HeldAt(s.clock.Now()) {
			continue
		}
		outcome, err := s.replayClaimed(ctx, item, keepOnOffline)
		switch outcome {
		case replayDelivered:
			replayed++
			if err != nil {
				failures = append(failures, err)
			}
		case replayOffline:
			return replayed, nil
		case replayFailed:
			failures = append(failures, err)
		case replayAborted:
			return replayed, err
		}
	}
	return replayed, errors.Join(failures...)
}

// ReplayDueHeld sends the held items whose hold has elapsed, returning how many were
// sent. It is the dispatcher's entry point, so it touches only held-and-due items: never the plain
// offline queue (which waits for a sync) and never an item still inside its window. Each send follows a
// won claim exactly as in ReplayOutbox, so a sync replaying the same due item cannot send it as well. A
// due item whose send finds the server unreachable has its hold cleared instead of being retried on
// every tick, degrading it to an ordinary queued item that the next sync replays; any other failure is
// stamped on the item exactly as in ReplayOutbox.
func (s *ComposeService) ReplayDueHeld(ctx context.Context) (int, error) {
	items, err := s.OutboxItems(ctx)
	if err != nil {
		return 0, err
	}
	clearHold := func(id string) error {
		if err := s.outbox.ClearOutboxHold(ctx, id); err != nil {
			return fmt.Errorf("compose: clear hold on %q: %w", id, err)
		}
		return nil
	}
	sent := 0
	var failures []error
	for _, item := range items {
		if item.Failed() || item.HoldUntil().IsZero() || item.HeldAt(s.clock.Now()) {
			continue
		}
		outcome, err := s.replayClaimed(ctx, item, clearHold)
		switch outcome {
		case replayDelivered:
			sent++
			if err != nil {
				failures = append(failures, err)
			}
		case replayFailed:
			failures = append(failures, err)
		case replayAborted:
			return sent, err
		}
	}
	return sent, errors.Join(failures...)
}

// NextHold returns when the earliest hold elapses and whether any held item exists, so the
// dispatcher can sleep until something is actually due.
func (s *ComposeService) NextHold(ctx context.Context) (time.Time, bool, error) {
	next, ok, err := s.outbox.NextOutboxHold(ctx)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("compose: next outbox hold: %w", err)
	}
	return next, ok, nil
}

// performQueued performs one queued operation against the server (dispatching on its kind) and returns
// the account it ran for. The best-effort Sent copy of a delivered send is left to the caller, which
// first takes the item off the queue (see replayClaimed); the copy still matches the record a direct
// send leaves.
func (s *ComposeService) performQueued(ctx context.Context, item domain.OutboxItem) (domain.Account, error) {
	account, err := s.accounts.GetAccount(ctx, item.AccountID())
	if err != nil {
		return domain.Account{}, fmt.Errorf("load account %q: %w", item.AccountID(), err)
	}
	switch item.Kind() {
	case domain.OutboxDraft:
		draftsPath, err := s.draftsPath(ctx, item.AccountID())
		if err != nil {
			return account, err
		}
		return account, s.drafts.SaveDraft(ctx, account, draftsPath, item.Message())
	default:
		return account, s.transport.Send(ctx, account, item.Message())
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
