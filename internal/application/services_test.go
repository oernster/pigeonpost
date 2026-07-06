package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

var errBoom = errors.New("boom")

func newAccountService() (*AccountService, *fakeAccountStore, *fakeCredentialStore, *fakeMailStore) {
	store := newFakeAccountStore()
	creds := newFakeCredentialStore()
	mail := newFakeMailStore()
	return NewAccountService(store, creds, mail), store, creds, mail
}

func TestAccountServiceList(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")

	accounts, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID() != "a1" {
		t.Fatalf("List returned %+v", accounts)
	}

	store.listErr = errBoom
	if _, err := svc.List(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("List error = %v, want wrapped boom", err)
	}
}

func TestAccountServiceAdd(t *testing.T) {
	svc, store, _, _ := newAccountService()

	if err := svc.Add(context.Background(), testAccount(t, "a1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected one saved account, got %d", len(store.saved))
	}

	store.saveErr = errBoom
	if err := svc.Add(context.Background(), testAccount(t, "a2")); !errors.Is(err, errBoom) {
		t.Errorf("Add error = %v, want wrapped boom", err)
	}
}

func TestAccountServiceRemoveSuccess(t *testing.T) {
	svc, store, creds, mail := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")
	creds.passwords["a1"] = "s3cret"

	if err := svc.Remove(context.Background(), "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.accounts["a1"]; ok {
		t.Error("account row should be gone")
	}
	if len(mail.deletedData) != 1 || mail.deletedData[0] != "a1" {
		t.Errorf("expected cached mail deleted for a1, got %v", mail.deletedData)
	}
	if _, ok := creds.passwords["a1"]; ok {
		t.Error("credential should be gone")
	}
}

func TestAccountServiceRemoveGetError(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.getErr = errBoom
	if err := svc.Remove(context.Background(), "a1"); !errors.Is(err, errBoom) {
		t.Errorf("Remove error = %v, want wrapped boom", err)
	}
}

func TestAccountServiceRemoveDeleteAccountError(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")
	store.deleteErr = errBoom
	if err := svc.Remove(context.Background(), "a1"); !errors.Is(err, errBoom) {
		t.Errorf("Remove error = %v, want wrapped boom", err)
	}
}

func TestAccountServiceRemoveMailError(t *testing.T) {
	svc, store, _, mail := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")
	mail.deleteDataErr = errBoom
	if err := svc.Remove(context.Background(), "a1"); !errors.Is(err, errBoom) {
		t.Errorf("Remove error = %v, want wrapped boom", err)
	}
}

func TestAccountServiceRemoveCredentialError(t *testing.T) {
	svc, store, creds, _ := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")
	creds.deleteErr = errBoom
	if err := svc.Remove(context.Background(), "a1"); !errors.Is(err, errBoom) {
		t.Errorf("Remove error = %v, want wrapped boom", err)
	}
}

func newSetupService() (*AccountSetupService, *fakeAccountStore, *fakeCredentialStore, *fakeVerifier) {
	store := newFakeAccountStore()
	creds := newFakeCredentialStore()
	verifier := newFakeVerifier()
	return NewAccountSetupService(store, creds, verifier), store, creds, verifier
}

