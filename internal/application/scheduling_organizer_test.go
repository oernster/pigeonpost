package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// An attendee's account must never send a REQUEST or CANCEL for someone else's meeting: doing so emails
// every other attendee an invitation from the wrong person. These pin the refusal for both methods and
// for a meeting that names no organiser at all; each proves nothing reaches the transport or the outbox.

func TestSendRequestRefusedWhenAccountIsNotOrganizer(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})

	if err := f.svc.SendRequest(context.Background(), "a1", []domain.Event{event}); !errors.Is(err, ErrNotOrganizer) {
		t.Fatalf("error = %v, want ErrNotOrganizer", err)
	}
	assertNothingSent(t, f)
}

func TestSendCancelRefusedWhenAccountIsNotOrganizer(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})

	if err := f.svc.SendCancel(context.Background(), "a1", []domain.Event{event}); !errors.Is(err, ErrNotOrganizer) {
		t.Fatalf("error = %v, want ErrNotOrganizer", err)
	}
	assertNothingSent(t, f)
}

func TestSendRequestRefusedWhenMeetingHasNoOrganizer(t *testing.T) {
	event := schedMeeting(t, "m1", "", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})

	if err := f.svc.SendRequest(context.Background(), "a1", []domain.Event{event}); !errors.Is(err, ErrNotOrganizer) {
		t.Fatalf("error = %v, want ErrNotOrganizer", err)
	}
	assertNothingSent(t, f)
}

func TestSendRequestOrganizerMatchIgnoresCase(t *testing.T) {
	event := schedMeeting(t, "m1", "USER@Example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})

	if err := f.svc.SendRequest(context.Background(), "a1", []domain.Event{event}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if len(f.transport.sent) != 1 {
		t.Errorf("sent %d messages, want 1", len(f.transport.sent))
	}
}

// assertNothingSent fails when a refused send reached the transport, the Sent copy or the outbox.
func assertNothingSent(t *testing.T, f *schedFixture) {
	t.Helper()
	if len(f.transport.sent) != 0 || len(f.sent.saved) != 0 || len(f.outbox.items) != 0 {
		t.Errorf("a refused send left a trace: sent=%d sentCopies=%d queued=%d",
			len(f.transport.sent), len(f.sent.saved), len(f.outbox.items))
	}
}
