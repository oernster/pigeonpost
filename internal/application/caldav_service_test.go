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
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
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
	svc := NewCalDAVService(&fakeCalendarAccountStore{}, &fakeCalendarCredentialStore{setErr: errBoom},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.AddAccount(context.Background(), davAccount(t, "c1"), "secret"); err == nil {
		t.Fatal("expected an error when storing the password fails")
	}
}

func TestCalDAVAddAccountSaveError(t *testing.T) {
	svc := NewCalDAVService(&fakeCalendarAccountStore{saveErr: errBoom}, &fakeCalendarCredentialStore{},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.AddAccount(context.Background(), davAccount(t, "c1"), "secret"); err == nil {
		t.Fatal("expected an error when saving the account fails")
	}
}

func TestCalDAVListAccounts(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	svc := NewCalDAVService(accts, &fakeCalendarCredentialStore{}, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
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
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
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
	svc := NewCalDAVService(&fakeCalendarAccountStore{}, &fakeCalendarCredentialStore{},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.RemoveAccount(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error removing an unknown account")
	}
}

func TestCalDAVRemoveAccountDeleteError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}, delErr: errBoom}
	svc := NewCalDAVService(accts, &fakeCalendarCredentialStore{}, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if err := svc.RemoveAccount(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when deleting the account fails")
	}
}

func TestCalDAVRemoveAccountPasswordDeleteError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{delErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
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
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{source: src}, codec, store)
	n, err := svc.Pull(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 1 {
		t.Errorf("saved = %d, want 1", n)
	}
}

func TestCalDAVPullAccountError(t *testing.T) {
	svc := NewCalDAVService(&fakeCalendarAccountStore{}, &fakeCalendarCredentialStore{},
		&fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if _, err := svc.Pull(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error pulling an unknown account")
	}
}

func TestCalDAVPullPasswordError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{getErr: errBoom}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{}, &davCodec{}, &fakeSyncStore{})
	if _, err := svc.Pull(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when the password cannot be read")
	}
}

func TestCalDAVPullSourceError(t *testing.T) {
	accts := &fakeCalendarAccountStore{accounts: map[string]domain.CalendarAccount{"c1": davAccount(t, "c1")}}
	creds := &fakeCalendarCredentialStore{passwords: map[string]string{"c1": "secret"}}
	svc := NewCalDAVService(accts, creds, &fakeCalDAVSourceFactory{err: errBoom}, &davCodec{}, &fakeSyncStore{})
	if _, err := svc.Pull(context.Background(), "c1"); err == nil {
		t.Fatal("expected an error when the source cannot be built")
	}
}
