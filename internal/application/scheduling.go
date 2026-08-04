package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Invitation is a scheduling message resolved for display: its method, the primary event, the account
// address it was received on and that address's own current response, so the reader can show the meeting
// and offer the right action.
type Invitation struct {
	Method   domain.Method
	Event    domain.Event
	Me       domain.EmailAddress
	MyStatus domain.ParticipationStatus
}

// SchedulingService is the use-case boundary for iTIP meeting scheduling (RFC 5546). On the attendee
// side it reads an incoming invite, replies to it with the recipient's answer and removes a cancelled
// meeting; on the organizer side it sends invites and cancellations and applies incoming replies to the
// stored meeting. Its sends leave the same record an ordinary message does: a copy in the Sent mailbox
// and, when the server is unreachable, a queued outbox item the dispatcher replays later.
type SchedulingService struct {
	codec     SchedulingCodec
	calendar  CalendarStore
	messages  MailStore
	accounts  AccountStore
	transport MailTransport
	sent      SentSaver
	outbox    OutboxStore
	clock     domain.Clock
	newID     IDGenerator
}

// NewSchedulingService constructs the service with its injected scheduling codec, calendar store, mail
// store, account store, transport, Sent-copy saver and offline outbox. The clock and id generator stamp
// queued outbox items, exactly as in ComposeService.
func NewSchedulingService(
	codec SchedulingCodec,
	calendar CalendarStore,
	messages MailStore,
	accounts AccountStore,
	transport MailTransport,
	sent SentSaver,
	outbox OutboxStore,
	clock domain.Clock,
	newID IDGenerator,
) *SchedulingService {
	return &SchedulingService{
		codec:     codec,
		calendar:  calendar,
		messages:  messages,
		accounts:  accounts,
		transport: transport,
		sent:      sent,
		outbox:    outbox,
		clock:     clock,
		newID:     newID,
	}
}

// Invitation resolves a message's scheduling payload for display, including the recipient's own current
// response. It returns ErrNoInvite when the message carries no calendar part. The email's ICS payload is
// frozen at the moment it arrived, so the attendee statuses are overlaid from the stored calendar copy
// of the meeting when one exists: that copy is where Respond records the recipient's answer and where
// ApplyReply folds in other attendees' responses, so it is the current truth the card must show.
func (s *SchedulingService) Invitation(ctx context.Context, messageID string) (Invitation, error) {
	sched, err := s.decodeInvite(ctx, messageID)
	if err != nil {
		return Invitation{}, err
	}
	account, err := s.accountForMessage(ctx, messageID)
	if err != nil {
		return Invitation{}, err
	}
	primary, err := s.overlayStoredStatuses(ctx, sched.PrimaryEvent())
	if err != nil {
		return Invitation{}, err
	}
	return Invitation{
		Method:   sched.Method(),
		Event:    primary,
		Me:       account.Address(),
		MyStatus: statusOf(primary, account.Address()),
	}, nil
}

// overlayStoredStatuses returns the invite event with each attendee's participation status replaced by
// the one on the stored calendar copy of the same meeting, matched by UID and recurrence id. An
// attendee the stored copy does not list, or a meeting not held locally, keeps the invite's own values.
func (s *SchedulingService) overlayStoredStatuses(ctx context.Context, event domain.Event) (domain.Event, error) {
	stored, err := s.calendar.ListEvents(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("scheduling: list meetings: %w", err)
	}
	for _, existing := range stored {
		if matches(existing, event) {
			return overlayAttendees(event, existing), nil
		}
	}
	return event, nil
}

// overlayAttendees copies each attendee's status from the stored event onto the invite event's matching
// attendee (by address, case-insensitively), leaving unmatched attendees untouched.
func overlayAttendees(event, stored domain.Event) domain.Event {
	attendees := event.Attendees()
	storedAttendees := stored.Attendees()
	for i, a := range attendees {
		for _, sa := range storedAttendees {
			if sameAddress(a.Address(), sa.Address()) {
				attendees[i] = a.WithStatus(sa.Status())
			}
		}
	}
	return event.WithAttendees(attendees)
}

