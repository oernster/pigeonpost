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

// ErrUntrustedScheduling reports a CANCEL or REQUEST that may not change the stored meeting: it names no
// organiser; it names a different one from the stored meeting's; it came in mail not sent by that organiser. RFC 5546 makes withdrawing and revising a meeting the organiser's alone.
var ErrUntrustedScheduling = errors.New("scheduling: the message does not come from the meeting's organiser")

// ErrUntrustedReply reports a REPLY that was not sent from the address of the attendee it answers for, so
// it may not change that attendee's status on the stored meeting.
var ErrUntrustedReply = errors.New("scheduling: the reply does not come from the attendee it answers for")

// ErrStaleInvitation reports an invitation older than the stored meeting: its SEQUENCE is lower than the
// one already held, so the organiser has revised the meeting since and this copy must not replace it.
var ErrStaleInvitation = errors.New("scheduling: the invitation is older than the meeting already in the calendar")

// ApplyCancellation removes the meeting a CANCEL message withdraws from the calendar. It returns
// ErrNotCancellation when the message is not a CANCEL, ErrUntrustedScheduling when the CANCEL does not come
// from the stored meeting's organiser (see organizerTrusted) and ErrStaleInvitation when the stored meeting
// is a later revision; in those cases nothing is changed. A cancellation for a meeting not held locally is
// a no-op.
func (s *SchedulingService) ApplyCancellation(ctx context.Context, messageID string) error {
	_, err := s.applyCancellation(ctx, messageID)
	return err
}

// applyCancellation is ApplyCancellation reporting whether the calendar changed. Every cancelled event is
// checked before any is applied, so an untrusted message changes nothing at all.
func (s *SchedulingService) applyCancellation(ctx context.Context, messageID string) (bool, error) {
	sched, err := s.decodeInvite(ctx, messageID)
	if err != nil {
		return false, err
	}
	if sched.Method() != domain.MethodCancel {
		return false, ErrNotCancellation
	}
	sender, err := s.senderOf(ctx, messageID)
	if err != nil {
		return false, err
	}
	stored, err := s.calendar.ListEvents(ctx)
	if err != nil {
		return false, fmt.Errorf("scheduling: list meetings: %w", err)
	}
	if err := checkOrganizerMessage(sched.Events(), sender, stored); err != nil {
		return false, err
	}
	changed := false
	for _, cancelled := range sched.Events() {
		moved, err := s.withdraw(ctx, cancelled, stored)
		if err != nil {
			return changed, err
		}
		changed = changed || moved
	}
	return changed, nil
}

// withdraw removes one cancelled event from the stored calendar and reports whether anything changed. A
// CANCEL naming no occurrence withdraws the whole series: the master and every stored override. One naming
// an occurrence drops that occurrence's stored override and excludes it from the master's recurrence set
// (EXDATE), since deleting the override alone would let the master's rule regenerate the occurrence.
func (s *SchedulingService) withdraw(ctx context.Context, cancelled domain.Event, stored []domain.Event) (bool, error) {
	changed := false
	for _, existing := range stored {
		whole := cancelled.RecurrenceID().IsZero() && sameSeries(existing, cancelled)
		if !whole && !matches(existing, cancelled) {
			continue
		}
		if err := s.calendar.DeleteEvent(ctx, existing.ID()); err != nil {
			return changed, fmt.Errorf("scheduling: remove cancelled meeting %q: %w", existing.ID(), err)
		}
		changed = true
	}
	if cancelled.RecurrenceID().IsZero() {
		return changed, nil
	}
	excluded, err := s.excludeOccurrence(ctx, cancelled, stored)
	return changed || excluded, err
}

// excludeOccurrence adds a cancelled occurrence to its stored recurring master's EXDATEs, reporting whether
// the master changed. A series not held, a master that does not recur or an occurrence already excluded is
// left as it is.
func (s *SchedulingService) excludeOccurrence(ctx context.Context, cancelled domain.Event, stored []domain.Event) (bool, error) {
	occurrence := cancelled.RecurrenceID()
	for _, master := range stored {
		if !sameSeries(master, cancelled) || master.IsOverride() || !master.IsRecurring() {
			continue
		}
		exdates := master.ExDates()
		for _, ex := range exdates {
			if ex.Equal(occurrence) {
				return false, nil
			}
		}
		if err := s.calendar.SaveEvent(ctx, master.WithExDates(append(exdates, occurrence))); err != nil {
			return false, fmt.Errorf("scheduling: exclude cancelled occurrence of %q: %w", master.ID(), err)
		}
		return true, nil
	}
	return false, nil
}

