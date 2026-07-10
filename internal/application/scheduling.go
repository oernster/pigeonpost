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
// stored meeting.
type SchedulingService struct {
	codec     SchedulingCodec
	calendar  CalendarStore
	messages  MailStore
	accounts  AccountStore
	transport MailTransport
}

// NewSchedulingService constructs the service with its injected scheduling codec, calendar store, mail
// store, account store and transport.
func NewSchedulingService(
	codec SchedulingCodec,
	calendar CalendarStore,
	messages MailStore,
	accounts AccountStore,
	transport MailTransport,
) *SchedulingService {
	return &SchedulingService{codec: codec, calendar: calendar, messages: messages, accounts: accounts, transport: transport}
}

// Invitation resolves a message's scheduling payload for display, including the recipient's own current
// response. It returns ErrNoInvite when the message carries no calendar part.
func (s *SchedulingService) Invitation(ctx context.Context, messageID string) (Invitation, error) {
	sched, err := s.decodeInvite(ctx, messageID)
	if err != nil {
		return Invitation{}, err
	}
	account, err := s.accountForMessage(ctx, messageID)
	if err != nil {
		return Invitation{}, err
	}
	primary := sched.PrimaryEvent()
	return Invitation{
		Method:   sched.Method(),
		Event:    primary,
		Me:       account.Address(),
		MyStatus: statusOf(primary, account.Address()),
	}, nil
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
// attendee's participation status on the matching event. It returns ErrNotReply when the message is not a
// REPLY, ErrNoReplyAttendee when the reply names no attendee, and ErrMeetingNotFound when no stored
// meeting matches.
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
	for _, existing := range stored {
		if !matches(existing, reply) {
			continue
		}
		updated := withStatus(existing, responder.Address(), responder.Status())
		if err := s.calendar.SaveEvent(ctx, updated); err != nil {
			return fmt.Errorf("scheduling: update meeting %q: %w", existing.ID(), err)
		}
		return nil
	}
	return ErrMeetingNotFound
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

// SendRequest emails a meeting REQUEST to the attendees of the given events (the series master plus any
// overrides), inviting them.
func (s *SchedulingService) SendRequest(ctx context.Context, accountID string, events []domain.Event) error {
	return s.sendOrganizer(ctx, accountID, events, domain.MethodRequest)
}

// SendCancel emails a meeting CANCEL to the attendees of the given events, withdrawing the meeting.
func (s *SchedulingService) SendCancel(ctx context.Context, accountID string, events []domain.Event) error {
	return s.sendOrganizer(ctx, accountID, events, domain.MethodCancel)
}

// sendOrganizer builds the REQUEST or CANCEL payload for the events and emails it to the primary event's
// attendees from the given account.
func (s *SchedulingService) sendOrganizer(ctx context.Context, accountID string, events []domain.Event, method domain.Method) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("scheduling: load account %q: %w", accountID, err)
	}
	if len(events) == 0 {
		return domain.ErrNoSchedulingEvents
	}
	primary := events[0]
	var payload []byte
	if method == domain.MethodCancel {
		payload, err = s.codec.EncodeCancel(events)
	} else {
		payload, err = s.codec.EncodeRequest(events)
	}
	if err != nil {
		return fmt.Errorf("scheduling: build %s: %w", method, err)
	}
	return s.sendCalendar(ctx, account, attendeeAddresses(primary),
		organizerSubject(primary, method), organizerBody(primary, method), method, payload)
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

// sendCalendar wraps a scheduling payload as a text/calendar part on a new message and sends it.
func (s *SchedulingService) sendCalendar(ctx context.Context, account domain.Account, to []domain.EmailAddress, subject, body string, method domain.Method, payload []byte) error {
	part, err := domain.NewCalendarPart(method, payload)
	if err != nil {
		return fmt.Errorf("scheduling: build calendar part: %w", err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From: account.Address(), To: to, Subject: subject, Body: body, Calendar: part,
	})
	if err != nil {
		return fmt.Errorf("scheduling: build message: %w", err)
	}
	if err := s.transport.Send(ctx, account, msg); err != nil {
		return fmt.Errorf("scheduling: send %s: %w", method, err)
	}
	return nil
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

// attendeeAddresses returns the addresses of an event's attendees, the recipients of an organizer send.
func attendeeAddresses(event domain.Event) []domain.EmailAddress {
	attendees := event.Attendees()
	out := make([]domain.EmailAddress, 0, len(attendees))
	for _, a := range attendees {
		out = append(out, a.Address())
	}
	return out
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

// responseWord is the human word for a participation status, used in a reply's subject and body.
func responseWord(status domain.ParticipationStatus) string {
	switch status {
	case domain.PartStatAccepted:
		return "Accepted"
	case domain.PartStatDeclined:
		return "Declined"
	case domain.PartStatTentative:
		return "Tentative"
	default:
		return "Responded"
	}
}

// organizerSubject is the subject line for an organizer's REQUEST or CANCEL message.
func organizerSubject(event domain.Event, method domain.Method) string {
	if method == domain.MethodCancel {
		return "Cancelled: " + event.Summary()
	}
	return "Invitation: " + event.Summary()
}

// organizerBody is the human-readable body for an organizer's REQUEST or CANCEL message.
func organizerBody(event domain.Event, method domain.Method) string {
	if method == domain.MethodCancel {
		return "The meeting " + event.Summary() + " has been cancelled."
	}
	return "You are invited to " + event.Summary() + "."
}
