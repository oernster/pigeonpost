package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pigeonpost.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func buildAccount(t *testing.T, id string) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	in, err := domain.NewServerConfig("imap.example.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("incoming: %v", err)
	}
	out, err := domain.NewServerConfig("smtp.example.com", 465, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("outgoing: %v", err)
	}
	account, err := domain.NewAccount(id, "Primary", addr, domain.ProtocolIMAP, in, out, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

func TestAccountRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("save account: %v", err)
	}

	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	got := accounts[0]
	if got.ID() != "a1" || got.Address().Address() != "user@example.com" {
		t.Errorf("unexpected account: %+v", got)
	}
	if got.Incoming().Port() != 993 || got.Outgoing().Host() != "smtp.example.com" {
		t.Errorf("server config lost: %+v", got.Incoming())
	}

	fetched, err := store.GetAccount(ctx, "a1")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if fetched.DisplayName() != "Primary" {
		t.Errorf("DisplayName = %q", fetched.DisplayName())
	}

	if _, err := store.GetAccount(ctx, "missing"); !errors.Is(err, application.ErrAccountNotFound) {
		t.Errorf("missing account error = %v, want ErrAccountNotFound", err)
	}
}

func TestSaveAccountReplaces(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("second save: %v", err)
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("expected replace, got %d accounts", len(accounts))
	}
}

func TestDeleteAccountAndData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("save account: %v", err)
	}
	inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 1, 1)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("save folders: %v", err)
	}
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	if err := store.DeleteAccountData(ctx, "a1"); err != nil {
		t.Fatalf("delete account data: %v", err)
	}
	folders, _ := store.ListFolders(ctx, "a1")
	if len(folders) != 0 {
		t.Errorf("expected folders cleared, got %d", len(folders))
	}
	messages, _ := store.ListMessages(ctx, "f1")
	if len(messages) != 0 {
		t.Errorf("expected messages cleared, got %d", len(messages))
	}

	if err := store.DeleteAccount(ctx, "a1"); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	accounts, _ := store.ListAccounts(ctx)
	if len(accounts) != 0 {
		t.Errorf("expected account removed, got %d", len(accounts))
	}
	// Deleting an absent account is not an error.
	if err := store.DeleteAccount(ctx, "a1"); err != nil {
		t.Errorf("delete missing account: %v", err)
	}
}

func TestFolderRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 2, 5)
	archive, _ := domain.NewFolder("f2", "a1", "INBOX/Archive", domain.FolderArchive, 0, 3)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox, archive}); err != nil {
		t.Fatalf("save folders: %v", err)
	}

	folders, err := store.ListFolders(ctx, "a1")
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}
	if folders[0].Path() != "INBOX" || folders[0].Unread() != 2 {
		t.Errorf("unexpected first folder: %+v", folders[0])
	}

	// Saving again replaces rather than accumulates.
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("resave folders: %v", err)
	}
	folders, _ = store.ListFolders(ctx, "a1")
	if len(folders) != 1 {
		t.Errorf("expected replace to 1 folder, got %d", len(folders))
	}
}

