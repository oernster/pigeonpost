package application

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

type fakeCalendarAccountStore struct {
	accounts map[string]domain.CalendarAccount
	saveErr  error
	getErr   error
	delErr   error
	listErr  error
}

func (f *fakeCalendarAccountStore) SaveCalendarAccount(_ context.Context, a domain.CalendarAccount) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.accounts == nil {
		f.accounts = map[string]domain.CalendarAccount{}
	}
	f.accounts[a.ID()] = a
	return nil
}

func (f *fakeCalendarAccountStore) ListCalendarAccounts(context.Context) ([]domain.CalendarAccount, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]domain.CalendarAccount, 0, len(f.accounts))
	for _, a := range f.accounts {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeCalendarAccountStore) GetCalendarAccount(_ context.Context, id string) (domain.CalendarAccount, error) {
	if f.getErr != nil {
		return domain.CalendarAccount{}, f.getErr
	}
	a, ok := f.accounts[id]
	if !ok {
		return domain.CalendarAccount{}, ErrCalendarAccountNotFound
	}
	return a, nil
}

func (f *fakeCalendarAccountStore) DeleteCalendarAccount(_ context.Context, id string) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.accounts, id)
	return nil
}

type fakeCalendarCredentialStore struct {
	passwords map[string]string
	getErr    error
	setErr    error
	delErr    error
}

func (f *fakeCalendarCredentialStore) CalendarPassword(_ context.Context, a domain.CalendarAccount) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.passwords[a.ID()], nil
}

func (f *fakeCalendarCredentialStore) SetCalendarPassword(_ context.Context, a domain.CalendarAccount, secret string) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.passwords == nil {
		f.passwords = map[string]string{}
	}
	f.passwords[a.ID()] = secret
	return nil
}

func (f *fakeCalendarCredentialStore) DeleteCalendarPassword(_ context.Context, a domain.CalendarAccount) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.passwords, a.ID())
	return nil
}

type fakeCalDAVSourceFactory struct {
	source CalDAVSource
	err    error
}

func (f *fakeCalDAVSourceFactory) NewSource(domain.CalendarAccount, string) (CalDAVSource, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.source, nil
}

type fakeCalDAVWriterFactory struct {
	writer CalDAVWriter
	err    error
}

func (f *fakeCalDAVWriterFactory) NewWriter(domain.CalendarAccount, string) (CalDAVWriter, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.writer, nil
}

// newTestCalDAVService builds a CalDAVService with a working writer factory and a fixed id generator, the
// arguments the account-management and pull tests do not vary.
func newTestCalDAVService(accts CalendarAccountStore, creds CalendarCredentialStore, factory CalDAVSourceFactory, codec CalendarCodec, store CalendarSyncStore) *CalDAVService {
	return NewCalDAVService(accts, creds, factory, &fakeCalDAVWriterFactory{writer: &fakeWriter{}}, codec, store, fixedID("x"))
}

