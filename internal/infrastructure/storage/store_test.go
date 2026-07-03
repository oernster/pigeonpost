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
		ID: id, FolderID: "f1", UID: 1, MessageID: "<" + id + "@x>", From: from,
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
