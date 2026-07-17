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
	return accounts, mail, source, rules, NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{})
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
		ID: "m9", FolderID: "f1", UID: "5", From: from, Subject: "Weekly", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	source.messagesByFolder = map[string][]domain.MessageSummary{"f1": {msg}}
	source.folders = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	rule, err := domain.NewRule("r1", "News", domain.RuleFieldFrom, domain.RuleOpContains, "news@", domain.RuleMarkRead)
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

func TestSyncReconcilesTagsForImapOnly(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{"f1": {testMessage(t, "m1", "f1")}}}
	tagSync := &fakeTagSyncer{}
	svc := NewSyncService(accounts, mail, source, &fakeRuleStore{}, tagSync, &fakeFlagSyncer{})

	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("sync folder: %v", err)
	}
	if tagSync.flushCalls == 0 {
		t.Error("expected pending tag changes to be flushed on sync")
	}
	if len(tagSync.reconciled) != 1 || len(tagSync.reconciled[0]) != 1 {
		t.Errorf("expected the fetched messages reconciled once, got %v", tagSync.reconciled)
	}
}

func TestSyncSkipsTagReconcileForPop3(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = pop3Account(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{"f1": {pop3Summary(t, "m1", "1", domain.NewFlags(0))}}}
	tagSync := &fakeTagSyncer{}
	svc := NewSyncService(accounts, mail, source, &fakeRuleStore{}, tagSync, &fakeFlagSyncer{})

	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("sync folder: %v", err)
	}
	if tagSync.flushCalls == 0 {
		t.Error("a pending flush should still run for a POP3 account")
	}
	if len(tagSync.reconciled) != 0 {
		t.Errorf("a POP3 folder carries no server keywords, so reconcile must be skipped, got %v", tagSync.reconciled)
	}
}

func TestSyncIgnoresTagSyncError(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{"f1": {testMessage(t, "m1", "f1")}}}
	tagSync := &fakeTagSyncer{reconcileErr: errBoom, flushErr: errBoom}
	svc := NewSyncService(accounts, mail, source, &fakeRuleStore{}, tagSync, &fakeFlagSyncer{})

	// A tag flush or reconcile failure is best-effort and must never fail the mail sync.
	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Errorf("sync should ignore a tag-sync error, got %v", err)
	}
}

func TestSyncOverlaysPendingFlagsBeforeSave(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{"f1": {testMessage(t, "m1", "f1")}}}
	flagSync := &fakeFlagSyncer{rewrite: func(messages []domain.MessageSummary) []domain.MessageSummary {
		out := make([]domain.MessageSummary, len(messages))
		for i, m := range messages {
			out[i] = m.WithFlags(m.Flags().With(domain.FlagSeen))
		}
		return out
	}}
	svc := NewSyncService(accounts, mail, source, &fakeRuleStore{}, &fakeTagSyncer{}, flagSync)

	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("sync folder: %v", err)
	}
	if flagSync.flushCalls == 0 {
		t.Error("expected pending flag changes to be flushed on sync")
	}
	if len(flagSync.reconciled) != 1 {
		t.Fatalf("expected the fetched messages reconciled once, got %v", flagSync.reconciled)
	}
	// What lands in the store is the reconciled set, so a save cannot regress a pending local change.
	if !mail.messages["f1"][0].IsRead() {
		t.Error("the saved messages must carry the reconcile's overlay")
	}
}

func TestSyncSkipsFlagReconcileForPop3(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = pop3Account(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{"f1": {pop3Summary(t, "m1", "1", domain.NewFlags(0))}}}
	flagSync := &fakeFlagSyncer{}
	svc := NewSyncService(accounts, mail, source, &fakeRuleStore{}, &fakeTagSyncer{}, flagSync)

	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("sync folder: %v", err)
	}
	if flagSync.flushCalls == 0 {
		t.Error("a pending flush should still run for a POP3 account")
	}
	if len(flagSync.reconciled) != 0 {
		t.Errorf("a POP3 account records no pending flag ops, so reconcile must be skipped, got %v", flagSync.reconciled)
	}
}

func TestSyncIgnoresFlagSyncError(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{"f1": {testMessage(t, "m1", "f1")}}}
	flagSync := &fakeFlagSyncer{reconcileErr: errBoom, flushErr: errBoom}
	svc := NewSyncService(accounts, mail, source, &fakeRuleStore{}, &fakeTagSyncer{}, flagSync)

	// A flag flush or reconcile failure is best-effort and must never fail the mail sync; the fetched
	// messages are saved as reported and the intents stay for the next pass.
	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Errorf("sync should ignore a flag-sync error, got %v", err)
	}
	if len(mail.messages["f1"]) != 1 {
		t.Errorf("expected the fetched messages saved despite the reconcile error, got %d", len(mail.messages["f1"]))
	}
}

