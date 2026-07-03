package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newSyncFixture(t *testing.T) (*fakeAccountStore, *fakeMailStore, *fakeMailSource, *SyncService) {
	t.Helper()
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")

	mail := newFakeMailStore()

	source := &fakeMailSource{
		folders: []domain.Folder{
			testFolder(t, "f1", "a1", "INBOX"),
			testFolder(t, "f2", "a1", "INBOX/Archive"),
		},
		messagesByFolder: map[string][]domain.MessageSummary{
			"f1": {testMessage(t, "m1", "f1"), testMessage(t, "m2", "f1")},
			"f2": {testMessage(t, "m3", "f2")},
		},
	}
	return accounts, mail, source, NewSyncService(accounts, mail, source)
}

func TestSyncAccountHappyPath(t *testing.T) {
	_, mail, _, svc := newSyncFixture(t)

	if err := svc.SyncAccount(context.Background(), "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mail.folders["a1"]) != 2 {
		t.Errorf("expected 2 folders persisted, got %d", len(mail.folders["a1"]))
	}
	if len(mail.messages["f1"]) != 2 {
		t.Errorf("expected 2 messages in f1, got %d", len(mail.messages["f1"]))
	}
	if len(mail.messages["f2"]) != 1 {
		t.Errorf("expected 1 message in f2, got %d", len(mail.messages["f2"]))
	}
}

func TestSyncAccountErrors(t *testing.T) {
	t.Run("load account", func(t *testing.T) {
		accounts, mail, source, _ := newSyncFixture(t)
		accounts.getErr = errBoom
		svc := NewSyncService(accounts, mail, source)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("fetch folders", func(t *testing.T) {
		accounts, mail, source, _ := newSyncFixture(t)
		source.fetchFoldersErr = errBoom
		svc := NewSyncService(accounts, mail, source)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("save folders", func(t *testing.T) {
		accounts, mail, source, _ := newSyncFixture(t)
		mail.saveFoldersErr = errBoom
		svc := NewSyncService(accounts, mail, source)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("fetch messages", func(t *testing.T) {
		accounts, mail, source, _ := newSyncFixture(t)
		source.fetchMessagesErr = errBoom
		svc := NewSyncService(accounts, mail, source)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("save messages", func(t *testing.T) {
		accounts, mail, source, _ := newSyncFixture(t)
		mail.saveMessagesErr = errBoom
		svc := NewSyncService(accounts, mail, source)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}
