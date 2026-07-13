package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// savedSynced records a SaveSyncedEvent call so a test can assert the etag stamped after a successful push.
type savedSynced struct {
	id, href, etag string
}

// savedRemoteCalendar records a SaveRemoteCalendar call so a pull test can assert the mirrored collection.
type savedRemoteCalendar struct {
	id, name, colour, accountID, href, ctag string
}

// savedWithPending records a SaveEventWithPending call so a test can assert the event, its stamped identity and
// the recorded intent.
type savedWithPending struct {
	id, href, etag string
	op             PendingCalendarObject
}

// deletedWithPending records a DeleteObjectWithPending call.
type deletedWithPending struct {
	href string
	op   PendingCalendarObject
}

// fakeSyncStore is a hand-written CalendarSyncStore with error injection and call recording. Only the methods a
// test exercises carry behaviour; the rest are interface-satisfying stubs.
type fakeSyncStore struct {
	pending       []PendingCalendarObject
	pendingErr    error
	eventsByHref  map[string][]domain.Event
	eventsErr     error
	synced        map[string][]SyncedObject
	syncedErr     error
	saved         []savedSynced
	savedEvents   []domain.Event
	saveSyncedErr error
	deletedHrefs  []string
	clearedOps    [][2]string
	savedCals     []savedRemoteCalendar
	saveCalErr    error
	// remoteByID keys a calendar id to its remote record; absence reports a local-only (or unknown) calendar.
	remoteByID  map[string]RemoteCalendarRecord
	remoteErr   error
	identities  map[string]SyncedEventIdentity
	identityErr error
	savedPend   []savedWithPending
	savePendErr error
	deletedPend []deletedWithPending
	delPendErr  error
	// ctagByID feeds CalendarCTag; calendarCTagErr injects a read failure; updatedCTags records
	// UpdateCalendarCTag calls as (calendarID, ctag) pairs.
	ctagByID        map[string]string
	calendarCTagErr error
	updatedCTags    [][2]string
	remoteCals      []RemoteCalendarRecord
	remoteCalsErr   error
}

var _ CalendarSyncStore = (*fakeSyncStore)(nil)

func (f *fakeSyncStore) ListPendingCalendarOps(context.Context) ([]PendingCalendarObject, error) {
	return f.pending, f.pendingErr
}

func (f *fakeSyncStore) EventsByHref(_ context.Context, href string) ([]domain.Event, error) {
	if f.eventsErr != nil {
		return nil, f.eventsErr
	}
	return f.eventsByHref[href], nil
}

func (f *fakeSyncStore) SaveSyncedEvent(_ context.Context, e domain.Event, href, etag string) error {
	if f.saveSyncedErr != nil {
		return f.saveSyncedErr
	}
	f.saved = append(f.saved, savedSynced{e.ID(), href, etag})
	f.savedEvents = append(f.savedEvents, e)
	return nil
}

func (f *fakeSyncStore) DeleteEventsByHref(_ context.Context, href string) error {
	f.deletedHrefs = append(f.deletedHrefs, href)
	return nil
}

func (f *fakeSyncStore) ClearPendingCalendarOp(_ context.Context, calendarID, href string) error {
	f.clearedOps = append(f.clearedOps, [2]string{calendarID, href})
	return nil
}

func (f *fakeSyncStore) ListSyncedObjects(_ context.Context, calendarID string) ([]SyncedObject, error) {
	if f.syncedErr != nil {
		return nil, f.syncedErr
	}
	return f.synced[calendarID], nil
}
func (f *fakeSyncStore) SaveRemoteCalendar(_ context.Context, c domain.Calendar, accountID, href, ctag string) error {
	if f.saveCalErr != nil {
		return f.saveCalErr
	}
	f.savedCals = append(f.savedCals, savedRemoteCalendar{c.ID(), c.Name(), c.Colour(), accountID, href, ctag})
	return nil
}
func (f *fakeSyncStore) ListRemoteCalendars(context.Context, string) ([]RemoteCalendarRecord, error) {
	return f.remoteCals, f.remoteCalsErr
}
func (f *fakeSyncStore) CalendarCTag(_ context.Context, calendarID string) (string, error) {
	if f.calendarCTagErr != nil {
		return "", f.calendarCTagErr
	}
	return f.ctagByID[calendarID], nil
}

func (f *fakeSyncStore) UpdateCalendarCTag(_ context.Context, calendarID, ctag string) error {
	f.updatedCTags = append(f.updatedCTags, [2]string{calendarID, ctag})
	return nil
}

