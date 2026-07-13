package application

import (
	"context"
	"errors"
	"testing"
)

// editService wires a CalendarEditService over the sync store and a plain CalendarService backed by cal, using
// a fixed id generator so a created event's id is predictable.
func editService(sync *fakeSyncStore, cal *fakeCalendarStore, id string) *CalendarEditService {
	return NewCalendarEditService(NewCalendarService(cal, fixedID(id), &fakeRecurrence{}), sync, fixedID(id))
}

// simpleEventInput is a minimal valid event input in the given calendar.
func simpleEventInput(calendarID string) EventInput {
	return EventInput{CalendarID: calendarID, Summary: "Meet", Start: day(4, 9)}
}

// remoteOnly makes a sync store whose one calendar is remote (has an owning account).
func remoteOnly(calendarID, href string) *fakeSyncStore {
	return &fakeSyncStore{remoteByID: map[string]RemoteCalendarRecord{
		calendarID: {CalendarID: calendarID, AccountID: "acc", Href: href},
	}}
}

func TestEditSaveEventLocalDelegates(t *testing.T) {
	sync := &fakeSyncStore{} // no remote record => a local calendar
	cal := &fakeCalendarStore{}
	id, err := editService(sync, cal, "new1").SaveEvent(context.Background(), simpleEventInput("local"))
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if id != "new1" {
		t.Errorf("id = %q, want new1", id)
	}
	if len(cal.savedEvt) != 1 {
		t.Errorf("a local save should persist through the plain service: %+v", cal.savedEvt)
	}
	if len(sync.savedPend) != 0 {
		t.Errorf("a local calendar records no pending op: %+v", sync.savedPend)
	}
}

func TestEditSaveEventLocalDelegateError(t *testing.T) {
	cal := &fakeCalendarStore{saveEvtErr: errBoom}
	if _, err := editService(&fakeSyncStore{}, cal, "x").SaveEvent(context.Background(), simpleEventInput("local")); err == nil {
		t.Fatal("expected the delegated save error")
	}
}

