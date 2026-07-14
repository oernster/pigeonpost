package application

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestAccountServiceReorder(t *testing.T) {
	svc, store, _, _ := newAccountService()

	if err := svc.Reorder(context.Background(), []string{"b", "a", "c"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.reordered) != 3 || store.reordered[0] != "b" || store.reordered[2] != "c" {
		t.Fatalf("reordered = %v, want [b a c]", store.reordered)
	}

	store.reorderErr = errBoom
	if err := svc.Reorder(context.Background(), []string{"a"}); !errors.Is(err, errBoom) {
		t.Errorf("Reorder error = %v, want wrapped boom", err)
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

func TestAccountServiceGet(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")

	got, err := svc.Get(context.Background(), "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID() != "a1" {
		t.Fatalf("Get returned %q, want a1", got.ID())
	}

	store.getErr = errBoom
	if _, err := svc.Get(context.Background(), "a1"); !errors.Is(err, errBoom) {
		t.Errorf("Get error = %v, want wrapped boom", err)
	}
}

func TestAccountServiceUpdateProfileSuccess(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")
	alias, err := domain.NewEmailAddress("Alias", "alias@example.com")
	if err != nil {
		t.Fatalf("build alias: %v", err)
	}

	err = svc.UpdateProfile(
		context.Background(), "a1", "New Name", "<p>Bye</p>", []domain.EmailAddress{alias},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	saved, ok := store.accounts["a1"]
	if !ok {
		t.Fatal("account should still exist")
	}
	if saved.DisplayName() != "New Name" {
		t.Errorf("display name = %q, want New Name", saved.DisplayName())
	}
	if saved.Signature() != "<p>Bye</p>" {
		t.Errorf("signature = %q, want <p>Bye</p>", saved.Signature())
	}
	ids := saved.Identities()
	if len(ids) != 1 || ids[0].Address() != "alias@example.com" {
		t.Errorf("identities = %+v, want one alias@example.com", ids)
	}
}

func TestAccountServiceUpdateProfileGetError(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.getErr = errBoom
	if err := svc.UpdateProfile(context.Background(), "a1", "New Name", "", nil); !errors.Is(err, errBoom) {
		t.Errorf("UpdateProfile error = %v, want wrapped boom", err)
	}
}

func TestAccountServiceUpdateProfileEmptyName(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")
	err := svc.UpdateProfile(context.Background(), "a1", "   ", "", nil)
	if !errors.Is(err, domain.ErrEmptyDisplayName) {
		t.Errorf("UpdateProfile error = %v, want ErrEmptyDisplayName", err)
	}
}

func TestAccountServiceUpdateProfileSaveError(t *testing.T) {
	svc, store, _, _ := newAccountService()
	store.accounts["a1"] = testAccount(t, "a1")
	store.saveErr = errBoom
	if err := svc.UpdateProfile(context.Background(), "a1", "New Name", "", nil); !errors.Is(err, errBoom) {
		t.Errorf("UpdateProfile error = %v, want wrapped boom", err)
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
	svc := NewMailboxService(store, time.UTC)

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
	svc := NewMailboxService(store, time.UTC)

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

func testMessageAt(t *testing.T, id, folderID string, dateMs int64) domain.MessageSummary {
	t.Helper()
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: "1", Size: 10, Flags: domain.NewFlags(0),
		Date: time.UnixMilli(dateMs),
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	return msg
}

func TestMailboxServiceMessagesPage(t *testing.T) {
	store := newFakeMailStore()
	store.messages["f1"] = []domain.MessageSummary{
		testMessageAt(t, "m1", "f1", 100),
		testMessageAt(t, "m2", "f1", 300),
		testMessageAt(t, "m3", "f1", 200),
	}
	svc := NewMailboxService(store, time.UTC)
	ctx := context.Background()

	// First page, newest first, limit two: m2 (300) then m3 (200).
	first, err := svc.MessagesPage(ctx, "f1", false, 0, "", 2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(first) != 2 || first[0].ID() != "m2" || first[1].ID() != "m3" {
		t.Fatalf("first page = %q, %q, want m2, m3", first[0].ID(), first[1].ID())
	}

	// Second page resumes strictly after the first page's last row (m3), yielding m1 (100).
	last := first[len(first)-1]
	second, err := svc.MessagesPage(ctx, "f1", true, last.Date().UnixMilli(), last.ID(), 2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(second) != 1 || second[0].ID() != "m1" {
		t.Fatalf("second page = %d rows, want [m1]", len(second))
	}

	// Ascending first page walks oldest first: m1 (100) then m3 (200).
	asc, err := svc.MessagesPage(ctx, "f1", false, 0, "", 2, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(asc) != 2 || asc[0].ID() != "m1" || asc[1].ID() != "m3" {
		t.Fatalf("ascending page = %q, %q, want m1, m3", asc[0].ID(), asc[1].ID())
	}

	store.listPageErr = errBoom
	if _, err := svc.MessagesPage(ctx, "f1", false, 0, "", 2, false); !errors.Is(err, errBoom) {
		t.Errorf("MessagesPage error = %v, want wrapped boom", err)
	}
}

func TestMailboxServiceThreads(t *testing.T) {
	store := newFakeMailStore()
	store.messages["f1"] = []domain.MessageSummary{
		testMessage(t, "m1", "f1"),
		testMessage(t, "m2", "f1"),
	}
	svc := NewMailboxService(store, time.UTC)

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
	store.searchResults = []SearchHit{{Summary: testMessage(t, "m1", "f1"), Snippet: "a \x01hello\x02 b"}}
	svc := NewMailboxService(store, time.UTC)

	hits, degraded, err := svc.Search(context.Background(), "hello", "f9", "a9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 || hits[0].Summary.ID() != "m1" || hits[0].Snippet == "" {
		t.Fatalf("Search returned %+v", hits)
	}
	if degraded {
		t.Error("a plain word must not report degraded parsing")
	}
	// The UI scope and the result cap are threaded through to the store's modelled query.
	if store.searchQuery.ScopeFolderID() != "f9" || store.searchQuery.ScopeAccountID() != "a9" {
		t.Errorf("scope not threaded: %q %q", store.searchQuery.ScopeFolderID(), store.searchQuery.ScopeAccountID())
	}
	if store.searchLimit != searchResultLimit {
		t.Errorf("limit = %d, want %d", store.searchLimit, searchResultLimit)
	}
}

func TestMailboxServiceSearchEmptyQuerySkipsStore(t *testing.T) {
	store := newFakeMailStore()
	store.searchErr = errBoom // must never be reached
	svc := NewMailboxService(store, time.UTC)
	hits, degraded, err := svc.Search(context.Background(), "   ", "", "")
	if err != nil || len(hits) != 0 || degraded {
		t.Fatalf("blank query: hits=%v degraded=%v err=%v, want none/false/nil", hits, degraded, err)
	}
}

func TestMailboxServiceSearchDegraded(t *testing.T) {
	store := newFakeMailStore()
	svc := NewMailboxService(store, time.UTC)
	if _, degraded, err := svc.Search(context.Background(), `broken "quote`, "", ""); err != nil || !degraded {
		t.Fatalf("degraded=%v err=%v, want true/nil", degraded, err)
	}
}

func TestMailboxServiceSearchError(t *testing.T) {
	store := newFakeMailStore()
	store.searchErr = errBoom
	svc := NewMailboxService(store, time.UTC)
	if _, _, err := svc.Search(context.Background(), "hello", "", ""); !errors.Is(err, errBoom) {
		t.Errorf("Search error = %v, want wrapped boom", err)
	}
}

func TestMailboxServiceUnreadCounts(t *testing.T) {
	store := newFakeMailStore()
	store.unreadByAccount = map[string]int{"a1": 2, "a2": 1}
	svc := NewMailboxService(store, time.UTC)

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
