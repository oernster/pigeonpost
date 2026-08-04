package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeSchedulingCodec is a hand-written SchedulingCodec with scripted output and error-injection fields.
type fakeSchedulingCodec struct {
	decoded            domain.SchedulingMessage
	decodeErr          error
	request            []byte
	requestErr         error
	cancel             []byte
	cancelErr          error
	reply              []byte
	replyErr           error
	encodedReplyStatus domain.ParticipationStatus
}

func (f *fakeSchedulingCodec) DecodeScheduling(_ []byte) (domain.SchedulingMessage, error) {
	if f.decodeErr != nil {
		return domain.SchedulingMessage{}, f.decodeErr
	}
	return f.decoded, nil
}

func (f *fakeSchedulingCodec) EncodeRequest(_ []domain.Event) ([]byte, error) {
	if f.requestErr != nil {
		return nil, f.requestErr
	}
	return f.request, nil
}

func (f *fakeSchedulingCodec) EncodeCancel(_ []domain.Event) ([]byte, error) {
	if f.cancelErr != nil {
		return nil, f.cancelErr
	}
	return f.cancel, nil
}

func (f *fakeSchedulingCodec) EncodeReply(_ domain.Event, _ domain.EmailAddress, status domain.ParticipationStatus) ([]byte, error) {
	if f.replyErr != nil {
		return nil, f.replyErr
	}
	f.encodedReplyStatus = status
	return f.reply, nil
}

func schedAddr(t *testing.T, address string) domain.EmailAddress {
	t.Helper()
	a, err := domain.NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("address %q: %v", address, err)
	}
	return a
}

// schedMeeting builds a meeting event with the given organizer and attendee addresses. An empty
// organizer yields an event with none; recurrenceID marks a per-occurrence event.
func schedMeeting(t *testing.T, uid, organizer string, recurrenceID time.Time, attendees ...string) domain.Event {
	t.Helper()
	in := domain.EventInput{
		ID: uid, UID: uid, Summary: "Sync",
		Start: time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC), RecurrenceID: recurrenceID,
	}
	if organizer != "" {
		org, err := domain.NewOrganizer(schedAddr(t, organizer), "")
		if err != nil {
			t.Fatalf("organizer: %v", err)
		}
		in.Organizer = org
	}
	for _, a := range attendees {
		att, err := domain.NewAttendee(domain.AttendeeInput{Address: schedAddr(t, a)})
		if err != nil {
			t.Fatalf("attendee: %v", err)
		}
		in.Attendees = append(in.Attendees, att)
	}
	ev, err := domain.NewEvent(in)
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return ev
}

func schedMessage(t *testing.T, method domain.Method, events ...domain.Event) domain.SchedulingMessage {
	t.Helper()
	m, err := domain.NewSchedulingMessage(method, events)
	if err != nil {
		t.Fatalf("scheduling message: %v", err)
	}
	return m
}

type schedFixture struct {
	svc       *SchedulingService
	codec     *fakeSchedulingCodec
	calendar  *fakeCalendarStore
	messages  *fakeMailStore
	accounts  *fakeAccountStore
	transport *fakeMailTransport
	sent      *fakeSentSaver
	outbox    *fakeOutboxStore
}