func buildMessage(t *testing.T, id string, when time.Time, withSender bool) domain.MessageSummary {
	t.Helper()
	var from domain.EmailAddress
	if withSender {
		var err error
		from, err = domain.NewEmailAddress("Alice", "alice@example.com")
		if err != nil {
			t.Fatalf("sender: %v", err)
		}
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: "f1", UID: "1", MessageID: "<" + id + "@x>", From: from,
		Subject: "Subject " + id, Date: when, Size: 1024, Flags: domain.NewFlags(domain.FlagSeen),
		HasAttachments: withSender, Snippet: "snippet " + id,
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

func TestMessageRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	older := buildMessage(t, "m1", time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC), true)
	newer := buildMessage(t, "m2", time.Date(2026, time.July, 2, 8, 0, 0, 0, time.UTC), false)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{older, newer}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	messages, err := store.ListMessages(ctx, "f1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	// Newest first.
	if messages[0].ID() != "m2" {
		t.Errorf("ordering wrong: first = %q, want m2", messages[0].ID())
	}
	if messages[1].From().Address() != "alice@example.com" {
		t.Errorf("sender lost: %q", messages[1].From().Address())
	}
	if !messages[1].From().IsZero() && messages[0].From().IsZero() {
		// m2 had no sender; confirm it round-tripped as zero.
	}
	if !messages[0].From().IsZero() {
		t.Errorf("expected zero sender for m2, got %q", messages[0].From().Address())
	}
	if !messages[0].IsRead() {
		t.Error("Seen flag lost in round trip")
	}
}

func TestSetSeen(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.SetSeen(ctx, "m1", false); err != nil {
		t.Fatalf("clear seen: %v", err)
	}
	msgs, _ := store.ListMessages(ctx, "f1")
	if msgs[0].IsRead() {
		t.Error("expected unread after clearing seen")
	}

	if err := store.SetSeen(ctx, "m1", true); err != nil {
		t.Fatalf("set seen: %v", err)
	}
	msgs, _ = store.ListMessages(ctx, "f1")
	if !msgs[0].IsRead() {
		t.Error("expected read after setting seen")
	}

	if err := store.SetSeen(ctx, "missing", true); err == nil {
		t.Error("expected error for a missing message")
	}
}

func buildTag(t *testing.T, id, name, hex string) domain.Tag {
	t.Helper()
	colour, err := domain.NewColour(hex)
	if err != nil {
		t.Fatalf("colour: %v", err)
	}
	tag, err := domain.NewTag(id, name, colour)
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	return tag
}

func TestTagRoundTripAndAssignment(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveTag(ctx, buildTag(t, "t1", "Work", "#3366ff")); err != nil {
		t.Fatalf("save tag: %v", err)
	}
	if err := store.SaveTag(ctx, buildTag(t, "t2", "Personal", "#ff8800")); err != nil {
		t.Fatalf("save tag: %v", err)
	}

	tags, err := store.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 2 || tags[0].Name() != "Personal" {
		t.Fatalf("expected 2 tags ordered by name, got %+v", tags)
	}

	if err := store.AddMessageTag(ctx, "m1", "t1"); err != nil {
		t.Fatalf("attach: %v", err)
	}
	// Re-attaching is a no-op, not an error.
	if err := store.AddMessageTag(ctx, "m1", "t1"); err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if err := store.AddMessageTag(ctx, "m1", "t2"); err != nil {
		t.Fatalf("attach: %v", err)
	}

	forMsg, err := store.TagsForMessage(ctx, "m1")
	if err != nil {
		t.Fatalf("tags for message: %v", err)
	}
	if len(forMsg) != 2 {
		t.Fatalf("expected 2 tags on m1, got %d", len(forMsg))
	}

	if err := store.RemoveMessageTag(ctx, "m1", "t1"); err != nil {
		t.Fatalf("detach: %v", err)
	}
	forMsg, _ = store.TagsForMessage(ctx, "m1")
	if len(forMsg) != 1 || forMsg[0].ID() != "t2" {
		t.Fatalf("expected only t2 left, got %+v", forMsg)
	}

	// Deleting a tag detaches it everywhere.
	if err := store.DeleteTag(ctx, "t2"); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	forMsg, _ = store.TagsForMessage(ctx, "m1")
	if len(forMsg) != 0 {
		t.Errorf("expected no tags after deleting t2, got %d", len(forMsg))
	}
	tags, _ = store.ListTags(ctx)
	if len(tags) != 1 {
		t.Errorf("expected 1 tag remaining, got %d", len(tags))
	}
}

func TestMessageBodyCache(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if _, err := store.GetMessageBody(ctx, "m1"); !errors.Is(err, application.ErrBodyNotCached) {
		t.Errorf("expected ErrBodyNotCached, got %v", err)
	}

	body, _ := domain.NewMessageBody("m1", "plain text", "<p>html</p>")
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}
	got, err := store.GetMessageBody(ctx, "m1")
	if err != nil {
		t.Fatalf("get body: %v", err)
	}
	if got.Plain() != "plain text" || got.HTML() != "<p>html</p>" {
		t.Errorf("round-tripped body wrong: %+v", got)
	}
}

func TestGetMessageAndFolder(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 0, 1)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("save folders: %v", err)
	}
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	gotMsg, err := store.GetMessage(ctx, "m1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if gotMsg.FolderID() != "f1" || gotMsg.UID() != "1" {
		t.Errorf("message wrong: %+v", gotMsg)
	}
	gotFolder, err := store.GetFolder(ctx, "f1")
	if err != nil {
		t.Fatalf("get folder: %v", err)
	}
	if gotFolder.AccountID() != "a1" || gotFolder.Path() != "INBOX" {
		t.Errorf("folder wrong: %+v", gotFolder)
	}

	if _, err := store.GetFolder(ctx, "missing"); err == nil {
		t.Error("expected error for a missing folder")
	}
}

func TestSearchMessages(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	quarterly, _ := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: "m1", FolderID: "f1", UID: "1", Size: 10, Flags: domain.NewFlags(0),
		Subject: "Quarterly report", Snippet: "the numbers are in",
	})
	lunch, _ := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: "m2", FolderID: "f1", UID: "2", Size: 10, Flags: domain.NewFlags(0),
		Subject: "Lunch plans", Snippet: "pizza on friday",
	})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{quarterly, lunch}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	// Prefix match on the subject.
	results, err := store.SearchMessages(ctx, "quart")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].ID() != "m1" {
		t.Fatalf("expected only m1 for 'quart', got %+v", results)
	}

	// Match against the snippet.
	results, _ = store.SearchMessages(ctx, "pizza")
	if len(results) != 1 || results[0].ID() != "m2" {
		t.Fatalf("expected only m2 for 'pizza', got %+v", results)
	}

	// An empty query returns nothing.
	results, _ = store.SearchMessages(ctx, "   ")
	if len(results) != 0 {
		t.Errorf("empty query should return no results, got %d", len(results))
	}

	// Re-saving the folder keeps the index in step (no duplicate results).
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{quarterly, lunch}); err != nil {
		t.Fatalf("resave: %v", err)
	}
	results, _ = store.SearchMessages(ctx, "quart")
	if len(results) != 1 {
		t.Errorf("expected 1 result after resync, got %d", len(results))
	}
}

func TestReopenPersistsAndMigratesOnce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "persist.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	accounts, err := reopened.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list after reopen: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("expected persisted account after reopen, got %d", len(accounts))
	}
}