func (f *fakeSyncStore) RemoteCalendarByID(_ context.Context, calendarID string) (RemoteCalendarRecord, bool, error) {
	if f.remoteErr != nil {
		return RemoteCalendarRecord{}, false, f.remoteErr
	}
	record, ok := f.remoteByID[calendarID]
	if !ok {
		return RemoteCalendarRecord{}, false, nil
	}
	return record, record.AccountID != "", nil
}

func (f *fakeSyncStore) SyncedEventIdentity(_ context.Context, eventID string) (SyncedEventIdentity, bool, error) {
	if f.identityErr != nil {
		return SyncedEventIdentity{}, false, f.identityErr
	}
	identity, ok := f.identities[eventID]
	if !ok {
		return SyncedEventIdentity{}, false, nil
	}
	return identity, true, nil
}

func (f *fakeSyncStore) SaveEventWithPending(_ context.Context, e domain.Event, href, etag string, op PendingCalendarObject) error {
	if f.savePendErr != nil {
		return f.savePendErr
	}
	f.savedPend = append(f.savedPend, savedWithPending{e.ID(), href, etag, op})
	return nil
}

func (f *fakeSyncStore) DeleteObjectWithPending(_ context.Context, href string, op PendingCalendarObject) error {
	if f.delPendErr != nil {
		return f.delPendErr
	}
	f.deletedPend = append(f.deletedPend, deletedWithPending{href, op})
	return nil
}

// fakeWriter is a hand-written CalDAVWriter recording the conditional headers and returning an injected etag
// or error.
type fakeWriter struct {
	putETag        string
	putErr         error
	delErr         error
	putHref        string
	putIfMatch     string
	putIfNoneMatch string
	putBody        []byte
	delHref        string
	delIfMatch     string
	putCalls       int
	delCalls       int
}

var _ CalDAVWriter = (*fakeWriter)(nil)

func (f *fakeWriter) PutObject(_ context.Context, href string, ics []byte, ifMatch, ifNoneMatch string) (string, error) {
	f.putCalls++
	f.putHref, f.putBody, f.putIfMatch, f.putIfNoneMatch = href, ics, ifMatch, ifNoneMatch
	return f.putETag, f.putErr
}

func (f *fakeWriter) DeleteObject(_ context.Context, href, ifMatch string) error {
	f.delCalls++
	f.delHref, f.delIfMatch = href, ifMatch
	return f.delErr
}

func writeEvent(t *testing.T, id string) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{ID: id, Summary: "S", Start: time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("event %q: %v", id, err)
	}
	return e
}

func TestFlushListError(t *testing.T) {
	svc := NewCalDAVWriteService(&fakeSyncStore{pendingErr: errBoom}, &fakeCalendarCodec{})
	if err := svc.Flush(context.Background(), &fakeWriter{}, map[string]bool{"cal1": true}); !errors.Is(err, errBoom) {
		t.Fatalf("Flush err = %v, want errBoom", err)
	}
}

func TestFlushSkipsOtherAccountsOps(t *testing.T) {
	// A pending op for a calendar the synced account does not own must not be pushed through this account's
	// writer: doing so would send another account's object to the wrong server under the wrong credentials.
	const href = "https://dav.example.com/other/z.ics"
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "otherAccount|/c", Href: href, Op: CalendarOpCreate}},
		eventsByHref: map[string][]domain.Event{href: {writeEvent(t, "z")}},
	}
	writer := &fakeWriter{putETag: "e"}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")})
	// The set names only THIS account's calendar, not otherAccount's.
	if err := svc.Flush(context.Background(), writer, map[string]bool{"cal1": true}); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if writer.putCalls != 0 || len(store.clearedOps) != 0 {
		t.Errorf("another account's op must be skipped: puts=%d cleared=%+v", writer.putCalls, store.clearedOps)
	}
}

func TestFlushCreatePut(t *testing.T) {
	const href = "https://dav.example.com/cal1/a.ics"
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpCreate}},
		eventsByHref: map[string][]domain.Event{href: {writeEvent(t, "e1")}},
	}
	writer := &fakeWriter{putETag: "srv-etag"}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("BEGIN:VCALENDAR")})
	if err := svc.Flush(context.Background(), writer, map[string]bool{"cal1": true}); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if writer.putIfNoneMatch != "*" || writer.putIfMatch != "" {
		t.Errorf("create headers: none=%q match=%q", writer.putIfNoneMatch, writer.putIfMatch)
	}
	if string(writer.putBody) != "BEGIN:VCALENDAR" {
		t.Errorf("body = %q", writer.putBody)
	}
	if len(store.saved) != 1 || store.saved[0] != (savedSynced{"e1", href, "srv-etag"}) {
		t.Errorf("saved = %+v", store.saved)
	}
	if len(store.clearedOps) != 1 || store.clearedOps[0] != [2]string{"cal1", href} {
		t.Errorf("cleared = %+v", store.clearedOps)
	}
}