// newSchedFixture wires the service with fakes and seeds message m1 in folder f1 owned by account a1
// (address user@example.com) alongside a Sent folder, with a cached body carrying an invite the fake
// codec decodes to sched.
func newSchedFixture(t *testing.T, sched domain.SchedulingMessage) *schedFixture {
	t.Helper()
	codec := &fakeSchedulingCodec{
		decoded: sched,
		request: []byte("BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nEND:VCALENDAR\r\n"),
		cancel:  []byte("BEGIN:VCALENDAR\r\nMETHOD:CANCEL\r\nEND:VCALENDAR\r\n"),
		reply:   []byte("BEGIN:VCALENDAR\r\nMETHOD:REPLY\r\nEND:VCALENDAR\r\n"),
	}
	calendar := &fakeCalendarStore{}
	messages := newFakeMailStore()
	accounts := newFakeAccountStore()
	transport := &fakeMailTransport{}
	sent := &fakeSentSaver{}
	outbox := &fakeOutboxStore{}

	accounts.accounts["a1"] = testAccount(t, "a1")
	messages.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), sentFolder(t, "a1", "Sent")}
	messages.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	body, err := domain.NewMessageBody("m1", "", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	messages.bodies["m1"] = body.WithInvite([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))

	return &schedFixture{
		svc: NewSchedulingService(codec, calendar, messages, accounts, transport, sent, outbox,
			fakeClock{now: time.Unix(0, 0).UTC()}, func() string { return "sched-q1" }),
		codec:     codec,
		calendar:  calendar,
		messages:  messages,
		accounts:  accounts,
		transport: transport,
		sent:      sent,
		outbox:    outbox,
	}
}

const me = "user@example.com"

func TestInvitationResolvesForDisplay(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))

	inv, err := f.svc.Invitation(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Invitation: %v", err)
	}
	if inv.Method != domain.MethodRequest {
		t.Errorf("Method = %q, want REQUEST", inv.Method)
	}
	if inv.Me.Address() != me {
		t.Errorf("Me = %q, want %q", inv.Me.Address(), me)
	}
	if inv.MyStatus != domain.PartStatNeedsAction {
		t.Errorf("MyStatus = %q, want NEEDS-ACTION", inv.MyStatus)
	}
	if inv.Event.UID() != "m1" {
		t.Errorf("Event UID = %q, want m1", inv.Event.UID())
	}
}

func TestInvitationStatusNeedsActionWhenNotListed(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "other@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))

	inv, err := f.svc.Invitation(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Invitation: %v", err)
	}
	if inv.MyStatus != domain.PartStatNeedsAction {
		t.Errorf("MyStatus = %q, want NEEDS-ACTION when not an attendee", inv.MyStatus)
	}
}

func TestInvitationNoInvite(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	// Replace the cached body with one that carries no invite.
	plain, _ := domain.NewMessageBody("m1", "just text", "")
	f.messages.bodies["m1"] = plain

	if _, err := f.svc.Invitation(context.Background(), "m1"); !errors.Is(err, ErrNoInvite) {
		t.Errorf("error = %v, want ErrNoInvite", err)
	}
}

func TestInvitationBodyNotCached(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	delete(f.messages.bodies, "m1")
	if _, err := f.svc.Invitation(context.Background(), "m1"); !errors.Is(err, ErrBodyNotCached) {
		t.Errorf("error = %v, want ErrBodyNotCached", err)
	}
}

func TestInvitationDecodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.decodeErr = errBoom
	if _, err := f.svc.Invitation(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestInvitationAccountResolutionErrors(t *testing.T) {
	base := func() *schedFixture {
		return newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	}
	t.Run("message", func(t *testing.T) {
		f := base()
		f.messages.getMessageErr = errBoom
		if _, err := f.svc.Invitation(context.Background(), "m1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
	t.Run("folder", func(t *testing.T) {
		f := base()
		f.messages.getFolderErr = errBoom
		if _, err := f.svc.Invitation(context.Background(), "m1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
	t.Run("account", func(t *testing.T) {
		f := base()
		f.accounts.getErr = errBoom
		if _, err := f.svc.Invitation(context.Background(), "m1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestRespondDecodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.decodeErr = errBoom
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyCancellationDecodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.decodeErr = errBoom
	if err := f.svc.ApplyCancellation(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyReplyDecodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.decodeErr = errBoom
	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRespondSavesMeetingAndSendsReply(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Fatalf("saved %d events, want 1", len(f.calendar.savedEvt))
	}
	if statusOf(f.calendar.savedEvt[0], schedAddr(t, me)) != domain.PartStatAccepted {
		t.Errorf("saved meeting did not record the recipient's ACCEPTED status")
	}
	if f.codec.encodedReplyStatus != domain.PartStatAccepted {
		t.Errorf("reply encoded with status %q, want ACCEPTED", f.codec.encodedReplyStatus)
	}
	if len(f.transport.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(f.transport.sent))
	}
	sent := f.transport.sent[0]
	if sent.Calendar().Method() != domain.MethodReply {
		t.Errorf("sent calendar method = %q, want REPLY", sent.Calendar().Method())
	}
	if len(sent.To()) != 1 || sent.To()[0].Address() != "chair@example.com" {
		t.Errorf("reply addressed to %v, want the organizer", sent.To())
	}
}

func TestRespondWhenRecipientNotListed(t *testing.T) {
	// The recipient is not in the invite's attendee list, so nothing is updated but the reply still sends.
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "other@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatDeclined); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(f.transport.sent) != 1 {
		t.Errorf("a reply should still be sent when the recipient was not listed")
	}
}

func TestRespondRejectsNonRequest(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, ErrNotInvitable) {
		t.Errorf("error = %v, want ErrNotInvitable", err)
	}
}

func TestRespondRejectsMissingOrganizer(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "", time.Time{}, me)))
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, ErrNoOrganizer) {
		t.Errorf("error = %v, want ErrNoOrganizer", err)
	}
}

func TestRespondAccountError(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	f.messages.getMessageErr = errBoom
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRespondSaveError(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	f.calendar.saveEvtErr = errBoom
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRespondReplyEncodeError(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	f.codec.replyErr = errBoom
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRespondEmptyReplyPayload(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	f.codec.reply = nil // an empty payload cannot form a calendar part
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, domain.ErrEmptyCalendarPart) {
		t.Errorf("error = %v, want ErrEmptyCalendarPart", err)
	}
}

func TestRespondTransportError(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	f.transport.sendErr = errBoom
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyCancellationRemovesMeeting(t *testing.T) {
	stored := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	f.calendar.events = []domain.Event{stored}

	if err := f.svc.ApplyCancellation(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyCancellation: %v", err)
	}
	if len(f.calendar.deletedEvt) != 1 || f.calendar.deletedEvt[0] != "m1" {
		t.Errorf("deleted = %v, want [m1]", f.calendar.deletedEvt)
	}
}

func TestApplyCancellationRejectsNonCancel(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	if err := f.svc.ApplyCancellation(context.Background(), "m1"); !errors.Is(err, ErrNotCancellation) {
		t.Errorf("error = %v, want ErrNotCancellation", err)
	}
}

func TestApplyCancellationNoMatchIsNoOp(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	// A stored meeting with a different UID must not be touched by this cancellation.
	f.calendar.events = []domain.Event{schedMeeting(t, "other", "chair@example.com", time.Time{}, me)}
	if err := f.svc.ApplyCancellation(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyCancellation: %v", err)
	}
	if len(f.calendar.deletedEvt) != 0 {
		t.Errorf("deleted = %v, want none", f.calendar.deletedEvt)
	}
}

func TestApplyCancellationListError(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	f.calendar.listEvtErr = errBoom
	if err := f.svc.ApplyCancellation(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyCancellationDeleteError(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)}
	f.calendar.failDelID = "m1"
	if err := f.svc.ApplyCancellation(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyReplyUpdatesAttendeeStatus(t *testing.T) {
	reply := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	reply = withStatus(reply, schedAddr(t, "guest@example.com"), domain.PartStatAccepted)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")}

	if err := f.svc.ApplyReply(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyReply: %v", err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Fatalf("saved %d events, want 1", len(f.calendar.savedEvt))
	}
	if statusOf(f.calendar.savedEvt[0], schedAddr(t, "guest@example.com")) != domain.PartStatAccepted {
		t.Errorf("stored meeting did not record the responder's ACCEPTED status")
	}
}

func TestApplyReplyAddsUnlistedResponder(t *testing.T) {
	// The reply comes from an address the stored meeting does not list (a delegate, or a guest answering
	// from a different mailbox than the one invited). The response must land, not vanish.
	reply := withStatus(schedMeeting(t, "m1", "chair@example.com", time.Time{}, "delegate@example.com"),
		schedAddr(t, "delegate@example.com"), domain.PartStatAccepted)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")}

	if err := f.svc.ApplyReply(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyReply: %v", err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Fatalf("saved %d events, want 1", len(f.calendar.savedEvt))
	}
	saved := f.calendar.savedEvt[0]
	if len(saved.Attendees()) != 2 {
		t.Fatalf("attendees = %d, want the original guest plus the appended responder", len(saved.Attendees()))
	}
	if statusOf(saved, schedAddr(t, "delegate@example.com")) != domain.PartStatAccepted {
		t.Errorf("appended responder's status not recorded")
	}
	if statusOf(saved, schedAddr(t, "guest@example.com")) != domain.PartStatNeedsAction {
		t.Errorf("the original guest's status must be untouched")
	}
}

func TestApplyReplySeriesUpdatesMasterAndOverrides(t *testing.T) {
	// A reply naming no occurrence covers the whole series, so the master and every stored override
	// must record it, not just the first match.
	occurrence := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	reply := withStatus(schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com"),
		schedAddr(t, "guest@example.com"), domain.PartStatAccepted)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.events = []domain.Event{
		schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com"),
		schedMeeting(t, "m1", "chair@example.com", occurrence, "guest@example.com"),
	}

	if err := f.svc.ApplyReply(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyReply: %v", err)
	}
	if len(f.calendar.savedEvt) != 2 {
		t.Fatalf("saved %d events, want the master and the override", len(f.calendar.savedEvt))
	}
	for i, saved := range f.calendar.savedEvt {
		if statusOf(saved, schedAddr(t, "guest@example.com")) != domain.PartStatAccepted {
			t.Errorf("saved event %d did not record the series-wide ACCEPTED", i)
		}
	}
}

func TestApplyReplyOccurrenceUpdatesOnlyThatOverride(t *testing.T) {
	// A reply naming an occurrence must touch that override alone, leaving the master as it was.
	occurrence := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	reply := withStatus(schedMeeting(t, "m1", "chair@example.com", occurrence, "guest@example.com"),
		schedAddr(t, "guest@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.events = []domain.Event{
		schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com"),
		schedMeeting(t, "m1", "chair@example.com", occurrence, "guest@example.com"),
	}

	if err := f.svc.ApplyReply(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyReply: %v", err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Fatalf("saved %d events, want only the named occurrence", len(f.calendar.savedEvt))
	}
	if !f.calendar.savedEvt[0].RecurrenceID().Equal(occurrence) {
		t.Errorf("the saved event is not the named occurrence")
	}
}

func TestApplyReplyRejectsNonReply(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)))
	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, ErrNotReply) {
		t.Errorf("error = %v, want ErrNotReply", err)
	}
}

func TestApplyReplyNoAttendee(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, schedMeeting(t, "m1", "chair@example.com", time.Time{})))
	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, ErrNoReplyAttendee) {
		t.Errorf("error = %v, want ErrNoReplyAttendee", err)
	}
}

func TestApplyReplyListError(t *testing.T) {
	reply := withStatus(schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com"),
		schedAddr(t, "guest@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.listEvtErr = errBoom
	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyReplySaveError(t *testing.T) {
	reply := withStatus(schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com"),
		schedAddr(t, "guest@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")}
	f.calendar.saveEvtErr = errBoom
	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyReplyNoMatch(t *testing.T) {
	reply := withStatus(schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com"),
		schedAddr(t, "guest@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodReply, reply))
	// A stored meeting with a different UID does not match the reply.
	f.calendar.events = []domain.Event{schedMeeting(t, "other", "chair@example.com", time.Time{}, "guest@example.com")}
	if err := f.svc.ApplyReply(context.Background(), "m1"); !errors.Is(err, ErrMeetingNotFound) {
		t.Errorf("error = %v, want ErrMeetingNotFound", err)
	}
}

func TestSendRequestEmailsAttendees(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})

	if err := f.svc.SendRequest(context.Background(), "a1", []domain.Event{event}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if len(f.transport.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(f.transport.sent))
	}
	sent := f.transport.sent[0]
	if sent.Calendar().Method() != domain.MethodRequest {
		t.Errorf("sent method = %q, want REQUEST", sent.Calendar().Method())
	}
	if len(sent.To()) != 1 || sent.To()[0].Address() != "guest@example.com" {
		t.Errorf("request addressed to %v, want the attendee", sent.To())
	}
}

func TestSendCancelEmailsAttendees(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})

	if err := f.svc.SendCancel(context.Background(), "a1", []domain.Event{event}); err != nil {
		t.Fatalf("SendCancel: %v", err)
	}
	if len(f.transport.sent) != 1 || f.transport.sent[0].Calendar().Method() != domain.MethodCancel {
		t.Errorf("expected one CANCEL message, got %+v", f.transport.sent)
	}
}

func TestSendOrganizerAccountError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	if err := f.svc.SendRequest(context.Background(), "missing", []domain.Event{event}); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("error = %v, want ErrAccountNotFound", err)
	}
}

func TestSendOrganizerNoEvents(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	if err := f.svc.SendRequest(context.Background(), "a1", nil); !errors.Is(err, domain.ErrNoSchedulingEvents) {
		t.Errorf("error = %v, want ErrNoSchedulingEvents", err)
	}
}

func TestSendRequestEncodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.requestErr = errBoom
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	if err := f.svc.SendRequest(context.Background(), "a1", []domain.Event{event}); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestSendCancelEncodeError(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.codec.cancelErr = errBoom
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	if err := f.svc.SendCancel(context.Background(), "a1", []domain.Event{event}); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestSendRequestNoAttendeesIsRejected(t *testing.T) {
	f := newSchedFixture(t, domain.SchedulingMessage{})
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}) // no attendees to address
	if err := f.svc.SendRequest(context.Background(), "a1", []domain.Event{event}); !errors.Is(err, domain.ErrNoRecipients) {
		t.Errorf("error = %v, want ErrNoRecipients", err)
	}
}

func TestRespondSavesSentCopy(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(f.sent.saved) != 1 {
		t.Fatalf("saved %d Sent copies, want 1", len(f.sent.saved))
	}
	if f.sent.paths[0] != "Sent" {
		t.Errorf("Sent copy path = %q, want Sent", f.sent.paths[0])
	}
	if f.sent.saved[0].Calendar().Method() != domain.MethodReply {
		t.Errorf("Sent copy calendar method = %q, want REPLY", f.sent.saved[0].Calendar().Method())
	}
}

func TestRespondSentCopyFailureDoesNotFailSend(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))
	f.sent.saveErr = errBoom

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Errorf("a failed Sent copy must not fail a delivered reply, got %v", err)
	}
}

