package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Answering an invitation for a meeting held in a CalDAV calendar must keep it in that calendar and record
// the pending update a later sync pushes, exactly as a local edit of a CalDAV event does.

const (
	schedRemoteCalendar = "acc|/cal/"
	schedObjectHref     = "/cal/m1.ics"
)

// schedHeldRemotely is the meeting stored in the CalDAV calendar.
func schedHeldRemotely(t *testing.T, event domain.Event) domain.Event {
	t.Helper()
	return event.WithCalendarID(schedRemoteCalendar)
}

// schedSync seeds the fixture's sync store so it maps the stored meeting m1 to its CalDAV object. It
// returns that store for the test to inspect.
func schedSync(f *schedFixture) *fakeSyncStore {
	f.sync.identities = map[string]SyncedEventIdentity{
		"m1": {Href: schedObjectHref, ETag: "e1", CalendarID: schedRemoteCalendar},
	}
	return f.sync
}

func TestRespondKeepsTheStoredMeetingInItsCalendar(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)))
	f.calendar.events = []domain.Event{schedHeldRemotely(t, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me))}

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(f.calendar.savedEvt) != 1 || f.calendar.savedEvt[0].CalendarID() != schedRemoteCalendar {
		t.Errorf("saved %v, want the meeting kept in %q", f.calendar.savedEvt, schedRemoteCalendar)
	}
}

func TestRespondRecordsAPendingUpdateForACalDAVMeeting(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)))
	f.calendar.events = []domain.Event{schedHeldRemotely(t, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me))}
	sync := schedSync(f)

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	want := PendingCalendarObject{CalendarID: schedRemoteCalendar, Href: schedObjectHref, Op: CalendarOpUpdate, BaseETag: "e1"}
	if len(sync.savedPend) != 1 || sync.savedPend[0].op != want || sync.savedPend[0].href != schedObjectHref {
		t.Errorf("pending saves = %+v, want one update of %s", sync.savedPend, schedObjectHref)
	}
	if len(f.calendar.savedEvt) != 0 {
		t.Errorf("the plain save must not also run: %v", f.calendar.savedEvt)
	}
}

func TestRespondFilesANewOverrideIntoTheSeriesObject(t *testing.T) {
	// Only the master is stored; the invitation adds an override. It joins the master's calendar and CalDAV
	// object, so the next push sends the whole series.
	master := schedSeries(t)
	override := schedMeeting(t, "m1", schedOrganizer, schedOccurrence, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, master, override))
	f.calendar.events = []domain.Event{schedHeldRemotely(t, master)}
	sync := schedSync(f)

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(sync.savedPend) != 2 {
		t.Fatalf("pending saves = %d, want the master and the override", len(sync.savedPend))
	}
	added := sync.savedPend[1]
	if added.id != domain.EventIDFor("m1", schedOccurrence) || added.href != schedObjectHref {
		t.Errorf("override saved as %q at %q, want its own id in the series object", added.id, added.href)
	}
}

func TestRespondSavesALocalMeetingPlainlyWithCalendarSync(t *testing.T) {
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)))
	sync := f.sync

	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(f.calendar.savedEvt) != 1 || len(sync.savedPend) != 0 {
		t.Errorf("plain saves %d, pending saves %d; want 1 and 0", len(f.calendar.savedEvt), len(sync.savedPend))
	}
}

func TestRespondReportsACalDAVIdentityOrPendingSaveFailure(t *testing.T) {
	for _, fail := range []func(f *schedFixture){
		func(f *schedFixture) { f.sync.identityErr = errBoom },
		func(f *schedFixture) { schedSync(f).savePendErr = errBoom },
	} {
		f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me)))
		f.calendar.events = []domain.Event{schedHeldRemotely(t, schedMeeting(t, "m1", schedOrganizer, time.Time{}, me))}
		fail(f)
		if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); !errors.Is(err, errBoom) {
			t.Errorf("Respond err = %v, want the sync store failure", err)
		}
		if len(f.transport.sent) != 0 {
			t.Errorf("no REPLY may go out when the meeting was not saved")
		}
	}
}