func TestFlushUpdateWithoutReturnedEtagStillClears(t *testing.T) {
	const href = "https://dav.example.com/cal1/b.ics"
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "old"}},
		eventsByHref: map[string][]domain.Event{href: {writeEvent(t, "e2")}},
	}
	writer := &fakeWriter{putETag: ""} // the server omitted an ETag on the response
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")})
	_ = svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
	if writer.putIfMatch != "old" || writer.putIfNoneMatch != "" {
		t.Errorf("update headers: match=%q none=%q", writer.putIfMatch, writer.putIfNoneMatch)
	}
	if len(store.saved) != 0 {
		t.Errorf("no etag returned, must not re-save: %+v", store.saved)
	}
	if len(store.clearedOps) != 1 {
		t.Errorf("a confirmed write clears the intent: %+v", store.clearedOps)
	}
}

func TestFlushPutConflictLeavesIntent(t *testing.T) {
	const href = "https://dav.example.com/cal1/c.ics"
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "old"}},
		eventsByHref: map[string][]domain.Event{href: {writeEvent(t, "e3")}},
	}
	writer := &fakeWriter{putErr: ErrCalDAVConflict}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")})
	_ = svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
	if len(store.clearedOps) != 0 || len(store.saved) != 0 {
		t.Errorf("a conflict must leave the intent: cleared=%+v saved=%+v", store.clearedOps, store.saved)
	}
}

func TestFlushPutSkipsWhenNoEvents(t *testing.T) {
	const href = "https://dav.example.com/cal1/d.ics"
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate}},
		eventsByHref: map[string][]domain.Event{}, // the object vanished locally
	}
	writer := &fakeWriter{}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")})
	_ = svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
	if writer.putCalls != 0 || len(store.clearedOps) != 0 {
		t.Errorf("empty events should skip the put: puts=%d cleared=%+v", writer.putCalls, store.clearedOps)
	}
}

func TestFlushPutEventsError(t *testing.T) {
	store := &fakeSyncStore{
		pending:   []PendingCalendarObject{{CalendarID: "cal1", Href: "h", Op: CalendarOpCreate}},
		eventsErr: errBoom,
	}
	writer := &fakeWriter{}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{encoded: []byte("X")})
	_ = svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
	if writer.putCalls != 0 {
		t.Errorf("an events-read error should skip the put")
	}
}

func TestFlushPutEncodeError(t *testing.T) {
	const href = "h"
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpCreate}},
		eventsByHref: map[string][]domain.Event{href: {writeEvent(t, "e")}},
	}
	writer := &fakeWriter{}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{encodeErr: errBoom})
	_ = svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
	if writer.putCalls != 0 {
		t.Errorf("an encode error should skip the put")
	}
}

func TestFlushDelete(t *testing.T) {
	const href = "https://dav.example.com/cal1/e.ics"
	store := &fakeSyncStore{
		pending: []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpDelete, BaseETag: "etg"}},
	}
	writer := &fakeWriter{}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{})
	_ = svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
	if writer.delIfMatch != "etg" || writer.delHref != href {
		t.Errorf("delete request: href=%q match=%q", writer.delHref, writer.delIfMatch)
	}
	if len(store.deletedHrefs) != 1 || store.deletedHrefs[0] != href {
		t.Errorf("local remnant not removed: %+v", store.deletedHrefs)
	}
	if len(store.clearedOps) != 1 {
		t.Errorf("delete intent not cleared: %+v", store.clearedOps)
	}
}

func TestFlushDeleteConflictLeavesIntent(t *testing.T) {
	store := &fakeSyncStore{
		pending: []PendingCalendarObject{{CalendarID: "cal1", Href: "h", Op: CalendarOpDelete, BaseETag: "e"}},
	}
	writer := &fakeWriter{delErr: ErrCalDAVConflict}
	svc := NewCalDAVWriteService(store, &fakeCalendarCodec{})
	_ = svc.Flush(context.Background(), writer, map[string]bool{"cal1": true})
	if len(store.deletedHrefs) != 0 || len(store.clearedOps) != 0 {
		t.Errorf("a delete conflict must leave the intent: deleted=%+v cleared=%+v", store.deletedHrefs, store.clearedOps)
	}
}