func TestRespondSkipsSentCopyWhenProviderSaves(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))
	f.accounts.accounts["a1"] = gmailStyleAccount(t, "a1")

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(f.sent.saved) != 0 {
		t.Errorf("saved %d Sent copies, want none for a provider that saves server-side", len(f.sent.saved))
	}
}

func TestRespondOfflineQueuesReply(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))
	f.transport.sendErr = domain.ErrOffline

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("an offline reply must queue, not fail: %v", err)
	}
	if len(f.outbox.items) != 1 {
		t.Fatalf("queued %d outbox items, want 1", len(f.outbox.items))
	}
	item := f.outbox.items[0]
	if item.Kind() != domain.OutboxSend {
		t.Errorf("queued kind = %v, want OutboxSend", item.Kind())
	}
	if item.Message().Calendar().Method() != domain.MethodReply {
		t.Errorf("queued calendar method = %q, want REPLY", item.Message().Calendar().Method())
	}
	if len(f.sent.saved) != 0 {
		t.Errorf("an undelivered reply must not get a Sent copy yet")
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Errorf("the meeting must still be saved with the response while offline")
	}
}

func TestRespondOfflineEnqueueError(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))
	f.transport.sendErr = domain.ErrOffline
	f.outbox.enqueueErr = errBoom

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestSendRequestSavesSentCopy(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})

	if err := f.svc.SendRequest(context.Background(), "a1", []domain.Event{event}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if len(f.sent.saved) != 1 || f.sent.saved[0].Calendar().Method() != domain.MethodRequest {
		t.Errorf("expected one REQUEST Sent copy, got %+v", f.sent.saved)
	}
}

