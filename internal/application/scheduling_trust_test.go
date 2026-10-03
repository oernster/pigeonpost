package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The trust and revision rules for incoming scheduling: only the organiser may withdraw or refresh a
// stored meeting, a whole-series CANCEL takes the series' overrides with it, a single-occurrence CANCEL
// really removes that occurrence, an older revision never overwrites a newer one and every override is
// stored under its own id.

// schedOccurrence is the original start of the series occurrence the override and occurrence tests name.
var schedOccurrence = time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)

// schedSeries is a weekly meeting organised by schedOrganizer with the recipient invited.
func schedSeries(t *testing.T) domain.Event {
	t.Helper()
	return schedMeeting(t, "m1", schedOrganizer, time.Time{}, me).WithRecurrence("FREQ=WEEKLY")
}

// schedOverride is the stored override of schedSeries at schedOccurrence, keyed as the store keys it.
func schedOverride(t *testing.T) domain.Event {
	t.Helper()
	ev := schedMeeting(t, "m1", schedOrganizer, schedOccurrence, me)
	return ev.WithID(domain.EventIDFor(ev.UID(), ev.RecurrenceID()))
}

// schedRevision returns the event at the given organiser revision (RFC 5545 SEQUENCE).
func schedRevision(t *testing.T, uid, organizer string, sequence int, attendees ...string) domain.Event {
	t.Helper()
	base := schedMeeting(t, uid, organizer, time.Time{}, attendees...)
	ev, err := domain.NewEvent(domain.EventInput{
		ID: base.ID(), UID: base.UID(), Summary: base.Summary(), Start: base.Start(),
		Organizer: base.Organizer(), Attendees: base.Attendees(), Sequence: sequence,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return ev
}

// sendFrom replaces the sender of the fixture's message m1.
func (f *schedFixture) sendFrom(t *testing.T, from string) {
	t.Helper()
	f.messages.messages["f1"] = []domain.MessageSummary{schedMailFrom(t, "m1", "f1", from)}
}

func TestApplyIncomingCancelFromAnotherOrganizerLeavesMeetingAndMailUnread(t *testing.T) {
	// The audit's regression: a CANCEL naming a stranger as organiser must not touch a meeting someone else
	// organises. Nor may it be reported resolved, so the poller leaves the mail unread for the user.
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel,
		schedMeeting(t, "m1", "stranger@evil.example", time.Time{})))
	f.sendFrom(t, "stranger@evil.example")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
	if len(f.calendar.deletedEvt) != 0 {
		t.Errorf("deleted = %v, want none: the sender does not organise the meeting", f.calendar.deletedEvt)
	}
}

func TestApplyIncomingCancelNamingTheOrganizerFromAnotherSenderIsRefused(t *testing.T) {
	// Naming the right organiser is not enough: the mail must come from that address too.
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, time.Time{})))
	f.sendFrom(t, "stranger@evil.example")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
	if len(f.calendar.deletedEvt) != 0 {
		t.Errorf("deleted = %v, want none", f.calendar.deletedEvt)
	}
}

func TestApplyCancellationMatchesTheOrganizerCaseInsensitively(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", "Chair@Example.COM", time.Time{})))
	f.sendFrom(t, "CHAIR@example.com")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}

	if err := f.svc.ApplyCancellation(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyCancellation: %v", err)
	}
	if len(f.calendar.deletedEvt) != 1 {
		t.Errorf("deleted = %v, want [m1]", f.calendar.deletedEvt)
	}
}

func TestSameAddressIgnoresAMailtoSchemeCaseAndSpace(t *testing.T) {
	// The codec strips mailto: from a CAL-ADDRESS. An address that kept it must still compare equal to the
	// From line it came in.
	if !sameAddress(schedAddr(t, " MAILTO:Chair@Example.com "), schedAddr(t, schedOrganizer)) {
		t.Error("a mailto: organiser must match the same bare sender")
	}
	if sameAddress(schedAddr(t, "mailto:other@example.com"), schedAddr(t, schedOrganizer)) {
		t.Error("different mailboxes must not match")
	}
}

func TestApplyCancellationFromAnotherOrganizerReportsUntrusted(t *testing.T) {
	// The manual path (the reader's Remove from calendar) says why nothing happened.
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel,
		schedMeeting(t, "m1", "stranger@evil.example", time.Time{})))
	f.sendFrom(t, "stranger@evil.example")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}

	if err := f.svc.ApplyCancellation(context.Background(), "m1"); !errors.Is(err, ErrUntrustedScheduling) {
		t.Errorf("error = %v, want ErrUntrustedScheduling", err)
	}
}