func TestAccountSetupConfigureSuccess(t *testing.T) {
	svc, store, creds, verifier := newSetupService()

	if err := svc.Configure(context.Background(), testAccount(t, "a1"), "s3cret"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verifier.verified["a1"] != "s3cret" {
		t.Errorf("verify saw password %q, want s3cret", verifier.verified["a1"])
	}
	if creds.passwords["a1"] != "s3cret" {
		t.Errorf("password not stored, got %q", creds.passwords["a1"])
	}
	if len(store.saved) != 1 {
		t.Errorf("expected account saved, got %d", len(store.saved))
	}
	if len(creds.deleted) != 0 {
		t.Errorf("expected no rollback on success, got %v", creds.deleted)
	}
}

func TestAccountSetupConfigureVerifyError(t *testing.T) {
	svc, store, creds, verifier := newSetupService()
	verifier.verifyErr = errBoom

	if err := svc.Configure(context.Background(), testAccount(t, "a1"), "x"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	// Verify runs first: nothing is written to the keychain or the store.
	if len(store.saved) != 0 || len(creds.passwords) != 0 || len(creds.deleted) != 0 {
		t.Errorf("failed verify must not touch keychain or store: saved=%v pw=%v del=%v",
			store.saved, creds.passwords, creds.deleted)
	}
}

func TestAccountSetupConfigureCredentialError(t *testing.T) {
	svc, store, creds, _ := newSetupService()
	creds.setErr = errBoom

	if err := svc.Configure(context.Background(), testAccount(t, "a1"), "x"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("account should not be saved when the credential store fails")
	}
}

func TestAccountSetupConfigureSaveErrorRollsBack(t *testing.T) {
	svc, store, creds, _ := newSetupService()
	store.saveErr = errBoom

	if err := svc.Configure(context.Background(), testAccount(t, "a1"), "x"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	if len(creds.deleted) != 1 || creds.deleted[0] != "a1" {
		t.Errorf("expected credential rollback for a1, got %v", creds.deleted)
	}
}

func TestAccountSetupUpdateNewPassword(t *testing.T) {
	svc, store, creds, verifier := newSetupService()
	creds.passwords["a1"] = "old"

	if err := svc.Update(context.Background(), testAccount(t, "a1"), "new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verifier.verified["a1"] != "new" {
		t.Errorf("verify saw %q, want new", verifier.verified["a1"])
	}
	if creds.passwords["a1"] != "new" {
		t.Errorf("password = %q, want new", creds.passwords["a1"])
	}
	if len(store.saved) != 1 {
		t.Errorf("expected account saved, got %d", len(store.saved))
	}
}

func TestAccountSetupUpdateKeepPassword(t *testing.T) {
	svc, store, creds, verifier := newSetupService()
	creds.passwords["a1"] = "kept"

	if err := svc.Update(context.Background(), testAccount(t, "a1"), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verifier.verified["a1"] != "kept" {
		t.Errorf("verify saw %q, want the kept password", verifier.verified["a1"])
	}
	if creds.passwords["a1"] != "kept" {
		t.Errorf("password should be unchanged, got %q", creds.passwords["a1"])
	}
	if len(store.saved) != 1 {
		t.Errorf("expected account saved, got %d", len(store.saved))
	}
}

func TestAccountSetupUpdateReadCredentialError(t *testing.T) {
	svc, _, creds, _ := newSetupService()
	creds.getErr = errBoom

	if err := svc.Update(context.Background(), testAccount(t, "a1"), ""); !errors.Is(err, errBoom) {
		t.Errorf("Update error = %v, want wrapped boom", err)
	}
}

func TestAccountSetupUpdateVerifyError(t *testing.T) {
	svc, store, _, verifier := newSetupService()
	verifier.verifyErr = errBoom

	if err := svc.Update(context.Background(), testAccount(t, "a1"), "new"); !errors.Is(err, errBoom) {
		t.Errorf("Update error = %v, want wrapped boom", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("account should not be saved when verification fails")
	}
}

func TestAccountSetupUpdateSetPasswordError(t *testing.T) {
	svc, _, creds, _ := newSetupService()
	creds.setErr = errBoom

	if err := svc.Update(context.Background(), testAccount(t, "a1"), "new"); !errors.Is(err, errBoom) {
		t.Errorf("Update error = %v, want wrapped boom", err)
	}
}

func TestAccountSetupUpdateSaveError(t *testing.T) {
	svc, store, creds, _ := newSetupService()
	store.saveErr = errBoom
	creds.passwords["a1"] = "kept"

	if err := svc.Update(context.Background(), testAccount(t, "a1"), ""); !errors.Is(err, errBoom) {
		t.Errorf("Update error = %v, want wrapped boom", err)
	}
}

func TestMailboxServiceFolders(t *testing.T) {
	store := newFakeMailStore()
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	svc := NewMailboxService(store)

	folders, err := svc.Folders(context.Background(), "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(folders) != 1 {
		t.Fatalf("expected one folder, got %d", len(folders))
	}

	store.listFoldersErr = errBoom
	if _, err := svc.Folders(context.Background(), "a1"); !errors.Is(err, errBoom) {
		t.Errorf("Folders error = %v, want wrapped boom", err)
	}
}

func TestMailboxServiceMessages(t *testing.T) {
	store := newFakeMailStore()
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	svc := NewMailboxService(store)

	messages, err := svc.Messages(context.Background(), "f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}

	store.listMessagesErr = errBoom
	if _, err := svc.Messages(context.Background(), "f1"); !errors.Is(err, errBoom) {
		t.Errorf("Messages error = %v, want wrapped boom", err)
	}
}

func TestMailboxServiceThreads(t *testing.T) {
	store := newFakeMailStore()
	store.messages["f1"] = []domain.MessageSummary{
		testMessage(t, "m1", "f1"),
		testMessage(t, "m2", "f1"),
	}
	svc := NewMailboxService(store)

	threads, err := svc.Threads(context.Background(), "f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// testMessage builds both summaries with the same (empty) subject, so they thread together.
	if len(threads) != 1 || threads[0].Count() != 2 {
		t.Fatalf("expected one thread of two messages, got %d threads", len(threads))
	}

	store.listMessagesErr = errBoom
	if _, err := svc.Threads(context.Background(), "f1"); !errors.Is(err, errBoom) {
		t.Errorf("Threads error = %v, want wrapped boom", err)
	}
}

func TestMailboxServiceSearch(t *testing.T) {
	store := newFakeMailStore()
	store.searchResults = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	svc := NewMailboxService(store)

	results, err := svc.Search(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].ID() != "m1" {
		t.Fatalf("Search returned %+v", results)
	}

	store.searchErr = errBoom
	if _, err := svc.Search(context.Background(), "hello"); !errors.Is(err, errBoom) {
		t.Errorf("Search error = %v, want wrapped boom", err)
	}
}

func TestMailboxServiceUnreadCounts(t *testing.T) {
	store := newFakeMailStore()
	store.unreadByAccount = map[string]int{"a1": 2, "a2": 1}
	svc := NewMailboxService(store)

	totals, err := svc.UnreadCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if totals.Total != 3 {
		t.Errorf("Total = %d, want 3", totals.Total)
	}
	if totals.ByAccount["a1"] != 2 || totals.ByAccount["a2"] != 1 {
		t.Errorf("ByAccount = %+v, want a1:2 a2:1", totals.ByAccount)
	}

	store.unreadErr = errBoom
	if _, err := svc.UnreadCounts(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("UnreadCounts error = %v, want wrapped boom", err)
	}
}
