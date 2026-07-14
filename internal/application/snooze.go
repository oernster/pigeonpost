package application

import (
	"context"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// SnoozeService is the use-case boundary for hiding a message until a chosen instant and bringing it
// back. Snooze is local-only state: nothing reaches the server, the message's read and flag state are
// never touched, and a hidden message simply drops out of the visible listings until it comes due.
type SnoozeService struct {
	snoozes SnoozeStore
	clock   domain.Clock
}

// NewSnoozeService constructs the service with its injected store and clock.
func NewSnoozeService(snoozes SnoozeStore, clock domain.Clock) *SnoozeService {
	return &SnoozeService{snoozes: snoozes, clock: clock}
}

// Snooze hides a message until the given instant, replacing any snooze it already carries. An instant
// that is not in the future is rejected, so a stale picker can never hide a message that would
// resurface immediately.
func (s *SnoozeService) Snooze(ctx context.Context, messageID string, until time.Time) error {
	if !until.After(s.clock.Now()) {
		return ErrSnoozeInPast
	}
	if err := s.snoozes.SetSnooze(ctx, messageID, until); err != nil {
		return fmt.Errorf("snooze: hide message %q: %w", messageID, err)
	}
	return nil
}

// Unsnooze brings a hidden message back at once. Unsnoozing a message that carries no snooze is a
// no-op.
func (s *SnoozeService) Unsnooze(ctx context.Context, messageID string) error {
	if err := s.snoozes.ClearSnooze(ctx, messageID); err != nil {
		return fmt.Errorf("snooze: unhide message %q: %w", messageID, err)
	}
	return nil
}

// Snoozed returns every hidden message with its due instant, soonest first, for the Snoozed view.
func (s *SnoozeService) Snoozed(ctx context.Context) ([]SnoozedMessage, error) {
	snoozed, err := s.snoozes.ListSnoozed(ctx)
	if err != nil {
		return nil, fmt.Errorf("snooze: list snoozed: %w", err)
	}
	return snoozed, nil
}

// PopDue removes every snooze whose instant has passed and returns the messages that just resurfaced,
// for the scheduler to announce. Idempotent: a second call finds nothing left to pop.
func (s *SnoozeService) PopDue(ctx context.Context) ([]domain.MessageSummary, error) {
	resurfaced, err := s.snoozes.PopDueSnoozed(ctx, s.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("snooze: pop due: %w", err)
	}
	return resurfaced, nil
}

// NextDue returns the earliest pending snooze instant and whether one exists, so the resurface
// scheduler only queries further when something can actually be due.
func (s *SnoozeService) NextDue(ctx context.Context) (time.Time, bool, error) {
	next, ok, err := s.snoozes.NextSnooze(ctx)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("snooze: next due: %w", err)
	}
	return next, ok, nil
}
