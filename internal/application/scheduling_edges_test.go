package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The edges of the scheduling service: the branches a happy-path test never reaches. They are
// gathered here because each is an error or a skip rather than a behaviour worth grouping with the
// flow it belongs to, and because `internal/application` carries a hard 100% gate that these close.

func TestApplyIncomingIgnoresAMethodItDoesNotHandle(t *testing.T) {
	// PUBLISH is a valid iTIP method that carries no response and cancels nothing, so there is
	// nothing to apply. It must be a silent no-op rather than an error: the poller runs
	// ApplyIncoming blind over every arriving message.
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodPublish, event))

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
}

func TestApplyRequestStatusesFailsWhenTheAccountCannotBeResolved(t *testing.T) {
	// The status merge needs the recipient's own address to know which row to leave alone, so a
	// message whose account cannot be resolved has to fail rather than merge blind.
	//
	// "m2" carries a decodable invite but sits in no folder, so the invite decodes and the account
	// lookup after it is what fails. Reaching this branch needs both halves: a body the codec
	// accepts, and no route from the message to an account.
	event := schedMeeting(t, "m2", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))
	orphan, err := domain.NewMessageBody("m2", "", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	f.messages.bodies["m2"] = orphan.WithInvite([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))

	if _, _, err := f.svc.ApplyIncoming(context.Background(), "m2"); err == nil {
		t.Error("an unresolvable account must fail rather than merge statuses blind")
	}
}

func TestApplyRequestStatusesSkipsMeetingsTheUpdateDoesNotName(t *testing.T) {
	// A REQUEST update only speaks for the meetings it carries. Every other stored meeting is
	// passed over untouched, which is what stops one organiser's update rewriting another's.
	incoming := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, incoming))
	f.calendar.events = []domain.Event{
		schedMeeting(t, "unrelated", "someone@example.com", time.Time{}, me),
	}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
	if len(f.calendar.savedEvt) != 0 {
		t.Errorf("saved %d meetings, want 0: an unnamed meeting must not be touched", len(f.calendar.savedEvt))
	}
}

func TestApplyRequestStatusesReportsAFailedSave(t *testing.T) {
	// A store that refuses the write must surface it. Reporting success here would leave the
	// user's calendar disagreeing with the organiser's view with nothing said.
	guest := "guest@example.com"
	incoming := schedMeeting(t, "m1", "chair@example.com", time.Time{}, guest)
	incoming = withStatus(incoming, schedAddr(t, guest), domain.PartStatAccepted)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, incoming))
	f.calendar.events = []domain.Event{
		schedMeeting(t, "m1", "chair@example.com", time.Time{}, guest),
	}
	f.calendar.saveEvtErr = errBoom

	if _, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestOfflineSchedulingSendRejectsAnUnusableOutboxID(t *testing.T) {
	// The outbox item is built before it is queued, so a generator that yields nothing has to fail
	// here. Swallowing it would drop the reply silently: not sent, not queued, no error.
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))
	f.transport.sendErr = domain.ErrOffline
	f.svc.newID = func() string { return "" }

	err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted)
	if !errors.Is(err, domain.ErrEmptyOutboxID) {
		t.Errorf("error = %v, want wrapped ErrEmptyOutboxID", err)
	}
	if len(f.outbox.items) != 0 {
		t.Errorf("queued %d items, want 0: an item that would not build must not be queued", len(f.outbox.items))
	}
}