// Respond records the recipient's answer to a meeting request: it saves the meeting to the calendar with
// the recipient's participation status set, then sends a REPLY to the organizer. It returns ErrNotInvitable
// when the message is not a REQUEST and ErrNoOrganizer when the meeting names no organizer to reply to.
func (s *SchedulingService) Respond(ctx context.Context, messageID string, status domain.ParticipationStatus) error {
	sched, err := s.decodeInvite(ctx, messageID)
	if err != nil {
		return err
	}
	if sched.Method() != domain.MethodRequest {
		return ErrNotInvitable
	}
	account, err := s.accountForMessage(ctx, messageID)
	if err != nil {
		return err
	}
	me := account.Address()
	primary := sched.PrimaryEvent()
	if !primary.HasOrganizer() {
		return ErrNoOrganizer
	}
	// Save every event in the invite (the series master plus any per-occurrence overrides) with the
	// recipient's own status set, so the meeting shows their answer in the calendar.
	for _, event := range sched.Events() {
		if err := s.calendar.SaveEvent(ctx, withStatus(event, me, status)); err != nil {
			return fmt.Errorf("scheduling: save meeting %q: %w", event.UID(), err)
		}
	}
	reply, err := s.codec.EncodeReply(primary, me, status)
	if err != nil {
		return fmt.Errorf("scheduling: build reply: %w", err)
	}
	return s.sendCalendar(ctx, account, []domain.EmailAddress{primary.Organizer().Address()},
		responseWord(status)+": "+primary.Summary(),
		me.Address()+" has "+strings.ToLower(responseWord(status))+" the meeting.",
		domain.MethodReply, reply)
}

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

// ApplyReply applies an incoming REPLY to the organizer's stored meeting, setting the responding
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

// ApplyIncoming folds a message's meeting scheduling into the calendar automatically, so the user does
// not have to open each reply or cancellation. A REPLY updates the responding attendee's status; a CANCEL
// removes the withdrawn meeting; a REQUEST or PUBLISH (which the user answers deliberately) and a message
// with no invite are left untouched. It reports whether the calendar changed. A reply for a meeting not
// held locally, or naming no attendee, is a harmless no-op rather than an error, since the poller applies
// replies blind across every arriving message.
func (s *SchedulingService) ApplyIncoming(ctx context.Context, messageID string) (bool, error) {
	sched, err := s.decodeInvite(ctx, messageID)
	if errors.Is(err, ErrNoInvite) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	switch sched.Method() {
	case domain.MethodReply:
		return appliedReply(s.ApplyReply(ctx, messageID))
	case domain.MethodCancel:
		if err := s.ApplyCancellation(ctx, messageID); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, nil
	}
}

// appliedReply maps an ApplyReply result to whether the calendar changed, treating a reply for a meeting
// not held locally or naming no attendee as a no-op rather than an error.
func appliedReply(err error) (bool, error) {
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrMeetingNotFound), errors.Is(err, ErrNoReplyAttendee):
		return false, nil
	default:
		return false, err
	}
}

// decodeInvite loads a message's cached body and decodes its scheduling payload. It returns ErrNoInvite
// when the message carries no calendar part.
func (s *SchedulingService) decodeInvite(ctx context.Context, messageID string) (domain.SchedulingMessage, error) {
	body, err := s.messages.GetMessageBody(ctx, messageID)
	if err != nil {
		return domain.SchedulingMessage{}, fmt.Errorf("scheduling: load body %q: %w", messageID, err)
	}
	if !body.HasInvite() {
		return domain.SchedulingMessage{}, ErrNoInvite
	}
	sched, err := s.codec.DecodeScheduling(body.Invite())
	if err != nil {
		return domain.SchedulingMessage{}, fmt.Errorf("scheduling: decode invite %q: %w", messageID, err)
	}
	return sched, nil
}

// accountForMessage resolves the account a message belongs to, through its folder.
func (s *SchedulingService) accountForMessage(ctx context.Context, messageID string) (domain.Account, error) {
	_, _, account, err := resolveMessageContext(ctx, s.messages, s.accounts, messageID)
	if err != nil {
		return domain.Account{}, fmt.Errorf("scheduling: %w", err)
	}
	return account, nil
}

// withStatus returns a copy of the event with the attendee matching who set to status. An event that
// does not list that address is returned with its attendees unchanged.
func withStatus(event domain.Event, who domain.EmailAddress, status domain.ParticipationStatus) domain.Event {
	attendees := event.Attendees()
	for i, a := range attendees {
		if sameAddress(a.Address(), who) {
			attendees[i] = a.WithStatus(status)
		}
	}
	return event.WithAttendees(attendees)
}

// statusOf returns the participation status of the attendee matching who, or NEEDS-ACTION when the event
// does not list that address.
func statusOf(event domain.Event, who domain.EmailAddress) domain.ParticipationStatus {
	for _, a := range event.Attendees() {
		if sameAddress(a.Address(), who) {
			return a.Status()
		}
	}
	return domain.PartStatNeedsAction
}

// matches reports whether two events are the same meeting occurrence: the same non-empty UID and the
// same recurrence id (both zero for a non-recurring meeting or a whole series).
func matches(a, b domain.Event) bool {
	return a.UID() != "" && a.UID() == b.UID() && a.RecurrenceID().Equal(b.RecurrenceID())
}

// sameAddress compares two addresses case-insensitively, since a mailbox address is not case-sensitive
// in practice.
func sameAddress(a, b domain.EmailAddress) bool {
	return strings.EqualFold(a.Address(), b.Address())
}