func TestSendCancelOfflineQueues(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, "guest@example.com")
	f := newSchedFixture(t, domain.SchedulingMessage{})
	f.transport.sendErr = domain.ErrOffline

	if err := f.svc.SendCancel(context.Background(), "a1", []domain.Event{event}); err != nil {
		t.Fatalf("an offline CANCEL must queue, not fail: %v", err)
	}
	if len(f.outbox.items) != 1 || f.outbox.items[0].Message().Calendar().Method() != domain.MethodCancel {
		t.Errorf("expected one queued CANCEL, got %+v", f.outbox.items)
	}
}

// gmailStyleAccount builds an account whose outgoing host saves sent mail server-side, so the Sent-copy
// skip path can be driven.
func gmailStyleAccount(t *testing.T, id string) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", me)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	in, err := domain.NewServerConfig("imap.gmail.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("incoming: %v", err)
	}
	out, err := domain.NewServerConfig("smtp.gmail.com", 587, domain.SecurityStartTLS)
	if err != nil {
		t.Fatalf("outgoing: %v", err)
	}
	account, err := domain.NewAccount(id, "Gmail", addr, domain.ProtocolIMAP, in, out, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

func TestInvitationOverlaysStoredStatuses(t *testing.T) {
	// The invite arrived with everyone NEEDS-ACTION; the stored meeting has recorded the recipient's
	// accept and another guest's decline, so the card must show those, not the email's frozen ICS.
	invite := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com")
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, invite))
	stored := withStatus(schedMeeting(t, "m1", "chair@example.com", time.Time{}, me, "other@example.com"),
		schedAddr(t, me), domain.PartStatAccepted)
	stored = withStatus(stored, schedAddr(t, "other@example.com"), domain.PartStatDeclined)
	f.calendar.events = []domain.Event{stored}

	inv, err := f.svc.Invitation(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Invitation: %v", err)
	}
	if inv.MyStatus != domain.PartStatAccepted {
		t.Errorf("MyStatus = %q, want ACCEPTED from the stored meeting", inv.MyStatus)
	}
	if got := statusOf(inv.Event, schedAddr(t, "other@example.com")); got != domain.PartStatDeclined {
		t.Errorf("other attendee status = %q, want DECLINED from the stored meeting", got)
	}
}

func TestInvitationOverlayIgnoresUnrelatedMeeting(t *testing.T) {
	invite := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, invite))
	f.calendar.events = []domain.Event{withStatus(schedMeeting(t, "other", "chair@example.com", time.Time{}, me),
		schedAddr(t, me), domain.PartStatAccepted)}

	inv, err := f.svc.Invitation(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Invitation: %v", err)
	}
	if inv.MyStatus != domain.PartStatNeedsAction {
		t.Errorf("MyStatus = %q, want NEEDS-ACTION when only an unrelated meeting is stored", inv.MyStatus)
	}
}

func TestInvitationOverlayListError(t *testing.T) {
	invite := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, invite))
	f.calendar.listEvtErr = errBoom

	if _, err := f.svc.Invitation(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestResponseWord(t *testing.T) {
	cases := map[domain.ParticipationStatus]string{
		domain.PartStatAccepted:    "Accepted",
		domain.PartStatDeclined:    "Declined",
		domain.PartStatTentative:   "Tentative",
		domain.PartStatNeedsAction: "Responded",
	}
	for status, want := range cases {
		if got := responseWord(status); got != want {
			t.Errorf("responseWord(%q) = %q, want %q", status, got, want)
		}
	}
}