func TestEditSaveEventRemoteLookupError(t *testing.T) {
	sync := &fakeSyncStore{remoteErr: errBoom}
	if _, err := editService(sync, &fakeCalendarStore{}, "x").SaveEvent(context.Background(), simpleEventInput("cal")); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

func TestEditSaveEventRemoteBuildError(t *testing.T) {
	sync := remoteOnly("cal", "/dav/work")
	in := EventInput{CalendarID: "cal"} // no summary or start, so the domain event fails to build
	if _, err := editService(sync, &fakeCalendarStore{}, "x").SaveEvent(context.Background(), in); err == nil {
		t.Fatal("expected a build error for an invalid event")
	}
}

func TestEditSaveEventRemoteIdentityError(t *testing.T) {
	sync := remoteOnly("cal", "/dav/work")
	sync.identityErr = errBoom
	if _, err := editService(sync, &fakeCalendarStore{}, "x").SaveEvent(context.Background(), simpleEventInput("cal")); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

func TestEditSaveEventRemoteCreateRecordsIntent(t *testing.T) {
	sync := remoteOnly("cal", "/dav/work/") // trailing slash must not double up in the object href
	in := simpleEventInput("cal")
	in.UID = "uid with space" // an unusual UID must be percent-escaped into the href
	id, err := editService(sync, &fakeCalendarStore{}, "gen").SaveEvent(context.Background(), in)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if id != "gen" {
		t.Errorf("id = %q, want gen", id)
	}
	if len(sync.savedPend) != 1 {
		t.Fatalf("want one atomic save-with-pending, got %+v", sync.savedPend)
	}
	got := sync.savedPend[0]
	const wantHref = "/dav/work/uid%20with%20space.ics"
	if got.href != wantHref || got.etag != "" {
		t.Errorf("saved identity href=%q etag=%q, want %q and empty etag", got.href, got.etag, wantHref)
	}
	if got.op.Op != CalendarOpCreate || got.op.Href != wantHref || got.op.BaseETag != "" || got.op.CalendarID != "cal" {
		t.Errorf("pending op = %+v, want a create for cal at %q", got.op, wantHref)
	}
}

func TestEditSaveEventRemoteUpdateRecordsIntent(t *testing.T) {
	sync := remoteOnly("cal", "/dav/work")
	sync.identities = map[string]SyncedEventIdentity{
		"e9": {Href: "/dav/work/e9.ics", ETag: "srv7", CalendarID: "cal"},
	}
	in := simpleEventInput("cal")
	in.ID = "e9"
	id, err := editService(sync, &fakeCalendarStore{}, "unused").SaveEvent(context.Background(), in)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if id != "e9" {
		t.Errorf("id = %q, want e9", id)
	}
	if len(sync.savedPend) != 1 {
		t.Fatalf("want one atomic save-with-pending, got %+v", sync.savedPend)
	}
	got := sync.savedPend[0]
	if got.href != "/dav/work/e9.ics" || got.etag != "srv7" {
		t.Errorf("update identity = %+v, want the stored href and etag", got)
	}
	if got.op.Op != CalendarOpUpdate || got.op.BaseETag != "srv7" || got.op.Href != "/dav/work/e9.ics" || got.op.CalendarID != "cal" {
		t.Errorf("pending op = %+v, want an update guarded on srv7", got.op)
	}
}

func TestEditSaveEventRemoteCreateSaveError(t *testing.T) {
	sync := remoteOnly("cal", "/dav/work")
	sync.savePendErr = errBoom
	if _, err := editService(sync, &fakeCalendarStore{}, "x").SaveEvent(context.Background(), simpleEventInput("cal")); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

func TestEditSaveEventRemoteUpdateSaveError(t *testing.T) {
	sync := remoteOnly("cal", "/dav/work")
	sync.identities = map[string]SyncedEventIdentity{"e9": {Href: "/dav/work/e9.ics", ETag: "srv7", CalendarID: "cal"}}
	sync.savePendErr = errBoom
	in := simpleEventInput("cal")
	in.ID = "e9"
	if _, err := editService(sync, &fakeCalendarStore{}, "x").SaveEvent(context.Background(), in); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

func TestEditDeleteEventIdentityError(t *testing.T) {
	sync := &fakeSyncStore{identityErr: errBoom}
	if err := editService(sync, &fakeCalendarStore{}, "x").DeleteEvent(context.Background(), "e1"); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

func TestEditDeleteEventLocalDelegates(t *testing.T) {
	sync := &fakeSyncStore{} // no identity => a local event
	cal := &fakeCalendarStore{}
	if err := editService(sync, cal, "x").DeleteEvent(context.Background(), "e1"); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if len(cal.deletedEvt) != 1 || cal.deletedEvt[0] != "e1" {
		t.Errorf("a local delete should go through the plain service: %+v", cal.deletedEvt)
	}
	if len(sync.deletedPend) != 0 {
		t.Errorf("no tombstone for a local event: %+v", sync.deletedPend)
	}
}

func TestEditDeleteEventLocalRowWithNoHrefDelegates(t *testing.T) {
	// A row that exists but carries no href is a local-only or safety-copy event, not a remote object.
	sync := &fakeSyncStore{identities: map[string]SyncedEventIdentity{"e1": {Href: "", CalendarID: "local"}}}
	cal := &fakeCalendarStore{}
	if err := editService(sync, cal, "x").DeleteEvent(context.Background(), "e1"); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if len(cal.deletedEvt) != 1 {
		t.Errorf("a local-only row should be deleted plainly: %+v", cal.deletedEvt)
	}
	if len(sync.deletedPend) != 0 {
		t.Errorf("no tombstone for a local-only event: %+v", sync.deletedPend)
	}
}

func TestEditDeleteEventLocalDelegateError(t *testing.T) {
	cal := &fakeCalendarStore{delEvtErr: errBoom}
	if err := editService(&fakeSyncStore{}, cal, "x").DeleteEvent(context.Background(), "e1"); err == nil {
		t.Fatal("expected the delegated delete error")
	}
}

func TestEditDeleteEventRemoteRecordsTombstone(t *testing.T) {
	sync := &fakeSyncStore{identities: map[string]SyncedEventIdentity{
		"e1": {Href: "/dav/work/e1.ics", ETag: "etg", CalendarID: "cal"},
	}}
	if err := editService(sync, &fakeCalendarStore{}, "x").DeleteEvent(context.Background(), "e1"); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if len(sync.deletedPend) != 1 {
		t.Fatalf("want one atomic delete-with-pending, got %+v", sync.deletedPend)
	}
	got := sync.deletedPend[0]
	if got.href != "/dav/work/e1.ics" {
		t.Errorf("deleted href = %q", got.href)
	}
	if got.op.Op != CalendarOpDelete || got.op.BaseETag != "etg" || got.op.CalendarID != "cal" || got.op.Href != "/dav/work/e1.ics" {
		t.Errorf("pending op = %+v, want a delete tombstone guarded on etg", got.op)
	}
}

func TestEditDeleteEventRemoteDeleteError(t *testing.T) {
	sync := &fakeSyncStore{
		identities: map[string]SyncedEventIdentity{"e1": {Href: "/x/e1.ics", ETag: "e", CalendarID: "cal"}},
		delPendErr: errBoom,
	}
	if err := editService(sync, &fakeCalendarStore{}, "x").DeleteEvent(context.Background(), "e1"); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}
