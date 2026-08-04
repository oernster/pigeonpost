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

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
}

func TestApplyIncomingDecodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.decodeErr = errBoom
	if _, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestApplyIncomingReplyUpdatesMeeting(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	reply = withStatus(reply, schedAddr(t, "guest@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || !changed || !resolved {
		t.Fatalf("got (%v, %v, %v), want (true, true, nil)", changed, resolved, err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Errorf("saved %d events, want 1", len(f.calendar.savedEvt))
	}
}

func TestApplyIncomingReplyForUnknownMeetingIsNoOp(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	// No stored meeting matches, so ApplyReply returns ErrMeetingNotFound, treated here as a no-op.
	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
}

func TestApplyIncomingReplyWithNoAttendeeIsNoOp(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}) // no attendee named
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
}

func TestApplyIncomingReplyListErrorPropagates(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.listEvtErr = errBoom
	if _, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestApplyIncomingCancelRemovesMeeting(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel,
		schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || !changed || !resolved {
		t.Fatalf("got (%v, %v, %v), want (true, true, nil)", changed, resolved, err)
	}
	if len(f.calendar.deletedEvt) != 1 || f.calendar.deletedEvt[0] != "m1" {
		t.Errorf("deleted = %v, want [m1]", f.calendar.deletedEvt)
	}
}

func TestApplyIncomingCancelErrorPropagates(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel,
		schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	f.calendar.listEvtErr = errBoom
	if _, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestApplyIncomingFirstRequestIsNoOp(t *testing.T) {
	// A first-time invitation has no stored meeting to refresh; the user answers it deliberately.
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest,
		schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
}

func TestApplyIncomingRequestFoldsOtherAttendeeStatuses(t *testing.T) {
	// The organiser's updated invitation carries the statuses they have collected. It is the only way
	// one attendee learns another accepted, so the snapshot must land on the stored meeting: the other
	// guest's status updates, while the recipient's own recorded answer stays untouched.
	update := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	update = withStatus(update, schedAddr(t, "other@example.com"), domain.PartStatAccepted)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	stored := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	stored = withStatus(stored, schedAddr(t, me), domain.PartStatAccepted)
	f.calendar.events = []domain.Event{stored}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || !changed {
		t.Fatalf("got (%v, %v, %v), want changed with no error", changed, resolved, err)
	}
	if resolved {
		t.Error("an updated invitation must not be resolved: the user may still need to act on it")
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Fatalf("saved %d events, want 1", len(f.calendar.savedEvt))
	}
	saved := f.calendar.savedEvt[0]
	if statusOf(saved, schedAddr(t, "other@example.com")) != domain.PartStatAccepted {
		t.Errorf("the other attendee's ACCEPTED was not folded in")
	}
	if statusOf(saved, schedAddr(t, me)) != domain.PartStatAccepted {
		t.Errorf("the recipient's own recorded answer must be untouched")
	}
}

func TestApplyIncomingRequestNeverDowngradesOrTouchesMe(t *testing.T) {
	// The snapshot says NEEDS-ACTION for the other guest (no news) and DECLINED for me (the organiser's
	// stale view of an answer I have since changed locally). Neither may move: no news never downgrades
	// a recorded response and my own row is mine.
	update := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	update = withStatus(update, schedAddr(t, me), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	stored := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	stored = withStatus(stored, schedAddr(t, me), domain.PartStatAccepted)
	stored = withStatus(stored, schedAddr(t, "other@example.com"), domain.PartStatTentative)
	f.calendar.events = []domain.Event{stored}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
	if len(f.calendar.savedEvt) != 0 {
		t.Errorf("saved %d events, want none: nothing legitimate moved", len(f.calendar.savedEvt))
	}
}

func TestApplyIncomingRequestListErrorPropagates(t *testing.T) {
	update := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	f.calendar.listEvtErr = errBoom
	if _, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}