func davAccount(t *testing.T, id string) domain.CalendarAccount {
	t.Helper()
	a, err := domain.NewCalendarAccount(id, "Fastmail", "https://d.example.com", "user", domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return a
}

func TestCalDAVAddAccount(t *testing.T) {
	accts := &fakeCalendarAccountStore{}
	creds := &fakeCalendarCredentialStore{}
	svc := newTestCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.AddAccount(context.Background(), davAccount(t, "c1"), "secret"); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if creds.passwords["c1"] != "secret" {
		t.Errorf("password not stored")
	}
	if _, ok := accts.accounts["c1"]; !ok {
		t.Errorf("account not stored")
	}
}

func TestCalDAVAddAccountPasswordError(t *testing.T) {
	svc := newTestCalDAVService(&fakeCalendarAccountStore{}, &fakeCalendarCredentialStore{setErr: errBoom},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.AddAccount(context.Background(), davAccount(t, "c1"), "secret"); err == nil {
		t.Fatal("expected an error when storing the password fails")
	}
}

func TestCalDAVAddAccountSaveError(t *testing.T) {
	svc := newTestCalDAVService(&fakeCalendarAccountStore{saveErr: errBoom}, &fakeCalendarCredentialStore{},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.AddAccount(context.Background(), davAccount(t, "c1"), "secret"); err == nil {
		t.Fatal("expected an error when saving the account fails")
	}
}

func TestCalDAVListAccounts(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	svc := newTestCalDAVService(accts, &fakeCalendarCredentialStore{}, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	list, err := svc.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(list) != 1 || list[0].ID() != "c1" {
		t.Errorf("accounts = %+v", list)
	}
}

func TestCalDAVRemoveAccount(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{passwords: map[string]string{"c1": "secret"}}
	svc := newTestCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.RemoveAccount(context.Background(), "c1"); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	if _, ok := accts.accounts["c1"]; ok {
		t.Errorf("account not removed")
	}
	if _, ok := creds.passwords["c1"]; ok {
		t.Errorf("password not removed")
	}
}

func TestCalDAVRemoveAccountGetError(t *testing.T) {
	svc := newTestCalDAVService(&fakeCalendarAccountStore{}, &fakeCalendarCredentialStore{},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.RemoveAccount(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error removing an unknown account")
	}
}

func TestCalDAVRemoveAccountDeleteError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}, delErr: errBoom}
	svc := newTestCalDAVService(accts, &fakeCalendarCredentialStore{}, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.RemoveAccount(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when deleting the account fails")
	}
}

func TestCalDAVRemoveAccountPasswordDeleteError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{delErr: errBoom}
	svc := newTestCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.RemoveAccount(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when deleting the password fails")
	}
}

func TestCalDAVPull(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a"}},
		objects:   map[string][]RemoteObject{"/a": {{Data: []byte("EV")}}},
	}
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{passwords: map[string]string{"c1": "secret"}}
	codec := &davCodec{decode: map[string][]domain.Event{"EV": {davEvent(t, "e1")}}}
	store := &fakeSyncStore{}
	svc := newTestCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, codec, store)
	n, err := svc.Pull(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 1 {
		t.Errorf("saved = %d, want 1", n)
	}
}

func TestCalDAVPullAccountError(t *testing.T) {
	svc := newTestCalDAVService(&fakeCalendarAccountStore{}, &fakeCalendarCredentialStore{},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if _, err := svc.Pull(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error pulling an unknown account")
	}
}

func TestCalDAVPullPasswordError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{getErr: errBoom}
	svc := newTestCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if _, err := svc.Pull(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when the password cannot be read")
	}
}

func TestCalDAVPullSourceError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{passwords: map[string]string{"c1": "secret"}}
	svc := newTestCalDAVService(accts, creds, &fakeCalDAVSourceFactory{err: errBoom}, &davCodec{}, &fakeSyncStore{})
	if _, err := svc.Pull(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when the source cannot be built")
	}
}

// syncAccounts is the account and credential pair a Sync happy-path or error test starts from.
func syncAccounts(t *testing.T) (*fakeCalendarAccountStore, *fakeCalendarCredentialStore) {
	t.Helper()
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{passwords: map[string]string{"c1": "secret"}}
	return accts, creds
}

func TestCalDAVSyncAccountError(t *testing.T) {
	svc := NewCalDAVService(&fakeCalendarAccountStore{}, &fakeCalendarCredentialStore{},
		&fakeCalDAVSourceFactory{}, &fakeCalDAVWriterFactory{}, &davCodec{}, &fakeSyncStore{}, fixedID("x"))
	if err := svc.Sync(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error syncing an unknown account")
	}
}

func TestCalDAVSyncPasswordError(t *testing.T) {
	accts, _ := syncAccounts(t)
	creds := &fakeCalendarCredentialStore{getErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &fakeCalDAVWriterFactory{}, &davCodec{}, &fakeSyncStore{}, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when the password cannot be read")
	}
}

func TestCalDAVSyncSourceError(t *testing.T) {
	accts, creds := syncAccounts(t)
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{err: errBoom}, &fakeCalDAVWriterFactory{}, &davCodec{}, &fakeSyncStore{}, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when the source cannot be built")
	}
}

func TestCalDAVSyncWriterError(t *testing.T) {
	accts, creds := syncAccounts(t)
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: &fakeCalDAVSource{}}, &fakeCalDAVWriterFactory{err: errBoom}, &davCodec{}, &fakeSyncStore{}, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when the writer cannot be built")
	}
}

func TestCalDAVSyncDiscoverError(t *testing.T) {
	accts, creds := syncAccounts(t)
	src := &fakeCalDAVSource{listCalErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: &fakeWriter{}}, &davCodec{}, &fakeSyncStore{}, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); err == nil {
		t.Fatal("expected a fatal error when discovering the collections fails")
	}
}

