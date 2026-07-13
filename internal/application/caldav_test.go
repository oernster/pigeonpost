package application

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

type fakeCalDAVSource struct {
	calendars  []RemoteCalendar
	listCalErr error
	objects    map[string][]RemoteObject
	listObjErr map[string]error
	ctag       map[string]string
}

func (f *fakeCalDAVSource) ListCalendars(context.Context) ([]RemoteCalendar, error) {
	if f.listCalErr != nil {
		return nil, f.listCalErr
	}
	return f.calendars, nil
}

func (f *fakeCalDAVSource) ListObjects(_ context.Context, c RemoteCalendar) ([]RemoteObject, error) {
	if err := f.listObjErr[c.Path]; err != nil {
		return nil, err
	}
	return f.objects[c.Path], nil
}

func (f *fakeCalDAVSource) CollectionCTag(_ context.Context, href string) (string, error) {
	return f.ctag[href], nil
}

// davCodec decodes an object body by looking its bytes up in decode; a body absent from the map
// yields a decode error, which the sync must skip.
type davCodec struct {
	decode map[string][]domain.Event
}

func (f *davCodec) Decode(data []byte) ([]domain.Event, []domain.CalendarPassthrough, error) {
	events, ok := f.decode[string(data)]
	if !ok {
		return nil, nil, errBoom
	}
	return events, nil, nil
}

func (f *davCodec) Encode([]domain.Event, []domain.CalendarPassthrough) ([]byte, error) {
	return nil, nil
}

func davEvent(t *testing.T, id string) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{ID: id, CalendarID: "cal1", Summary: "Ev", Start: day(4, 9)})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return e
}

const pullAccount = "acc1"

func TestCalDAVPullListCalendarsError(t *testing.T) {
	svc := NewCalDAVSyncService(&fakeCalDAVSource{listCalErr: errBoom}, &davCodec{}, &fakeSyncStore{}, pullAccount)
	if _, err := svc.Pull(context.Background()); err == nil {
		t.Fatal("expected an error when listing calendars fails")
	}
}

func TestCalDAVPullSaveCalendarErrorIsFatal(t *testing.T) {
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}}}
	svc := NewCalDAVSyncService(src, &davCodec{}, &fakeSyncStore{saveCalErr: errBoom}, pullAccount)
	if _, err := svc.Pull(context.Background()); err == nil {
		t.Fatal("expected a fatal error when saving a mirrored calendar fails")
	}
}

func TestCalDAVDiscoverPreservesCTag(t *testing.T) {
	// A re-discovery must carry the CTag the last sync recorded, not wipe it, so the reconcile can still skip an
	// unchanged collection.
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}}}
	store := &fakeSyncStore{ctagByID: map[string]string{pullAccount + "|/a": "stored-ctag"}}
	records, err := NewCalDAVSyncService(src, &davCodec{}, store, pullAccount).Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(records) != 1 || records[0].CTag != "stored-ctag" {
		t.Errorf("record ctag = %+v, want the stored ctag preserved", records)
	}
	if len(store.savedCals) != 1 || store.savedCals[0].ctag != "stored-ctag" {
		t.Errorf("saved calendar ctag = %+v, want the stored ctag preserved, not wiped", store.savedCals)
	}
}

func TestCalDAVDiscoverCTagReadError(t *testing.T) {
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "/a"}}}
	store := &fakeSyncStore{calendarCTagErr: errBoom}
	if _, err := NewCalDAVSyncService(src, &davCodec{}, store, pullAccount).Discover(context.Background()); err == nil {
		t.Fatal("expected an error when the stored ctag cannot be read")
	}
}

func TestCalDAVPullBuildCalendarErrorIsFatal(t *testing.T) {
	// A collection with neither a display name nor a resource path cannot form a valid calendar name, so the
	// mirror cannot be built and the pull fails rather than saving a nameless calendar.
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "", DisplayName: ""}}}
	svc := NewCalDAVSyncService(src, &davCodec{}, &fakeSyncStore{}, pullAccount)
	if _, err := svc.Pull(context.Background()); err == nil {
		t.Fatal("expected a fatal error when a collection cannot form a valid calendar")
	}
}

