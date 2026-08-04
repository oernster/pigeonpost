package application

// The apply half of SchedulingService: folding incoming scheduling messages (REPLY, CANCEL and the
// attendee-status snapshot an updated REQUEST carries) into the stored calendar. Kept apart from the
// read and respond flows in scheduling.go so each file stays within the module-size limit.

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ApplyCancellation removes the meeting a CANCEL message withdraws from the calendar, matching stored
// events by UID and recurrence id. It returns ErrNotCancellation when the message is not a CANCEL. A
// cancellation for a meeting not held locally is a no-op.
func (s *SchedulingService) ApplyCancellation(ctx context.Context, messageID string) error {
	sched, err := s.decodeInvite(ctx, messageID)
	if err != nil {
		return err
	}
	if sched.Method() != domain.MethodCancel {
		return ErrNotCancellation
	}
	stored, err := s.calendar.ListEvents(ctx)
	if err != nil {
		return fmt.Errorf("scheduling: list meetings: %w", err)
	}
	for _, cancelled := range sched.Events() {
		for _, existing := range stored {
			if !matches(existing, cancelled) {
				continue
			}
			if err := s.calendar.DeleteEvent(ctx, existing.ID()); err != nil {
				return fmt.Errorf("scheduling: remove cancelled meeting %q: %w", existing.ID(), err)
			}
		}
	}
	return nil
}

// ApplyReply applies an incoming REPLY to the organiser's stored meeting, setting the responding
// attendee's participation status on every event the reply covers: the exact occurrence when the reply
// names one (RECURRENCE-ID), or the series master plus every stored override when it does not, since a
// whole-series reply is the attendee's latest word for all occurrences (RFC 5546). A responder the
// stored meeting does not list (a delegate, or a guest answering from a different address than the one
// invited) is added rather than dropped, so their response is never silently lost. It returns
// ErrNotReply when the message is not a REPLY, ErrNoReplyAttendee when the reply names no attendee,
// and ErrMeetingNotFound when no stored meeting matches.
func (s *SchedulingService) ApplyReply(ctx context.Context, messageID string) error {
	sched, err := s.decodeInvite(ctx, messageID)
	if err != nil {
		return err
	}
	if sched.Method() != domain.MethodReply {
		return ErrNotReply
	}
	reply := sched.PrimaryEvent()
	responders := reply.Attendees()
	if len(responders) == 0 {
		return ErrNoReplyAttendee
	}
	responder := responders[0]
	stored, err := s.calendar.ListEvents(ctx)
	if err != nil {
		return fmt.Errorf("scheduling: list meetings: %w", err)
	}
	applied := false
	for _, existing := range stored {
		if !replyCovers(existing, reply) {
			continue
		}
		if err := s.calendar.SaveEvent(ctx, withResponder(existing, responder)); err != nil {
			return fmt.Errorf("scheduling: update meeting %q: %w", existing.ID(), err)
		}
		applied = true
	}
	if !applied {
		return ErrMeetingNotFound
	}
	return nil
}

// ApplyIncoming folds a message's meeting scheduling into the calendar automatically, so the user does
// not have to open each message. A REPLY updates the responding attendee's status and a CANCEL removes
// the withdrawn meeting; both are resolved outright, needing nothing from the user. A REQUEST for a
// meeting already held locally has the organiser's attendee-status snapshot folded in, statuses of
// attendees other than the recipient only, never the meeting's content, because an attendee's reply
// travels only to the organiser, so an updated REQUEST is the sole channel through which one attendee
// learns that another accepted; the message itself is NOT resolved, since the update may carry changes
// the user must still act on. A first-time REQUEST (no stored meeting), a PUBLISH and a message with no
// invite are left untouched. It returns whether the calendar changed and whether the message was fully
// resolved (safe to mark read). A reply for a meeting not held locally, or naming no attendee, is a
// harmless no-op rather than an error, since the poller applies replies blind across every arriving
// message.
func (s *SchedulingService) ApplyIncoming(ctx context.Context, messageID string) (changed, resolved bool, err error) {
	sched, err := s.decodeInvite(ctx, messageID)
	if errors.Is(err, ErrNoInvite) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	switch sched.Method() {
	case domain.MethodReply:
		return appliedReply(s.ApplyReply(ctx, messageID))
	case domain.MethodCancel:
		if err := s.ApplyCancellation(ctx, messageID); err != nil {
			return false, false, err
		}
		return true, true, nil
	case domain.MethodRequest:
		changed, err := s.applyRequestStatuses(ctx, messageID, sched)
		return changed, false, err
	default:
		return false, false, nil
	}
}

