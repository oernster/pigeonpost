package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func TestApplyIncomingNoInviteIsNoOp(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	body, err := domain.NewMessageBody("m1", "", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	f.messages.bodies["m1"] = body // no invite

	changed, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed {
		t.Errorf("got (%v, %v), want (false, nil)", changed, err)
	}
}

func TestApplyIncomingDecodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.decodeErr = errBoom
	if _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestApplyIncomingReplyUpdatesMeeting(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	reply = withStatus(reply, schedAddr(t, "guest@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")}

	changed, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || !changed {
		t.Fatalf("got (%v, %v), want (true, nil)", changed, err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Errorf("saved %d events, want 1", len(f.calendar.savedEvt))
	}
}

func TestApplyIncomingReplyForUnknownMeetingIsNoOp(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	// No stored meeting matches, so ApplyReply returns ErrMeetingNotFound, treated here as a no-op.
	changed, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed {
		t.Errorf("got (%v, %v), want (false, nil)", changed, err)
	}
}

func TestApplyIncomingReplyWithNoAttendeeIsNoOp(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}) // no attendee named
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	changed, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed {
		t.Errorf("got (%v, %v), want (false, nil)", changed, err)
	}
}

func TestApplyIncomingReplyListErrorPropagates(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.listEvtErr = errBoom
	if _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestApplyIncomingCancelRemovesMeeting(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel,
		schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)}

	changed, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || !changed {
		t.Fatalf("got (%v, %v), want (true, nil)", changed, err)
	}
	if len(f.calendar.deletedEvt) != 1 || f.calendar.deletedEvt[0] != "m1" {
		t.Errorf("deleted = %v, want [m1]", f.calendar.deletedEvt)
	}
}

func TestApplyIncomingCancelErrorPropagates(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel,
		schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	f.calendar.listEvtErr = errBoom
	if _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestApplyIncomingRequestIsNoOp(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest,
		schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	changed, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed {
		t.Errorf("got (%v, %v), want (false, nil)", changed, err)
	}
}
