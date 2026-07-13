package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeReconcileSource is a CalDAVSource returning canned objects per collection path, with per-collection
// ListObjects error injection and per-collection CTag stubbing.
type fakeReconcileSource struct {
	objects map[string][]RemoteObject
	listErr map[string]error
	ctag    map[string]string
	ctagErr map[string]error
}

var _ CalDAVSource = (*fakeReconcileSource)(nil)

func (f *fakeReconcileSource) ListCalendars(context.Context) ([]RemoteCalendar, error) {
	return nil, nil
}
func (f *fakeReconcileSource) ListObjects(_ context.Context, c RemoteCalendar) ([]RemoteObject, error) {
	if err := f.listErr[c.Path]; err != nil {
		return nil, err
	}
	return f.objects[c.Path], nil
}
func (f *fakeReconcileSource) CollectionCTag(_ context.Context, href string) (string, error) {
	if err := f.ctagErr[href]; err != nil {
		return "", err
	}
	return f.ctag[href], nil
}

// recWithCTag is rec with a last-synced CTag, for the collection-skip tests.
func recWithCTag(calendarID, collectionHref, ctag string) RemoteCalendarRecord {
	r := rec(calendarID, collectionHref)
	r.CTag = ctag
	return r
}

// reconcileCodec decodes a server object body to a canned event slice keyed by the raw bytes, with per-body
// decode error injection. Encode is unused by Reconcile.
type reconcileCodec struct {
	byData    map[string][]domain.Event
	decodeErr map[string]error
}

var _ CalendarCodec = (*reconcileCodec)(nil)

func (c *reconcileCodec) Decode(data []byte) ([]domain.Event, []domain.CalendarPassthrough, error) {
	if err := c.decodeErr[string(data)]; err != nil {
		return nil, nil, err
	}
	return c.byData[string(data)], nil, nil
}
func (c *reconcileCodec) Encode([]domain.Event, []domain.CalendarPassthrough) ([]byte, error) {
	return nil, nil
}

func seqID() func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("copy-%d", n)
	}
}

func rec(calendarID, collectionHref string) RemoteCalendarRecord {
	return RemoteCalendarRecord{CalendarID: calendarID, Href: collectionHref, Name: "Cal"}
}

func savedHrefs(store *fakeSyncStore) map[string]bool {
	out := map[string]bool{}
	for _, s := range store.saved {
		out[s.href] = true
	}
	return out
}

func clearedHrefs(store *fakeSyncStore) map[string]bool {
	out := map[string]bool{}
	for _, c := range store.clearedOps {
		out[c[1]] = true
	}
	return out
}

func TestReconcileListPendingError(t *testing.T) {
	svc := NewCalDAVReconcileService(&fakeSyncStore{pendingErr: errBoom}, &reconcileCodec{}, seqID())
	err := svc.Reconcile(context.Background(), &fakeReconcileSource{}, []RemoteCalendarRecord{rec("cal1", "/c1/")})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

func TestReconcileSkipsUnreadableCollection(t *testing.T) {
	store := &fakeSyncStore{synced: map[string][]SyncedObject{"cal1": {{Href: "/c1/a.ics", ETag: "e"}}}}
	src := &fakeReconcileSource{listErr: map[string]error{"/c1/": errBoom}}
	svc := NewCalDAVReconcileService(store, &reconcileCodec{}, seqID())
	if err := svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(store.deletedHrefs) != 0 || len(store.saved) != 0 {
		t.Errorf("an unreadable collection must not change local state")
	}
}

func TestReconcileSkipsWhenLocalUnreadable(t *testing.T) {
	store := &fakeSyncStore{syncedErr: errBoom}
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/a.ics", ETag: "e", Data: []byte("A")}}}}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"A": {writeEvent(t, "a")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")})
	if len(store.saved) != 0 {
		t.Errorf("a local-read error must skip the calendar")
	}
}

