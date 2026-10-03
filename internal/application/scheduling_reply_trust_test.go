package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// A REPLY speaks for one attendee, so it is applied only when the mail comes from that attendee's own
// address. A reply sent by anyone else (a delegate, a spoofed From) changes nothing and stays unread.

// guestReply is a REPLY in which guest@example.com declines meeting m1.
func guestReply(t *testing.T) domain.Event {
	t.Helper()
	reply := schedMeeting(t, "m1", schedOrganizer, time.Time{}, "guest@example.com")
	return withStatus(reply, schedAddr(t, "guest@example.com"), domain.PartStatDeclined)
}

func TestApplyIncomingReplyFromAnotherSenderChangesNothing(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, guestReply(t)))
	f.sendFrom(t, "stranger@evil.example")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, "guest@example.com")}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil): the mail stays unread", changed, resolved, err)
	}
	if len(f.calendar.savedEvt) != 0 {
		t.Errorf("saved %d events, want none: the sender does not speak for the attendee", len(f.calendar.savedEvt))
	}
}

func TestApplyReplyFromAnotherSenderReportsUntrusted(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, guestReply(t)))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, "guest@example.com")}

	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, ErrUntrustedReply) {
		t.Errorf("error = %v, want ErrUntrustedReply", err)
	}
}

func TestApplyReplyFromTheAttendeeMatchesCaseInsensitively(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, guestReply(t)))
	f.sendFrom(t, "Guest@Example.COM")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, "guest@example.com")}

	if err := f.svc.ApplyReply(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyReply: %v", err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Errorf("saved %d events, want 1", len(f.calendar.savedEvt))
	}
}

func TestApplyReplySenderLookupFailurePropagates(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, guestReply(t)))
	f.messages.getMessageErr = errBoom

	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}
