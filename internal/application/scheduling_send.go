package application

// The send half of SchedulingService: building and dispatching the iMIP messages the scheduling flows
// email out (the attendee's REPLY, the organizer's REQUEST and CANCEL). Kept apart from the read and
// apply flows in scheduling.go so each file stays within the module-size limit. Every send here leaves
// the same record an ordinary composed message does: a best-effort copy in the account's Sent mailbox,
// and a queued outbox item (replayed by the compose dispatcher) when the server is unreachable.

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

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

// sendCalendar wraps a scheduling payload as a text/calendar part on a new message and sends it. A
// delivered message gets its best-effort Sent copy; a server that is unreachable queues the message in
// the outbox instead of failing (mirroring ComposeService.Send), so the reply is delivered on the next
// replay rather than silently lost.
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
		if errors.Is(err, domain.ErrOffline) {
			return s.enqueue(ctx, account.ID(), msg)
		}
		return fmt.Errorf("scheduling: send %s: %w", method, err)
	}
	saveCopyToSent(ctx, s.messages, s.sent, account, msg)
	return nil
}

// enqueue records an undeliverable scheduling message in the offline outbox, stamped with a fresh id
// and the current time, for the compose dispatcher to replay once connectivity returns.
func (s *SchedulingService) enqueue(ctx context.Context, accountID string, msg domain.OutgoingMessage) error {
	item, err := domain.NewOutboxItem(s.newID(), accountID, domain.OutboxSend, msg, s.clock.Now())
	if err != nil {
		return fmt.Errorf("scheduling: build outbox item: %w", err)
	}
	if err := s.outbox.EnqueueOutbox(ctx, item); err != nil {
		return fmt.Errorf("scheduling: queue outbox item: %w", err)
	}
	return nil
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
