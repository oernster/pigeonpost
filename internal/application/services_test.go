package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

var errBoom = errors.New("boom")

func TestAccountServiceList(t *testing.T) {
	store := newFakeAccountStore()
	store.accounts["a1"] = testAccount(t, "a1")
	svc := NewAccountService(store)

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
	store := newFakeAccountStore()
	svc := NewAccountService(store)

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

func TestMailboxServiceMarkRead(t *testing.T) {
	store := newFakeMailStore()
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	svc := NewMailboxService(store)

	if err := svc.MarkRead(context.Background(), "m1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.messages["f1"][0].IsRead() {
		t.Error("expected message to be marked read")
	}

	store.setSeenErr = errBoom
	if err := svc.MarkRead(context.Background(), "m1", false); !errors.Is(err, errBoom) {
		t.Errorf("MarkRead error = %v, want wrapped boom", err)
	}
}