func TestApplyCancellationWithNoOrganizerIsUntrusted(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", "", time.Time{})))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}

	if err := f.svc.ApplyCancellation(context.Background(), "m1"); !errors.Is(err, ErrUntrustedScheduling) {
		t.Errorf("error = %v, want ErrUntrustedScheduling", err)
	}
}

func TestApplyCancellationSenderLookupFailurePropagates(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, time.Time{})))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}
	delete(f.messages.messages, "f1")

	if err := f.svc.ApplyCancellation(context.Background(), "m1"); err == nil {
		t.Error("expected an error when the message's sender cannot be read")
	}
}

func TestApplyCancellationOfTheSeriesRemovesItsOverrides(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, time.Time{})))
	override := schedOverride(t)
	f.calendar.events = []domain.Event{schedSeries(t), override}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || !changed || !resolved {
		t.Fatalf("got (%v, %v, %v), want (true, true, nil)", changed, resolved, err)
	}
	if len(f.calendar.deletedEvt) != 2 || f.calendar.deletedEvt[0] != "m1" || f.calendar.deletedEvt[1] != override.ID() {
		t.Errorf("deleted = %q, want the master and its override", f.calendar.deletedEvt)
	}
}

func TestApplyCancellationOfOneOccurrenceExcludesItFromTheSeries(t *testing.T) {
	// No override is stored for the cancelled occurrence, so the master must gain an EXDATE for it:
	// reporting it resolved while the occurrence still shows was the audit's second defect.
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, schedOccurrence)))
	f.calendar.events = []domain.Event{schedSeries(t)}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || !changed || !resolved {
		t.Fatalf("got (%v, %v, %v), want (true, true, nil)", changed, resolved, err)
	}
	if len(f.calendar.savedEvt) != 1 {
		t.Fatalf("saved %d events, want the master", len(f.calendar.savedEvt))
	}
	ex := f.calendar.savedEvt[0].ExDates()
	if len(ex) != 1 || !ex[0].Equal(schedOccurrence) {
		t.Errorf("master EXDATEs = %v, want [%v]", ex, schedOccurrence)
	}
}

func TestApplyCancellationOfAnOverriddenOccurrenceDropsTheOverrideAndExcludesIt(t *testing.T) {
	// Deleting only the override would let the master's rule regenerate the occurrence.
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, schedOccurrence)))
	override := schedOverride(t)
	f.calendar.events = []domain.Event{schedSeries(t), override}

	if err := f.svc.ApplyCancellation(context.Background(), "m1"); err != nil {
		t.Fatalf("ApplyCancellation: %v", err)
	}
	if len(f.calendar.deletedEvt) != 1 || f.calendar.deletedEvt[0] != override.ID() {
		t.Errorf("deleted = %q, want only the override", f.calendar.deletedEvt)
	}
	if len(f.calendar.savedEvt) != 1 || len(f.calendar.savedEvt[0].ExDates()) != 1 {
		t.Errorf("the master must exclude the cancelled occurrence: saved %v", f.calendar.savedEvt)
	}
}

func TestApplyCancellationOfAnAlreadyExcludedOccurrenceChangesNothing(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, schedOccurrence)))
	f.calendar.events = []domain.Event{schedSeries(t).WithExDates([]time.Time{schedOccurrence})}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
	if len(f.calendar.savedEvt) != 0 {
		t.Errorf("saved %d events, want none", len(f.calendar.savedEvt))
	}
}

func TestApplyCancellationOfAnOccurrenceOfASingleMeetingIsUnresolved(t *testing.T) {
	// A non-recurring stored meeting has no occurrence to exclude; nothing changes, so nothing is resolved.
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, schedOccurrence)))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
}

