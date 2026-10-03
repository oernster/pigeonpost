package application

// The trust and revision rules SchedulingService applies before an incoming scheduling message may change a
// stored meeting. RFC 5546 makes withdrawing (CANCEL) and revising (REQUEST) a meeting the organiser's
// alone; it orders revisions by SEQUENCE; mail is not authenticated, so the From line is the evidence the
// message carries of who sent it. Kept apart from the apply flows so each rule has one home.

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// mailtoScheme is the URI scheme an iCalendar CAL-ADDRESS carries (ORGANIZER:mailto:chair@example.com).
// The codec strips it on decode; mailboxKey strips it again so an address that kept it still compares.
const mailtoScheme = "mailto:"

// mailboxKey is the comparable form of an address: trimmed, without a mailto: scheme and lower-cased, since
// a mailbox address is not case-sensitive in practice. The zero address yields the empty string.
func mailboxKey(a domain.EmailAddress) string {
	addr := strings.TrimSpace(a.Address())
	if len(addr) >= len(mailtoScheme) && strings.EqualFold(addr[:len(mailtoScheme)], mailtoScheme) {
		addr = addr[len(mailtoScheme):]
	}
	return strings.ToLower(strings.TrimSpace(addr))
}

// organizerAgrees reports whether every stored event of the incoming event's series records the same
// organiser the incoming event names. A meeting not held locally has nothing to disagree with.
func organizerAgrees(incoming domain.Event, stored []domain.Event) bool {
	for _, e := range stored {
		if sameSeries(e, incoming) && !sameAddress(e.Organizer().Address(), incoming.Organizer().Address()) {
			return false
		}
	}
	return true
}

// organizerTrusted reports whether an organiser's message may change the stored meeting: it names an
// organiser, the mail came from that address and the stored meeting records the same organiser.
func organizerTrusted(incoming domain.Event, sender domain.EmailAddress, stored []domain.Event) bool {
	organizer := incoming.Organizer().Address()
	if mailboxKey(organizer) == "" || !sameAddress(organizer, sender) {
		return false
	}
	return organizerAgrees(incoming, stored)
}

// superseded reports whether the stored copy of the incoming event's occurrence carries a higher SEQUENCE,
// meaning the organiser has revised the meeting since this message was sent. An equal SEQUENCE is the
// same revision (an attendee-status refresh; the same invitation read twice) and is not superseded.
func superseded(incoming domain.Event, stored []domain.Event) bool {
	for _, e := range stored {
		if matches(e, incoming) && e.Sequence() > incoming.Sequence() {
			return true
		}
	}
	return false
}

// sameSeries reports whether two events belong to the same meeting: the same non-empty UID, whatever
// occurrence each one is.
func sameSeries(a, b domain.Event) bool {
	return a.UID() != "" && a.UID() == b.UID()
}

// senderOf returns the From address of a cached message, the evidence of who sent a scheduling message.
func (s *SchedulingService) senderOf(ctx context.Context, messageID string) (domain.EmailAddress, error) {
	msg, err := s.messages.GetMessage(ctx, messageID)
	if err != nil {
		return domain.EmailAddress{}, fmt.Errorf("scheduling: locate message %q: %w", messageID, err)
	}
	return msg.From(), nil
}

// checkOrganizerMessage applies the trust and revision rules to every event of an organiser's message,
// before anything is changed, so a message is applied whole or not at all.
func checkOrganizerMessage(events []domain.Event, sender domain.EmailAddress, stored []domain.Event) error {
	for _, e := range events {
		if !organizerTrusted(e, sender, stored) {
			return ErrUntrustedScheduling
		}
		if superseded(e, stored) {
			return ErrStaleInvitation
		}
	}
	return nil
}

// storedKey returns the event keyed for saving: under the id and calendar of the stored copy of the same
// occurrence when one is held (so it is updated in place and stays in its calendar). An occurrence not
// held yet takes its own UID and RECURRENCE-ID as its id and joins the calendar its series is held in.
func storedKey(event domain.Event, stored []domain.Event) domain.Event {
	if held, ok := heldRow(event, stored); ok {
		event = event.WithCalendarID(held.CalendarID())
		if matches(held, event) {
			return event.WithID(held.ID())
		}
	}
	return keyedEvent(event)
}

// heldRow returns the stored row an incoming event belongs with: the stored copy of the same occurrence
// when there is one, otherwise any stored row of its series.
func heldRow(event domain.Event, stored []domain.Event) (domain.Event, bool) {
	var sibling domain.Event
	found := false
	for _, e := range stored {
		if matches(e, event) {
			return e, true
		}
		if !found && sameSeries(e, event) {
			sibling, found = e, true
		}
	}
	return sibling, found
}

// saveMeeting saves an answered meeting. A meeting whose series is held in a CalDAV calendar is saved with
// the pending update a later sync pushes (If-Match on the object's last-seen etag), the same intent a
// local edit of a CalDAV event records (see CalendarEditService.SaveEvent); any other meeting is saved
// as a local one.
func (s *SchedulingService) saveMeeting(ctx context.Context, event domain.Event, stored []domain.Event) error {
	if held, ok := heldRow(event, stored); ok {
		identity, found, err := s.sync.SyncedEventIdentity(ctx, held.ID())
		if err != nil {
			return fmt.Errorf("scheduling: meeting %q identity: %w", event.ID(), err)
		}
		if found && identity.Href != "" {
			op := PendingCalendarObject{CalendarID: identity.CalendarID, Href: identity.Href, Op: CalendarOpUpdate, BaseETag: identity.ETag}
			if err := s.sync.SaveEventWithPending(ctx, event, identity.Href, identity.ETag, op); err != nil {
				return fmt.Errorf("scheduling: save meeting %q: %w", event.ID(), err)
			}
			return nil
		}
	}
	if err := s.calendar.SaveEvent(ctx, event); err != nil {
		return fmt.Errorf("scheduling: save meeting %q: %w", event.ID(), err)
	}
	return nil
}
