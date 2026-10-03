package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// A CalDAV sync reports what it could not do: a push that failed, a collection that could not be read or
// an object that did not land reaches the caller (and so the calendar's error banner) instead of the sync
// reading as a success. A conflict is not a failure: the reconcile settles it. Events decoded from a server
// object are keyed by UID and RECURRENCE-ID so a series and its overrides keep a row each.

// surfaceOccurrence is the original start of the occurrence the override fixtures replace.
var surfaceOccurrence = time.Date(2026, 7, 11, 9, 0, 0, 0, time.UTC)

// surfaceEvent builds an event with the given id, UID and recurrence id.
func surfaceEvent(t *testing.T, id, uid string, recurrenceID time.Time) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{
		ID: id, UID: uid, Summary: "S", Start: time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC), RecurrenceID: recurrenceID,
	})
	if err != nil {
		t.Fatalf("event %q: %v", id, err)
	}
	return e
}

// pendingPut is a store holding one pending update of href in cal1 whose object holds the given events.
func pendingPut(href string, events ...domain.Event) *fakeSyncStore {
	return &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "old"}},
		eventsByHref: map[string][]domain.Event{href: events},
	}
}

func flushCal1(svc *CalDAVWriteService, writer CalDAVWriter) error {
	return svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
}

func TestFlushReportsAFailedPush(t *testing.T) {
	store := pendingPut("/c/a.ics", writeEvent(t, "e1"))
	err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")}), &fakeWriter{putErr: errBoom})
	if !errors.Is(err, errBoom) {
		t.Errorf("Flush err = %v, want the push failure", err)
	}
}

func TestFlushConflictIsNotAFailure(t *testing.T) {
	store := pendingPut("/c/a.ics", writeEvent(t, "e1"))
	if err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")}), &fakeWriter{putErr: ErrCalDAVConflict}); err != nil {
		t.Errorf("Flush err = %v, want nil: the reconcile settles a conflict", err)
	}
}

func TestFlushReportsUnreadableAndUnencodableObjects(t *testing.T) {
	store := pendingPut("/c/a.ics", writeEvent(t, "e1"))
	store.eventsErr = errBoom
	if err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{}), &fakeWriter{}); !errors.Is(err, errBoom) {
		t.Errorf("read failure: Flush err = %v, want errBoom", err)
	}
	store = pendingPut("/c/a.ics", writeEvent(t, "e1"))
	if err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{encodeErr: errBoom}), &fakeWriter{}); !errors.Is(err, errBoom) {
		t.Errorf("encode failure: Flush err = %v, want errBoom", err)
	}
}

func TestFlushReportsBookkeepingFailuresAfterAConfirmedPush(t *testing.T) {
	// The server accepted the write, so the intent is still cleared; failing to record that locally is
	// reported rather than dropped.
	store := pendingPut("/c/a.ics", writeEvent(t, "e1"))
	store.saveSyncedErr = errBoom
	store.clearErr = errBoom
	err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")}), &fakeWriter{putETag: "new"})
	if !errors.Is(err, errBoom) {
		t.Errorf("Flush err = %v, want the bookkeeping failure", err)
	}
	if len(store.clearedOps) != 1 {
		t.Errorf("the confirmed write must still clear the intent: %+v", store.clearedOps)
	}
}

func TestFlushReportsAFailedDeleteAndItsCleanup(t *testing.T) {
	op := []PendingCalendarObject{{CalendarID: "cal1", Href: "/c/d.ics", Op: CalendarOpDelete, BaseETag: "e"}}
	store := &fakeSyncStore{pending: op}
	if err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{}), &fakeWriter{delErr: errBoom}); !errors.Is(err, errBoom) {
		t.Errorf("delete failure: Flush err = %v, want errBoom", err)
	}
	store = &fakeSyncStore{pending: op, deleteHrefErr: errBoom}
	if err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{}), &fakeWriter{}); !errors.Is(err, errBoom) {
		t.Errorf("cleanup failure: Flush err = %v, want errBoom", err)
	}
}