func TestSyncInboxesOverlaysPendingFlags(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	// The message is already cached (so it is not announced as new) and the server still reports it
	// unseen; the overlay must keep the pending local read state in what the pass saves.
	mail.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{"f1": {testMessage(t, "m1", "f1")}}}
	flagSync := &fakeFlagSyncer{rewrite: func(messages []domain.MessageSummary) []domain.MessageSummary {
		out := make([]domain.MessageSummary, len(messages))
		for i, m := range messages {
			out[i] = m.WithFlags(m.Flags().With(domain.FlagSeen))
		}
		return out
	}}
	svc := NewSyncService(accounts, mail, source, &fakeRuleStore{}, &fakeTagSyncer{}, flagSync)

	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("sync inboxes: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("no new mail expected, got %v", fresh)
	}
	if !mail.messages["f1"][0].IsRead() {
		t.Error("the inbox poll's save must carry the reconcile's overlay")
	}
}

func TestSyncAccountErrors(t *testing.T) {
	t.Run("load account", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		accounts.getErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{})
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("load rules", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		rules.listErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{})
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("fetch folders", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		source.fetchFoldersErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{})
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("save folders", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		mail.saveFoldersErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{})
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("fetch messages", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		source.fetchMessagesErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{})
		if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("save messages", func(t *testing.T) {
		accounts, mail, source, rules, _ := newSyncFixture(t)
		mail.saveMessagesErr = errBoom
		svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{})
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
		ID: "m9", FolderID: "f1", UID: "5", From: from, Subject: "Weekly", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	source.messagesByFolder = map[string][]domain.MessageSummary{"f1": {msg}}
	rule, err := domain.NewRule("r1", "News", domain.RuleFieldFrom, domain.RuleOpContains, "news@", domain.RuleMarkRead)
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

// pop3Account builds a POP3 account so the flag-preservation path can be exercised.
func pop3Account(t *testing.T, id string) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	sc, err := domain.NewServerConfig("host.example.com", 995, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	account, err := domain.NewAccount(id, "Test", addr, domain.ProtocolPOP3, sc, sc, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

// pop3Summary builds a summary with the given id, uid and flags for the POP3 preservation tests.
func pop3Summary(t *testing.T, id, uid string, flags domain.Flags) domain.MessageSummary {
	t.Helper()
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: "f1", UID: uid, Size: 1, Flags: flags,
	})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	return msg
}

func TestSyncFolderPreservesPop3ReadState(t *testing.T) {
	accounts, mail, source, _, svc := newSyncFixture(t)
	accounts.accounts["a1"] = pop3Account(t, "a1")
	seedFolderForSync(t, mail)

	// An existing cached message that the user has read.
	mail.messages["f1"] = []domain.MessageSummary{
		pop3Summary(t, "f1\x1fu1", "u1", domain.NewFlags(domain.FlagSeen)),
	}
	// The POP3 fetch reports every message as unread, since POP3 carries no server flags. u1 is the
	// already-read message; u2 is genuinely new.
	source.messagesByFolder = map[string][]domain.MessageSummary{
		"f1": {
			pop3Summary(t, "f1\x1fu1", "u1", domain.NewFlags(0)),
			pop3Summary(t, "f1\x1fu2", "u2", domain.NewFlags(0)),
		},
	}

	if err := svc.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("sync: %v", err)
	}

	byID := make(map[string]domain.MessageSummary)
	for _, m := range mail.messages["f1"] {
		byID[m.ID()] = m
	}
	if !byID["f1\x1fu1"].IsRead() {
		t.Error("an existing POP3 message should keep its local read state across a sync")
	}
	if byID["f1\x1fu2"].IsRead() {
		t.Error("a newly fetched POP3 message should remain unread")
	}
}

func TestSyncFolderPop3PreserveFlagsError(t *testing.T) {
	accounts, mail, _, _, svc := newSyncFixture(t)
	accounts.accounts["a1"] = pop3Account(t, "a1")
	seedFolderForSync(t, mail)
	mail.listMessagesErr = errBoom
	if err := svc.SyncFolder(context.Background(), "f1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestSyncAccountPop3PreserveFlagsError(t *testing.T) {
	accounts, mail, _, _, svc := newSyncFixture(t)
	accounts.accounts["a1"] = pop3Account(t, "a1")
	mail.listMessagesErr = errBoom
	if err := svc.SyncAccount(context.Background(), "a1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}