// appliedReply maps an ApplyReply result to the ApplyIncoming return, treating a reply for a meeting
// not held locally or naming no attendee as a no-op rather than an error.
func appliedReply(err error) (bool, bool, error) {
	switch {
	case err == nil:
		return true, true, nil
	case errors.Is(err, ErrMeetingNotFound), errors.Is(err, ErrNoReplyAttendee):
		return false, false, nil
	default:
		return false, false, err
	}
}

// applyRequestStatuses folds the attendee-status snapshot an updated REQUEST carries into the stored
// copies of a meeting already held locally. Only statuses move: a status the snapshot leaves at
// NEEDS-ACTION never downgrades a recorded response, and the recipient's own row is never touched (the
// user's locally recorded answer outranks the organiser's possibly stale view of it). It reports
// whether any stored event changed.
func (s *SchedulingService) applyRequestStatuses(ctx context.Context, messageID string, sched domain.SchedulingMessage) (bool, error) {
	account, err := s.accountForMessage(ctx, messageID)
	if err != nil {
		return false, err
	}
	stored, err := s.calendar.ListEvents(ctx)
	if err != nil {
		return false, fmt.Errorf("scheduling: list meetings: %w", err)
	}
	changed := false
	for _, incoming := range sched.Events() {
		for _, existing := range stored {
			if !matches(existing, incoming) {
				continue
			}
			merged, moved := mergeStatuses(existing, incoming, account.Address())
			if !moved {
				continue
			}
			if err := s.calendar.SaveEvent(ctx, merged); err != nil {
				return changed, fmt.Errorf("scheduling: update meeting %q: %w", existing.ID(), err)
			}
			changed = true
		}
	}
	return changed, nil
}

// mergeStatuses copies each non-default attendee status from the incoming snapshot onto the stored
// event's matching attendee (by address, case-insensitively), skipping the recipient's own row. It
// returns the merged event and whether anything actually moved.
func mergeStatuses(existing, incoming domain.Event, me domain.EmailAddress) (domain.Event, bool) {
	attendees := existing.Attendees()
	incomingAttendees := incoming.Attendees()
	moved := false
	for i, a := range attendees {
		if sameAddress(a.Address(), me) {
			continue
		}
		for _, in := range incomingAttendees {
			if !sameAddress(a.Address(), in.Address()) {
				continue
			}
			if in.Status() == domain.PartStatNeedsAction || in.Status() == a.Status() {
				continue
			}
			attendees[i] = a.WithStatus(in.Status())
			moved = true
		}
	}
	if !moved {
		return existing, false
	}
	return existing.WithAttendees(attendees), true
}

// replyCovers reports whether a stored event is within a reply's reach: the same non-empty UID, and
// either the reply names that exact occurrence (matching RECURRENCE-ID) or it names none, in which
// case it covers the whole series (the master and every override).
func replyCovers(existing, reply domain.Event) bool {
	if reply.UID() == "" || existing.UID() != reply.UID() {
		return false
	}
	if reply.RecurrenceID().IsZero() {
		return true
	}
	return existing.RecurrenceID().Equal(reply.RecurrenceID())
}

// withResponder records a responder's participation status on the event: a listed attendee (matched by
// address, case-insensitively) has their status replaced; an unlisted one is appended as sent, so a
// delegate's or re-addressed reply still lands on the meeting.
func withResponder(event domain.Event, responder domain.Attendee) domain.Event {
	attendees := event.Attendees()
	for i, a := range attendees {
		if sameAddress(a.Address(), responder.Address()) {
			attendees[i] = a.WithStatus(responder.Status())
			return event.WithAttendees(attendees)
		}
	}
	return event.WithAttendees(append(attendees, responder))
}