func TestFlushRefusesToPushACollapsedSeries(t *testing.T) {
	// A row stored before overrides had their own key: an override under the bare UID, the master it
	// overwrote gone. Pushing it would replace the server's whole series with one occurrence.
	store := pendingPut("/c/s.ics", surfaceEvent(t, "u1", "u1", surfaceOccurrence))
	writer := &fakeWriter{putETag: "new"}
	err := flushCal1(NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")}), writer)
	if !errors.Is(err, ErrCalDAVCollapsedSeries) {
		t.Errorf("Flush err = %v, want ErrCalDAVCollapsedSeries", err)
	}
	if writer.putCalls != 0 || len(store.clearedOps) != 0 {
		t.Errorf("nothing may be pushed or cleared: puts=%d cleared=%+v", writer.putCalls, store.clearedOps)
	}
}

func TestFlushPushesAMasterWithItsOverride(t *testing.T) {
	master := surfaceEvent(t, "u1", "u1", time.Time{})
	override := surfaceEvent(t, domain.EventIDFor("u1", surfaceOccurrence), "u1", surfaceOccurrence)
	store := pendingPut("/c/s.ics", master, override)
	codec := &fakeCalendarCodec{encoded: []byte("X")}
	writer := &fakeWriter{putETag: "new"}
	if err := flushCal1(NewCalDAVWriteService(store, codec), writer); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if writer.putCalls != 1 || len(codec.gotEncode) != 2 {
		t.Errorf("want one push of both events: puts=%d encoded=%d", writer.putCalls, len(codec.gotEncode))
	}
}

func TestReconcileReportsWhatDidNotLand(t *testing.T) {
	ctx := context.Background()
	records := []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "old")}
	obj := map[string][]RemoteObject{"/c1/": {{Href: "/c1/a.ics", ETag: "e2", Data: []byte("A")}}}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"A": {writeEvent(t, "a")}}}
	cases := []struct {
		name   string
		source *fakeReconcileSource
		store  *fakeSyncStore
		codec  *reconcileCodec
	}{
		{"unreadable collection", &fakeReconcileSource{listErr: map[string]error{"/c1/": errBoom}}, &fakeSyncStore{}, codec},
		{"unreadable local objects", &fakeReconcileSource{objects: obj}, &fakeSyncStore{syncedErr: errBoom}, codec},
		{"object failed to save", &fakeReconcileSource{objects: obj}, &fakeSyncStore{saveSyncedErr: errBoom}, codec},
		{"undecodable object", &fakeReconcileSource{objects: obj}, &fakeSyncStore{},
			&reconcileCodec{decodeErr: map[string]error{"A": errBoom}}},
		{"ctag not recorded", &fakeReconcileSource{objects: obj, ctag: map[string]string{"/c1/": "new"}},
			&fakeSyncStore{updateCTagErr: errBoom}, codec},
		{"server removal not applied", &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": nil}},
			&fakeSyncStore{synced: map[string][]SyncedObject{"cal1": {{Href: "/c1/gone.ics", ETag: "e"}}}, deleteHrefErr: errBoom}, codec},
	}
	for _, c := range cases {
		err := NewCalDAVReconcileService(c.store, c.codec, seqID()).Reconcile(ctx, c.source, records)
		if !errors.Is(err, errBoom) {
			t.Errorf("%s: Reconcile err = %v, want the failure", c.name, err)
		}
	}
}

func TestReconcileKeysASeriesAndItsOverrideApart(t *testing.T) {
	// The codec keyed both events by the bare UID; saving them as decoded would let the override overwrite
	// the master. A UID-less event keeps the id it was decoded with.
	decoded := []domain.Event{
		surfaceEvent(t, "u1", "u1", time.Time{}),
		surfaceEvent(t, "u1", "u1", surfaceOccurrence),
		surfaceEvent(t, "no-uid", "", time.Time{}),
	}
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/s.ics", ETag: "e", Data: []byte("S")}}}}
	store := &fakeSyncStore{}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"S": decoded}}
	if err := NewCalDAVReconcileService(store, codec, seqID()).Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	want := []string{"u1", domain.EventIDFor("u1", surfaceOccurrence), "no-uid"}
	if len(store.saved) != len(want) {
		t.Fatalf("saved %d events, want %d", len(store.saved), len(want))
	}
	for i, id := range want {
		if store.saved[i].id != id {
			t.Errorf("saved[%d] id = %q, want %q", i, store.saved[i].id, id)
		}
	}
}

func TestCalDAVSyncReportsAFailedFlushAndReconcile(t *testing.T) {
	// Listing the pending ops fails, which breaks both halves. Discovery still runs; the sync must still say
	// it did not complete rather than read as a success.
	accts, creds := syncAccounts(t)
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}}}
	store := &fakeSyncStore{pendingErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: &fakeWriter{}}, &davCodec{}, store, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); !errors.Is(err, errBoom) {
		t.Errorf("Sync err = %v, want the flush and reconcile failure", err)
	}
	if len(store.savedCals) != 1 {
		t.Errorf("discovery must still run: %+v", store.savedCals)
	}
}

func TestCalDAVSyncReportsAFailedPush(t *testing.T) {
	accts, creds := syncAccounts(t)
	const href = "/a/new.ics"
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}}}
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "c1|/a", Href: href, Op: CalendarOpCreate}},
		eventsByHref: map[string][]domain.Event{href: {davEvent(t, "e1")}},
		remoteCals:   []RemoteCalendarRecord{{CalendarID: "c1|/a", AccountID: "c1", Href: "/a", Name: "A"}},
	}
	writer := &fakeWriter{putErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: writer}, &davCodec{}, store, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); !errors.Is(err, errBoom) {
		t.Errorf("Sync err = %v, want the push failure", err)
	}
}

func TestCalDAVSyncReportsAnUnscopableFlushAndAFailedDiscovery(t *testing.T) {
	// Both the skipped flush and the discovery failure reach the caller.
	accts, creds := syncAccounts(t)
	discoverErr := errors.New("discover failed")
	src := &fakeCalDAVSource{listCalErr: discoverErr}
	store := &fakeSyncStore{remoteCalsErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: &fakeWriter{}}, &davCodec{}, store, fixedID("x"))
	err := svc.Sync(context.Background(), "c1")
	if !errors.Is(err, errBoom) || !errors.Is(err, discoverErr) {
		t.Errorf("Sync err = %v, want both the scoping and the discovery failure", err)
	}
}