func TestApplyCancellationExcludeSaveErrorPropagates(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, schedOccurrence)))
	f.calendar.events = []domain.Event{schedSeries(t)}
	f.calendar.saveEvtErr = errBoom

	if err := f.svc.ApplyCancellation(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestApplyIncomingCancelForAMeetingNotHeldIsUnresolved(t *testing.T) {
	// Nothing was removed, so the mail must stay unread rather than be marked handled.
	f := newSchedFixture(t, schedMessage(t, domain.MethodCancel, schedMeeting(t, "m1", schedOrganizer, time.Time{})))

	changed, resolved, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed || resolved {
		t.Errorf("got (%v, %v, %v), want (false, false, nil)", changed, resolved, err)
	}
}

func TestApplyIncomingRequestFromAnotherOrganizerLeavesStatusesAlone(t *testing.T) {
	// An updated REQUEST is folded in automatically, so it carries the same trust rule as a CANCEL.
	update := schedMeeting(t, "m1", "stranger@evil.example", time.Time{}, me, "other@example.com")
	update = withStatus(update, schedAddr(t, "other@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	f.sendFrom(t, "stranger@evil.example")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me, "other@example.com")}

	changed, _, err := f.svc.ApplyIncoming(context.Background(), "m1")
	if err != nil || changed {
		t.Errorf("got (changed %v, %v), want (false, nil)", changed, err)
	}
	if len(f.calendar.savedEvt) != 0 {
		t.Errorf("saved %d events, want none", len(f.calendar.savedEvt))
	}
}

func TestApplyIncomingRequestFromTheOrganizerAtAnotherSenderIsRefused(t *testing.T) {
	update := schedMeeting(t, "m1", schedOrganizer, time.Time{}, me, "other@example.com")
	update = withStatus(update, schedAddr(t, "other@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	f.sendFrom(t, "stranger@evil.example")
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me, "other@example.com")}

	if changed, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); err != nil || changed {
		t.Errorf("got (changed %v, %v), want (false, nil)", changed, err)
	}
}

func TestApplyIncomingRequestOfAnOlderRevisionIsIgnored(t *testing.T) {
	update := schedRevision(t, "m1", schedOrganizer, 1, me, "other@example.com")
	update = withStatus(update, schedAddr(t, "other@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	f.calendar.events = []domain.Event{schedRevision(t, "m1", schedOrganizer, 2, me, "other@example.com")}

	if changed, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); err != nil || changed {
		t.Errorf("got (changed %v, %v), want (false, nil): an older revision must not land", changed, err)
	}
}

func TestApplyIncomingRequestOfTheSameRevisionStillFoldsStatuses(t *testing.T) {
	update := schedRevision(t, "m1", schedOrganizer, 2, me, "other@example.com")
	update = withStatus(update, schedAddr(t, "other@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	f.calendar.events = []domain.Event{schedRevision(t, "m1", schedOrganizer, 2, me, "other@example.com")}

	if changed, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); err != nil || !changed {
		t.Errorf("got (changed %v, %v), want (true, nil): an equal revision is allowed", changed, err)
	}
}

func TestApplyIncomingRequestSenderLookupFailurePropagates(t *testing.T) {
	update := schedMeeting(t, "m1", schedOrganizer, time.Time{}, me, "other@example.com")
	update = withStatus(update, schedAddr(t, "other@example.com"), domain.PartStatDeclined)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, update))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me, "other@example.com")}
	f.messages.getMessageErr = errBoom

	if _, _, err := f.svc.ApplyIncoming(context.Background(), "m1"); err == nil {
		t.Error("expected an error when the message's sender cannot be read")
	}
}

func TestRespondRefusesAnInvitationFromAnotherOrganizer(t *testing.T) {
	// Answering would save the stranger's copy over a meeting someone else organises.
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest,
		schedMeeting(t, "m1", "stranger@evil.example", time.Time{}, me)))
	f.calendar.events = []domain.Event{schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)}

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, ErrUntrustedScheduling) {
		t.Fatalf("error = %v, want ErrUntrustedScheduling", err)
	}
	if len(f.calendar.savedEvt) != 0 || len(f.transport.sent) != 0 {
		t.Errorf("saved %d, sent %d; want nothing", len(f.calendar.savedEvt), len(f.transport.sent))
	}
}

func TestRespondRefusesAnOlderRevision(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedRevision(t, "m1", schedOrganizer, 1, me)))
	f.calendar.events = []domain.Event{schedRevision(t, "m1", schedOrganizer, 3, me)}

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, ErrStaleInvitation) {
		t.Fatalf("error = %v, want ErrStaleInvitation", err)
	}
	if len(f.calendar.savedEvt) != 0 || len(f.transport.sent) != 0 {
		t.Errorf("saved %d, sent %d; want nothing", len(f.calendar.savedEvt), len(f.transport.sent))
	}
}

func TestRespondAcceptsTheSameRevision(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedRevision(t, "m1", schedOrganizer, 3, me)))
	f.calendar.events = []domain.Event{schedRevision(t, "m1", schedOrganizer, 3, me)}

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
}

func TestRespondListErrorPropagates(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)))
	f.calendar.listEvtErr = errBoom

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRespondStoresTheMasterAndItsOverrideUnderTheirOwnIDs(t *testing.T) {
	// The codec has keyed both events by the bare UID (as it did before overrides had a key of their own);
	// saving them as decoded would let the override overwrite the master.
	master := schedSeries(t)
	override := schedMeeting(t, "m1", schedOrganizer, schedOccurrence, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, master, override))

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	want := []string{"m1", domain.EventIDFor("m1", schedOccurrence)}
	if len(f.calendar.savedEvt) != len(want) {
		t.Fatalf("saved %d events, want %d", len(f.calendar.savedEvt), len(want))
	}
	for i, id := range want {
		if got := f.calendar.savedEvt[i].ID(); got != id {
			t.Errorf("saved[%d] id = %q, want %q", i, got, id)
		}
	}
}