func TestReconcileServerWinsWithoutPending(t *testing.T) {
	store := &fakeSyncStore{synced: map[string][]SyncedObject{"cal1": {
		{Href: "/c1/changed.ics", ETag: "old"},
		{Href: "/c1/same.ics", ETag: "keep"},
	}}}
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {
		{Href: "/c1/new.ics", ETag: "n", Data: []byte("N")},
		{Href: "/c1/changed.ics", ETag: "new", Data: []byte("C")},
		{Href: "/c1/same.ics", ETag: "keep", Data: []byte("S")},
	}}}
	codec := &reconcileCodec{byData: map[string][]domain.Event{
		"N": {writeEvent(t, "n")}, "C": {writeEvent(t, "c")}, "S": {writeEvent(t, "s")},
	}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")})
	saved := savedHrefs(store)
	if !saved["/c1/new.ics"] || !saved["/c1/changed.ics"] || saved["/c1/same.ics"] {
		t.Errorf("saved = %+v; want the new and changed objects, not the unchanged one", store.saved)
	}
}

func TestReconcilePendingGuardsUntilServerAgrees(t *testing.T) {
	const href = "/c1/g.ics"
	store := &fakeSyncStore{
		synced:  map[string][]SyncedObject{"cal1": {{Href: href, ETag: "base"}}},
		pending: []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "base"}},
	}
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {{Href: href, ETag: "base", Data: []byte("B")}}}}
	svc := NewCalDAVReconcileService(store, &reconcileCodec{}, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")})
	if len(store.saved) != 0 || len(store.clearedOps) != 0 {
		t.Errorf("a guarded object must be left untouched: saved=%+v cleared=%+v", store.saved, store.clearedOps)
	}
}

func TestReconcileConflictKeepsSafetyCopy(t *testing.T) {
	const href = "/c1/x.ics"
	store := &fakeSyncStore{
		synced:       map[string][]SyncedObject{"cal1": {{Href: href, ETag: "base"}}},
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "base"}},
		eventsByHref: map[string][]domain.Event{href: {writeEvent(t, "local")}},
	}
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {{Href: href, ETag: "server-changed", Data: []byte("SV")}}}}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"SV": {writeEvent(t, "srv")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")})

	var safety, applied bool
	for _, s := range store.saved {
		if s.href == "" && s.id == "copy-1" && s.etag == "" {
			safety = true
		}
		if s.href == href && s.etag == "server-changed" {
			applied = true
		}
	}
	if !safety {
		t.Errorf("the losing local version must be kept as a safety copy: %+v", store.saved)
	}
	if !applied {
		t.Errorf("the server version must overwrite local: %+v", store.saved)
	}
	if !clearedHrefs(store)[href] {
		t.Errorf("the pending intent must clear after the conflict resolves: %+v", store.clearedOps)
	}
}

func TestReconcileConflictWithNoLocalEventsSkipsSafetyCopy(t *testing.T) {
	const href = "/c1/x.ics"
	store := &fakeSyncStore{
		synced:       map[string][]SyncedObject{"cal1": {{Href: href, ETag: "base"}}},
		pending:      []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "base"}},
		eventsByHref: map[string][]domain.Event{}, // nothing local to copy
	}
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {{Href: href, ETag: "srv", Data: []byte("SV")}}}}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"SV": {writeEvent(t, "srv")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")})
	for _, s := range store.saved {
		if s.href == "" {
			t.Errorf("no safety copy expected when there are no local events: %+v", store.saved)
		}
	}
	if len(store.clearedOps) != 1 {
		t.Errorf("the pending intent must still clear: %+v", store.clearedOps)
	}
}

func TestReconcileDecodeErrorSavesNothing(t *testing.T) {
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/d.ics", ETag: "e", Data: []byte("D")}}}}
	store := &fakeSyncStore{}
	codec := &reconcileCodec{decodeErr: map[string]error{"D": errBoom}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")})
	if len(store.saved) != 0 {
		t.Errorf("an undecodable object must save nothing")
	}
}

func TestReconcileSkipsUnchangedCollection(t *testing.T) {
	// The server reports the same CTag the last sync recorded, so the collection's objects are neither fetched
	// nor merged, and its stored CTag is not rewritten. The source carries a changed object to prove that a
	// skipped collection is genuinely not listed (else it would be saved).
	store := &fakeSyncStore{synced: map[string][]SyncedObject{"cal1": {{Href: "/c1/a.ics", ETag: "e"}}}}
	src := &fakeReconcileSource{
		ctag:    map[string]string{"/c1/": "ctag-1"},
		objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/a.ics", ETag: "changed", Data: []byte("A")}}},
	}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"A": {writeEvent(t, "a")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "ctag-1")})
	if len(store.saved) != 0 {
		t.Errorf("an unchanged collection must not be merged: %+v", store.saved)
	}
	if len(store.updatedCTags) != 0 {
		t.Errorf("an unchanged collection must not rewrite its ctag: %+v", store.updatedCTags)
	}
}

