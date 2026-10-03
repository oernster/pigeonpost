package application

import (
	"context"
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
// meeting; on the organiser side it sends invites and cancellations and applies incoming replies to the
// stored meeting. Its sends leave the same record an ordinary message does: a copy in the Sent mailbox
// plus, when the server is unreachable, a queued outbox item the dispatcher replays later.
type SchedulingService struct {
	codec    SchedulingCodec
	calendar CalendarStore
	// sync records the CalDAV pending write when an answered meeting is held in a CalDAV calendar, so the
	// answer reaches the server on the next sync rather than living only in the local copy.
	sync      CalendarSyncStore
	messages  MailStore
	accounts  AccountStore
	transport MailTransport
	sent      SentSaver
	outbox    OutboxStore
	clock     domain.Clock
	newID     IDGenerator
}

// NewSchedulingService constructs the service with its injected scheduling codec, calendar store, CalDAV
// sync store, mail store, account store, transport, Sent-copy saver and offline outbox. The sync store is
// a required argument rather than an optional setter, so a wiring that forgets it fails to compile instead
// of silently saving every answered CalDAV meeting as a local one. The clock and id generator stamp
// queued outbox items, exactly as in ComposeService.
func NewSchedulingService(
	codec SchedulingCodec,
	calendar CalendarStore,
	sync CalendarSyncStore,
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
		sync:      sync,
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
// attendee the stored copy does not list keeps the invite's own values; so does a meeting not held locally.
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
// the recipient's participation status set, then sends a REPLY to the organiser. It returns ErrNotInvitable
// when the message is not a REQUEST and ErrNoOrganizer when the meeting names no organiser to reply to. An
// invitation that names a different organiser from the meeting already stored under its UID is refused
// with ErrUntrustedScheduling; one older than the stored meeting (a lower SEQUENCE) is refused with
// ErrStaleInvitation. So answering never saves a stranger's or an outdated copy over the meeting. The
// sender is not checked here, unlike the automatic apply: answering is the user's deliberate act and an
// invitation is routinely forwarded or relayed by a calendar service.
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
	stored, err := s.calendar.ListEvents(ctx)
	if err != nil {
		return fmt.Errorf("scheduling: list meetings: %w", err)
	}
	events := sched.Events()
	for _, event := range events {
		if !organizerAgrees(event, stored) {
			return ErrUntrustedScheduling
		}
		if superseded(event, stored) {
			return ErrStaleInvitation
		}
	}
	// Save every event in the invite (the series master plus any per-occurrence overrides) with the
	// recipient's own status set, so the meeting shows their answer in the calendar. Each is keyed by its
	// UID and RECURRENCE-ID so an override never overwrites its master. Each stays in the calendar its
	// meeting is held in; in a CalDAV calendar it also stays in the meeting's server object.
	for _, event := range events {
		if err := s.saveMeeting(ctx, storedKey(withStatus(event, me, status), stored), stored); err != nil {
			return err
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

// statusOf returns the participation status of the attendee matching who. When the event does not list
// that address it returns NEEDS-ACTION.
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

// sameAddress compares two addresses by their mailboxKey: case-insensitively, trimmed and without a
// mailto: scheme, since a mailbox address is not case-sensitive in practice.
func sameAddress(a, b domain.EmailAddress) bool {
	return mailboxKey(a) == mailboxKey(b)
}
