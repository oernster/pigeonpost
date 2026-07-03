package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newSyncFixture(t *testing.T) (*fakeAccountStore, *fakeMailStore, *fakeMailSource, *fakeRuleStore, *SyncService) {
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
	rules := &fakeRuleStore{}
	return accounts, mail, source, rules, NewSyncService(accounts, mail, source, rules)
}

func TestSyncAccountHappyPath(t *testing.T) {
	_, mail, _, _, svc := newSyncFixture(t)

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

func TestSyncAppliesRules(t *testing.T) {
	accounts, mail, source, rules, svc := newSyncFixture(t)

	from, err := domain.NewEmailAddress("", "news@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: "m9", FolderID: "f1", UID: 5, From: from, Subject: "Weekly", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	source.messagesByFolder = map[string][]domain.MessageSummary{"f1": {msg}}
	source.folders = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	rule, err := domain.NewRule("r1", "News", domain.RuleFieldFrom, "news@", domain.RuleMarkRead)
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	rules.rules = []domain.Rule{rule}
	_ = accounts

	if err := svc.SyncAccount(context.Background(), "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved := mail.messages["f1"]
	if len(saved) != 1 || !saved[0].IsRead() {
		t.Errorf("expected the matching message to be marked read on arrival, got %+v", saved)
	}
}

func TestSyncAccountErrors(t *testing.T) {
	t.Run("load account", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		accounts.getErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("load rules", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		rules.listErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("fetch folders", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		source.fetchFoldersErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("save folders", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		mail.saveFoldersErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("fetch messages", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		source.fetchMessagesErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("save messages", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		mail.saveMessagesErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules)
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

// seedFolderForSync puts folder f1 into the store so SyncFolder can resolve it by id.
func seedFolderForSync(t *testing.T, mail *fakeMailStore) {
	t.Helper()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
}

func TestSyncFolderHappyPath(t *testing.T) {
	_, mail, _, _, svc := newSyncFixture(t)
	seedFolderForSync(t, mail)

	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mail.messages["f1"]) != 2 {
		t.Errorf("expected 2 messages saved for f1, got %d", len(mail.messages["f1"]))
	}
}

func TestSyncFolderAppliesRules(t *testing.T) {
	_, mail, source, rules, svc := newSyncFixture(t)
	seedFolderForSync(t, mail)
	from, err := domain.NewEmailAddress("", "news@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: "m9", FolderID: "f1", UID: 5, From: from, Subject: "Weekly", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	source.messagesByFolder = map[string][]domain.MessageSummary{"f1": {msg}}
	rule, err := domain.NewRule("r1", "News", domain.RuleFieldFrom, "news@", domain.RuleMarkRead)
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	rules.rules = []domain.Rule{rule}

	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved := mail.messages["f1"]
	if len(saved) != 1 || !saved[0].IsRead() {
		t.Errorf("expected the matching message marked read, got %+v", saved)
	}
}

func TestSyncFolderErrors(t *testing.T) {
	t.Run("load folder", func(t *testing.T) {
		_, mail, _, _, svc := newSyncFixture(t)
		mail.getFolderErr = errBoom
		if err := svc.SyncFolder(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("load account", func(t *testing.T) {
		accounts, mail, _, _, svc := newSyncFixture(t)
		seedFolderForSync(t, mail)
		accounts.getErr = errBoom
		if err := svc.SyncFolder(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("load rules", func(t *testing.T) {
		_, mail, _, rules, svc := newSyncFixture(t)
		seedFolderForSync(t, mail)
		rules.listErr = errBoom
		if err := svc.SyncFolder(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("fetch messages", func(t *testing.T) {
		_, mail, source, _, svc := newSyncFixture(t)
		seedFolderForSync(t, mail)
		source.fetchMessagesErr = errBoom
		if err := svc.SyncFolder(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("save messages", func(t *testing.T) {
		_, mail, _, _, svc := newSyncFixture(t)
		seedFolderForSync(t, mail)
		mail.saveMessagesErr = errBoom
		if err := svc.SyncFolder(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}