func TestReconcileUpdatesCTagAfterMerge(t *testing.T) {
	// The server CTag differs from the recorded one, so the collection is reconciled and its new CTag stored.
	store := &fakeSyncStore{}
	src := &fakeReconcileSource{
		ctag:    map[string]string{"/c1/": "ctag-2"},
		objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/a.ics", ETag: "e", Data: []byte("A")}}},
	}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"A": {writeEvent(t, "a")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "ctag-1")})
	if !savedHrefs(store)["/c1/a.ics"] {
		t.Errorf("a changed collection must be merged: %+v", store.saved)
	}
	if len(store.updatedCTags) != 1 || store.updatedCTags[0] != [2]string{"cal1", "ctag-2"} {
		t.Errorf("the new server ctag must be recorded: %+v", store.updatedCTags)
	}
}

func TestReconcileCTagErrorStillReconciles(t *testing.T) {
	// A CTag read failure means the collection cannot be proven unchanged, so it is reconciled anyway and its
	// CTag is not rewritten, since there is no reliable value to store.
	store := &fakeSyncStore{}
	src := &fakeReconcileSource{
		ctagErr: map[string]error{"/c1/": errBoom},
		objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/a.ics", ETag: "e", Data: []byte("A")}}},
	}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"A": {writeEvent(t, "a")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "ctag-1")})
	if !savedHrefs(store)["/c1/a.ics"] {
		t.Errorf("a ctag read error must fall back to a full reconcile: %+v", store.saved)
	}
	if len(store.updatedCTags) != 0 {
		t.Errorf("a ctag read error must not record a ctag: %+v", store.updatedCTags)
	}
}

func TestReconcileCTagNotAdvancedWhenLocalUnreadable(t *testing.T) {
	// The server CTag changed, so the collection is fetched, but reading the local objects fails, so the merge
	// never runs and the CTag must not advance: advancing it would wrongly skip the collection next time.
	store := &fakeSyncStore{syncedErr: errBoom}
	src := &fakeReconcileSource{
		ctag:    map[string]string{"/c1/": "ctag-2"},
		objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/a.ics", ETag: "e", Data: []byte("A")}}},
	}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"A": {writeEvent(t, "a")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "ctag-1")})
	if len(store.updatedCTags) != 0 {
		t.Errorf("the ctag must not advance when the merge could not run: %+v", store.updatedCTags)
	}
}

func TestReconcileCTagNotAdvancedWhenApplyFails(t *testing.T) {
	// The server changed, so the collection is fetched, but persisting the server object fails. The merge did
	// not fully land, so the CTag must not advance: advancing it would skip the object forever, whereas
	// withholding it re-reconciles next sync and the object gets another chance to land.
	store := &fakeSyncStore{saveSyncedErr: errBoom}
	src := &fakeReconcileSource{
		ctag:    map[string]string{"/c1/": "ctag-2"},
		objects: map[string][]RemoteObject{"/c1/": {{Href: "/c1/a.ics", ETag: "e", Data: []byte("A")}}},
	}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"A": {writeEvent(t, "a")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "ctag-1")})
	if len(store.updatedCTags) != 0 {
		t.Errorf("a failed object save must withhold the ctag: %+v", store.updatedCTags)
	}
}

func TestReconcileCTagWithheldWhenServerConflictSafetyReadFails(t *testing.T) {
	// A conflict on a server-present object where reading the local rows to copy fails: the merge is incomplete,
	// so the CTag is withheld even though the server version is still applied.
	const href = "/c1/x.ics"
	store := &fakeSyncStore{
		synced:    map[string][]SyncedObject{"cal1": {{Href: href, ETag: "base"}}},
		pending:   []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "base"}},
		eventsErr: errBoom, // reading the local events to copy fails
	}
	src := &fakeReconcileSource{
		ctag:    map[string]string{"/c1/": "ctag-2"},
		objects: map[string][]RemoteObject{"/c1/": {{Href: href, ETag: "server", Data: []byte("SV")}}},
	}
	codec := &reconcileCodec{byData: map[string][]domain.Event{"SV": {writeEvent(t, "srv")}}}
	svc := NewCalDAVReconcileService(store, codec, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "ctag-1")})
	if len(store.updatedCTags) != 0 {
		t.Errorf("a failed safety-copy read must withhold the ctag: %+v", store.updatedCTags)
	}
}

