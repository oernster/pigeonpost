package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// replayOutcome is what became of one attempt to replay a queued item.
type replayOutcome int

const (
	// replayLost means another replay holds the item (or it was cancelled or failed meanwhile): leave it.
	replayLost replayOutcome = iota
	// replayDelivered means the operation reached the server and the item is gone from the queue.
	replayDelivered
	// replayOffline means the server was unreachable; the item is queued again.
	replayOffline
	// replayFailed means the operation was refused; the item is queued again, stamped with the reason.
	replayFailed
	// replayAborted means the queue itself could not be updated, so the replay must stop.
	replayAborted
)

// RecoverOutbox returns to queued every item an earlier run of the app left mid-send (it crashed or
// was killed between claiming and finishing), reporting how many. Without it such an item would sit
// in the Outbox unsendable for ever. Re-queueing can resend a message whose server accepted it in the
// instant before the crash. The replay removes an item as soon as the server accepts it, which keeps
// that window to one database write; a lost message is the worse failure.
func (s *ComposeService) RecoverOutbox(ctx context.Context) (int, error) {
	released, err := s.outbox.RecoverOutboxClaims(ctx)
	if err != nil {
		return 0, fmt.Errorf("compose: recover interrupted outbox sends: %w", err)
	}
	return released, nil
}

// replayClaimed claims one item and performs it when the claim is won. A delivered item is removed from
// the queue before its Sent copy is saved, so the window in which a crash could leave a delivered
// message claimed (and so resent on recovery) is one write, not an IMAP upload. Any outcome short of
// delivery releases the claim, so no item is ever left stuck in sending by a live run. onOffline runs
// before that release for an unreachable server, letting a caller adjust the item first. For
// replayFailed the error is the item's own failure, for the caller to collect; for replayAborted it is
// the queue failure that stopped the replay.
func (s *ComposeService) replayClaimed(ctx context.Context, item domain.OutboxItem,
	onOffline func(id string) error) (replayOutcome, error) {
	id := item.ID()
	claimed, err := s.outbox.ClaimOutbox(ctx, id)
	if err != nil {
		return replayAborted, fmt.Errorf("compose: claim outbox item %q: %w", id, err)
	}
	if !claimed {
		return replayLost, nil
	}
	account, sendErr := s.performQueued(ctx, item)
	if sendErr == nil {
		if _, err := s.outbox.DeleteOutbox(ctx, id); err != nil {
			return replayAborted, fmt.Errorf("compose: remove replayed item %q: %w", id, err)
		}
		if item.Kind() != domain.OutboxDraft {
			// Delivered either way; a lost Sent copy is answered beside the outcome for the caller to
			// pass on, never as a failure.
			return replayDelivered, s.saveToSent(ctx, account, item.Message())
		}
		return replayDelivered, nil
	}
	outcome := replayFailed
	if domain.IsRetryLater(sendErr) {
		outcome = replayOffline
		if err := onOffline(id); err != nil {
			return replayAborted, err
		}
	} else if err := s.outbox.MarkOutboxFailed(ctx, id, sendErr.Error()); err != nil {
		return replayAborted, fmt.Errorf("compose: mark outbox item %q failed: %w", id, err)
	}
	if err := s.outbox.ReleaseOutbox(ctx, id); err != nil {
		return replayAborted, fmt.Errorf("compose: release outbox item %q: %w", id, err)
	}
	if outcome == replayOffline {
		return outcome, nil
	}
	return outcome, fmt.Errorf("compose: outbox item %q failed: %w", id, sendErr)
}

// keepOnOffline is the offline step for a plain replay: nothing to adjust, the item simply waits.
func keepOnOffline(string) error { return nil }