// ApplyReply applies an incoming REPLY to the organiser's stored meeting, setting the responding
// attendee's participation status on every event the reply covers: the exact occurrence when the reply
// names one (RECURRENCE-ID); otherwise the series master plus every stored override, since a
// whole-series reply is the attendee's latest word for all occurrences (RFC 5546). A reply speaks for
// one attendee, so it is applied only when the mail comes from that attendee's own address; one sent by
// anyone else (a delegate; a spoofed From) returns ErrUntrustedReply and changes nothing. A genuine
// responder the stored meeting does not list (a guest answering from an address other than the one
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
	sender, err := s.senderOf(ctx, messageID)
	if err != nil {
		return err
	}
	if !sameAddress(responder.Address(), sender) {
		return ErrUntrustedReply
	}
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
// the withdrawn meeting; both are resolved outright when they change the calendar, needing nothing from
// the user. A CANCEL that is not from the meeting's organiser, is older than the stored meeting or
// changes nothing is left unresolved, so the mail stays unread for the user to judge. A REQUEST for a
// meeting already held locally has the organiser's attendee-status snapshot folded in, statuses of
// attendees other than the recipient only, never the meeting's content, because an attendee's reply
// travels only to the organiser, so an updated REQUEST is the sole channel through which one attendee
// learns that another accepted; the message itself is NOT resolved, since the update may carry changes
// the user must still act on. A first-time REQUEST (no stored meeting), a PUBLISH and a message with no
// invite are left untouched. It returns whether the calendar changed and whether the message was fully
// resolved (safe to mark read). A reply for a meeting not held locally is a harmless no-op rather than an
// error, as is one naming no attendee, since the poller applies replies blind across every arriving message.
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
		changed, err := s.applyCancellation(ctx, messageID)
		if errors.Is(err, ErrUntrustedScheduling) || errors.Is(err, ErrStaleInvitation) {
			return false, false, nil
		}
		if err != nil {
			return false, false, err
		}
		return changed, changed, nil
	case domain.MethodRequest:
		changed, err := s.applyRequestStatuses(ctx, messageID, sched)
		return changed, false, err
	default:
		return false, false, nil
	}
}

// appliedReply maps an ApplyReply result to the ApplyIncoming return. A reply for a meeting not held
// locally is a no-op rather than an error; so is one naming no attendee or sent by someone other than the
// attendee it speaks for. None of these is resolved, so the mail stays unread for the user.
func appliedReply(err error) (bool, bool, error) {
	switch {
	case err == nil:
		return true, true, nil
	case errors.Is(err, ErrMeetingNotFound), errors.Is(err, ErrNoReplyAttendee), errors.Is(err, ErrUntrustedReply):
		return false, false, nil
	default:
		return false, false, err
	}
}

// applyRequestStatuses folds the attendee-status snapshot an updated REQUEST carries into the stored
// copies of a meeting already held locally. Only statuses move: a status the snapshot leaves at
// NEEDS-ACTION never downgrades a recorded response; the recipient's own row is never touched (the
// user's locally recorded answer outranks the organiser's possibly stale view of it). The snapshot is the
// organiser's to give, so an update not from the stored meeting's organiser is ignored; so is one older
// than the stored meeting (see checkOrganizerMessage). It reports whether any stored event changed.
func (s *SchedulingService) applyRequestStatuses(ctx context.Context, messageID string, sched domain.SchedulingMessage) (bool, error) {
	msg, _, account, err := resolveMessageContext(ctx, s.messages, s.accounts, messageID)
	if err != nil {
		return false, fmt.Errorf("scheduling: %w", err)
	}
	stored, err := s.calendar.ListEvents(ctx)
	if err != nil {
		return false, fmt.Errorf("scheduling: list meetings: %w", err)
	}
	changed := false
	for _, incoming := range sched.Events() {
		if checkOrganizerMessage([]domain.Event{incoming}, msg.From(), stored) != nil {
			continue
		}
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

// replyCovers reports whether a stored event is within a reply's reach: the same non-empty UID plus
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