func TestReconcileCTagWithheldWhenMissingObjectSafetyFails(t *testing.T) {
	// The server dropped an object under a pending local update, and reading the local rows to copy fails, so
	// the missing-object merge is incomplete and the CTag is withheld.
	const href = "/c1/gone.ics"
	store := &fakeSyncStore{
		synced:    map[string][]SyncedObject{"cal1": {{Href: href, ETag: "e"}}},
		pending:   []PendingCalendarObject{{CalendarID: "cal1", Href: href, Op: CalendarOpUpdate, BaseETag: "e"}},
		eventsErr: errBoom,
	}
	src := &fakeReconcileSource{
		ctag:    map[string]string{"/c1/": "ctag-2"},
		objects: map[string][]RemoteObject{"/c1/": {}}, // the server has dropped the object
	}
	svc := NewCalDAVReconcileService(store, &reconcileCodec{}, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{recWithCTag("cal1", "/c1/", "ctag-1")})
	if len(store.updatedCTags) != 0 {
		t.Errorf("a failed safety copy on a server-gone object must withhold the ctag: %+v", store.updatedCTags)
	}
}

func TestReconcileMissingObjects(t *testing.T) {
	store := &fakeSyncStore{
		synced: map[string][]SyncedObject{"cal1": {
			{Href: "/c1/gone-nopending.ics", ETag: "e"},
			{Href: "/c1/gone-create.ics", ETag: ""},
			{Href: "/c1/gone-update.ics", ETag: "e"},
			{Href: "/c1/gone-delete.ics", ETag: "e"},
		}},
		pending: []PendingCalendarObject{
			{CalendarID: "cal1", Href: "/c1/gone-create.ics", Op: CalendarOpCreate},
			{CalendarID: "cal1", Href: "/c1/gone-update.ics", Op: CalendarOpUpdate, BaseETag: "e"},
			{CalendarID: "cal1", Href: "/c1/gone-delete.ics", Op: CalendarOpDelete, BaseETag: "e"},
			// A pending op for another calendar must be ignored when reconciling cal1.
			{CalendarID: "cal2", Href: "/c2/z.ics", Op: CalendarOpCreate},
		},
		eventsByHref: map[string][]domain.Event{"/c1/gone-update.ics": {writeEvent(t, "u")}},
	}
	src := &fakeReconcileSource{objects: map[string][]RemoteObject{"/c1/": {}}} // the server has none of them
	svc := NewCalDAVReconcileService(store, &reconcileCodec{}, seqID())
	_ = svc.Reconcile(context.Background(), src, []RemoteCalendarRecord{rec("cal1", "/c1/")})

	deleted := map[string]bool{}
	for _, h := range store.deletedHrefs {
		deleted[h] = true
	}
	if !deleted["/c1/gone-nopending.ics"] {
		t.Errorf("a server-removed object with no pending change should be deleted locally")
	}
	if deleted["/c1/gone-create.ics"] {
		t.Errorf("a pending create should be left for the flush, not deleted")
	}
	if !deleted["/c1/gone-update.ics"] || !deleted["/c1/gone-delete.ics"] {
		t.Errorf("a pending update or delete on a server-gone object should drop the local rows")
	}
	safety := 0
	for _, s := range store.saved {
		if s.href == "" {
			safety++
		}
	}
	if safety != 1 {
		t.Errorf("exactly one safety copy (for the pending update) expected, got %d", safety)
	}
	cleared := clearedHrefs(store)
	if !cleared["/c1/gone-update.ics"] || !cleared["/c1/gone-delete.ics"] || cleared["/c1/gone-create.ics"] {
		t.Errorf("cleared = %+v; want the update and delete cleared, the create left", store.clearedOps)
	}
}