func TestCalDAVPullSkipsCalendarWhoseObjectsFail(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars:  []RemoteCalendar{{Path: "/a"}, {Path: "/b"}},
		listObjErr: map[string]error{"/a": errBoom},
		objects:    map[string][]RemoteObject{"/b": {{Href: "/b/1", ETag: "e1", Data: []byte("EV1")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV1": {davEvent(t, "e1")}}}
	store := &fakeSyncStore{}
	n, err := NewCalDAVSyncService(src, codec, store, pullAccount).Pull(context.Background())
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 1 {
		t.Errorf("saved = %d, want 1 (calendar /a skipped, /b pulled)", n)
	}
	// both collections are still mirrored: /a is saved before its objects are listed and fail.
	if len(store.savedCals) != 2 {
		t.Errorf("mirrored %d calendars, want 2", len(store.savedCals))
	}
	if len(store.saved) != 1 || store.saved[0].href != "/b/1" || store.saved[0].etag != "e1" {
		t.Errorf("saved object identity = %+v", store.saved)
	}
}

func TestCalDAVPullSkipsUndecodableObject(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a"}},
		objects:   map[string][]RemoteObject{"/a": {{Href: "/a/bad", Data: []byte("BAD")}, {Href: "/a/good", ETag: "g", Data: []byte("GOOD")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"GOOD": {davEvent(t, "e1")}}}
	n, err := NewCalDAVSyncService(src, codec, &fakeSyncStore{}, pullAccount).Pull(context.Background())
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 1 {
		t.Errorf("saved = %d, want 1 (undecodable object skipped)", n)
	}
}

func TestCalDAVPullSaveEventErrorIsFatal(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a"}},
		objects:   map[string][]RemoteObject{"/a": {{Href: "/a/1", Data: []byte("EV")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV": {davEvent(t, "e1")}}}
	svc := NewCalDAVSyncService(src, codec, &fakeSyncStore{saveSyncedErr: errBoom}, pullAccount)
	if _, err := svc.Pull(context.Background()); err == nil {
		t.Fatal("expected a fatal error when saving an event fails")
	}
}

func TestCalDAVPullMirrorsCollectionsAndTagsEvents(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/work", DisplayName: "Work"}, {Path: "/home"}},
		objects: map[string][]RemoteObject{
			"/work": {{Href: "/work/1.ics", ETag: "w1", Data: []byte("EV")}},
		},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV": {davEvent(t, "e1"), davEvent(t, "e2")}}}
	store := &fakeSyncStore{}
	n, err := NewCalDAVSyncService(src, codec, store, pullAccount).Pull(context.Background())
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 2 {
		t.Errorf("saved = %d, want 2", n)
	}
	if len(store.savedCals) != 2 {
		t.Fatalf("mirrored %d calendars, want 2", len(store.savedCals))
	}
	work := store.savedCals[0]
	wantID := pullAccount + "|/work"
	if work.id != wantID || work.name != "Work" || work.accountID != pullAccount || work.href != "/work" || work.colour != DefaultRemoteCalendarColour {
		t.Errorf("work calendar = %+v, want id %q name Work", work, wantID)
	}
	// a collection with no display name falls back to its resource path for the name.
	if home := store.savedCals[1]; home.name != "/home" {
		t.Errorf("home name = %q, want fallback to path", home.name)
	}
	// every event is tagged with its object's href and etag and associated with the mirrored calendar.
	for _, s := range store.saved {
		if s.href != "/work/1.ics" || s.etag != "w1" {
			t.Errorf("event %q identity = %+v", s.id, s)
		}
	}
	for _, e := range store.savedEvents {
		if e.CalendarID() != wantID {
			t.Errorf("event %q calendar = %q, want %q", e.ID(), e.CalendarID(), wantID)
		}
	}
}