func TestCalDAVSync(t *testing.T) {
	accts, creds := syncAccounts(t)
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}},
		objects:   map[string][]RemoteObject{"/a": {{Href: "/a/o1.ics", ETag: "e1", Data: []byte("EV")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV": {davEvent(t, "e1")}}}
	store := &fakeSyncStore{}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: &fakeWriter{}}, codec, store, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	// Discovery mirrored the collection as a local calendar under the account.
	if len(store.savedCals) != 1 || store.savedCals[0].id != "c1|/a" {
		t.Errorf("discovered calendars = %+v", store.savedCals)
	}
	// Reconcile applied the server object into the local calendar tagged with its href and etag.
	if len(store.saved) != 1 || store.saved[0].href != "/a/o1.ics" || store.saved[0].etag != "e1" {
		t.Errorf("reconciled events = %+v", store.saved)
	}
}

func TestCalDAVSyncSwallowsBestEffortErrors(t *testing.T) {
	// A failure listing pending ops breaks both the flush and the reconcile, but discovery still succeeds, so
	// the sync completes rather than failing and the pending intents are left for the next run.
	accts, creds := syncAccounts(t)
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}}}
	store := &fakeSyncStore{pendingErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: &fakeWriter{}}, &davCodec{}, store, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); err != nil {
		t.Fatalf("best-effort flush and reconcile must not fail the sync: %v", err)
	}
	if len(store.savedCals) != 1 {
		t.Errorf("discovery did not run despite the pending-list failure: %+v", store.savedCals)
	}
}

func TestCalDAVSyncGuardsLocalEditThroughReconcile(t *testing.T) {
	// The reason Sync routes object I/O through the reconcile rather than a naive object-saving pull is that an
	// unpushed local edit must be guarded: a server change under it resolves last-writer-wins yet keeps the
	// local version as a safety copy. Driving that through Sync pins the contract here, so a regression to a
	// clobbering pull is caught at the Sync boundary and not only in the reconcile's own unit test.
	accts, creds := syncAccounts(t)
	const href = "/a/o1.ics"
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}},
		objects:   map[string][]RemoteObject{"/a": {{Href: href, ETag: "server-new", Data: []byte("EV")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV": {davEvent(t, "e1")}}}
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "c1|/a", Href: href, Op: CalendarOpUpdate, BaseETag: "base"}},
		eventsByHref: map[string][]domain.Event{href: {davEvent(t, "local1")}},
	}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: &fakeWriter{}}, codec, store, fixedID("safety"))
	if err := svc.Sync(context.Background(), "c1"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	var hasSafetyCopy, hasServerApplied bool
	for _, s := range store.saved {
		if s.id == "safety" && s.href == "" && s.etag == "" {
			hasSafetyCopy = true
		}
		if s.href == href && s.etag == "server-new" {
			hasServerApplied = true
		}
	}
	if !hasSafetyCopy {
		t.Errorf("the conflicting local edit was not preserved as a safety copy: %+v", store.saved)
	}
	if !hasServerApplied {
		t.Errorf("the server version was not applied through the reconcile: %+v", store.saved)
	}
}

func TestCalDAVSyncFlushesPendingChangeThroughWriter(t *testing.T) {
	// Sync must push the account's pending local changes to the server through the writer before it reconciles.
	// Asserting the writer was actually called with the right precondition proves the flush half runs at the
	// Sync boundary, so dropping it is caught here rather than passing on an empty store.
	accts, creds := syncAccounts(t)
	const href = "/a/new.ics"
	src := &fakeCalDAVSource{calendars: []RemoteCalendar{{Path: "/a", DisplayName: "A"}}}
	store := &fakeSyncStore{
		pending:      []PendingCalendarObject{{CalendarID: "c1|/a", Href: href, Op: CalendarOpCreate}},
		eventsByHref: map[string][]domain.Event{href: {davEvent(t, "e1")}},
	}
	writer := &fakeWriter{putETag: "srv-created"}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, &fakeCalDAVWriterFactory{writer: writer}, &davCodec{}, store, fixedID("x"))
	if err := svc.Sync(context.Background(), "c1"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if writer.putCalls != 1 || writer.putHref != href {
		t.Errorf("the pending create was not pushed through the writer: putCalls=%d href=%q", writer.putCalls, writer.putHref)
	}
	if writer.putIfNoneMatch != "*" {
		t.Errorf("a create must guard the push with If-None-Match:* , got %q", writer.putIfNoneMatch)
	}
}
