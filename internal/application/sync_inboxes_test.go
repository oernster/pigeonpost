package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// readMessage builds a message summary that is already marked read.
func readMessage(t *testing.T, id, folderID string) domain.MessageSummary {
	t.Helper()
	m, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: "1", Size: 10, Flags: domain.NewFlags(domain.FlagSeen),
	})
	if err != nil {
		t.Fatalf("build read message: %v", err)
	}
	return m
}

// inboxFixture wires a sync service with one IMAP account whose inbox folder f1 is already cached, so
// SyncInboxes treats it as an already-populated inbox rather than a first population.
func inboxFixture(t *testing.T) (*fakeAccountStore, *fakeMailStore, *fakeMailSource, *fakeRuleStore, *SyncService) {
	t.Helper()
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	source := &fakeMailSource{messagesByFolder: map[string][]domain.MessageSummary{}}
	rules := &fakeRuleStore{}
	return accounts, mail, source, rules, NewSyncService(accounts, mail, source, rules)
}

func inboxIDs(messages []domain.MessageSummary) []string {
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		out = append(out, m.ID())
	}
	return out
}

func TestSyncInboxesReturnsNewMailRegardlessOfReadState(t *testing.T) {
	_, mail, source, _, svc := inboxFixture(t)
	mail.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	source.messagesByFolder["f1"] = []domain.MessageSummary{
		testMessage(t, "m1", "f1"), // already known
		testMessage(t, "m2", "f1"), // new and unread
		readMessage(t, "m3", "f1"), // new, already read on the server (e.g. by another client)
	}
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("SyncInboxes: %v", err)
	}
	// m3 is a new arrival even though another client already read it, so it is announced alongside m2.
	if got := inboxIDs(fresh); len(got) != 2 || got[0] != "m2" || got[1] != "m3" {
		t.Fatalf("fresh = %v, want [m2 m3]", got)
	}
	if len(mail.messages["f1"]) != 3 {
		t.Errorf("saved %d messages, want 3", len(mail.messages["f1"]))
	}
}

func TestSyncInboxesSkipsMailAFilterRuleMarkedRead(t *testing.T) {
	_, mail, source, rules, svc := inboxFixture(t)
	mail.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	from, err := domain.NewEmailAddress("", "news@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	// m9 is unread on the server; a mark-read rule matches it, so its notification is silenced.
	m9, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: "m9", FolderID: "f1", UID: "9", From: from, Subject: "Weekly", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	source.messagesByFolder["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1"), m9}
	rule, err := domain.NewRule("r1", "News", domain.RuleFieldFrom, domain.RuleOpContains, "news@", domain.RuleMarkRead)
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	rules.rules = []domain.Rule{rule}

	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("SyncInboxes: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none: a rule-read message should not notify", inboxIDs(fresh))
	}
}

func TestSyncInboxesReportsMailIntoEmptyFolder(t *testing.T) {
	// A message arriving into a folder with nothing cached is still reported: the poller establishes the
	// baseline with a priming call, so an empty inbox does not silence its first real arrival.
	_, mail, source, _, svc := inboxFixture(t)
	source.messagesByFolder["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("SyncInboxes: %v", err)
	}
	if got := inboxIDs(fresh); len(got) != 1 || got[0] != "m1" {
		t.Errorf("fresh = %v, want [m1]", got)
	}
	if len(mail.messages["f1"]) != 1 {
		t.Errorf("the message should be persisted, got %d", len(mail.messages["f1"]))
	}
}

func TestSyncInboxesPreservesPop3FlagsAndDetectsNew(t *testing.T) {
	accounts, mail, source, _, svc := inboxFixture(t)
	accounts.accounts["a1"] = pop3Account(t, "a1")
	mail.messages["f1"] = []domain.MessageSummary{readMessage(t, "m1", "f1")}
	// POP3 reports every message as unread, so m1 must keep its stored read flag; m2 is genuinely new.
	source.messagesByFolder["f1"] = []domain.MessageSummary{
		testMessage(t, "m1", "f1"),
		testMessage(t, "m2", "f1"),
	}
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("SyncInboxes: %v", err)
	}
	if got := inboxIDs(fresh); len(got) != 1 || got[0] != "m2" {
		t.Fatalf("fresh = %v, want [m2]", got)
	}
	for _, m := range mail.messages["f1"] {
		if m.ID() == "m1" && !m.IsRead() {
			t.Errorf("m1 read flag was lost across the POP3 sync")
		}
	}
}

func TestSyncInboxesSkipsNonInboxFolders(t *testing.T) {
	_, mail, source, _, svc := inboxFixture(t)
	sent, err := domain.NewFolder("f2", "a1", "Sent", domain.FolderSent, 0, 0)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	mail.folders["a1"] = []domain.Folder{sent}
	mail.messages["f2"] = []domain.MessageSummary{testMessage(t, "m1", "f2")}
	source.messagesByFolder["f2"] = []domain.MessageSummary{
		testMessage(t, "m1", "f2"), testMessage(t, "m2", "f2"),
	}
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("SyncInboxes: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("a non-inbox folder should be ignored, got %v", inboxIDs(fresh))
	}
}

func TestSyncInboxesListFoldersErrorSkipsAccount(t *testing.T) {
	_, mail, _, _, svc := inboxFixture(t)
	mail.listFoldersErr = errBoom
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("should skip the account, not fail: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none", inboxIDs(fresh))
	}
}

func TestSyncInboxesListMessagesErrorSkipsFolder(t *testing.T) {
	_, mail, _, _, svc := inboxFixture(t)
	mail.listMessagesErr = errBoom
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("should skip the folder, not fail: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none", inboxIDs(fresh))
	}
}

func TestSyncInboxesFetchErrorSkipsFolder(t *testing.T) {
	_, mail, source, _, svc := inboxFixture(t)
	mail.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	source.fetchMessagesErr = errBoom
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("should skip the folder, not fail: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none", inboxIDs(fresh))
	}
}

func TestSyncInboxesSaveErrorSkipsFolder(t *testing.T) {
	_, mail, source, _, svc := inboxFixture(t)
	mail.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	source.messagesByFolder["f1"] = []domain.MessageSummary{testMessage(t, "m2", "f1")}
	mail.saveMessagesErr = errBoom
	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("should skip the folder, not fail: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none", inboxIDs(fresh))
	}
}

func TestSyncInboxesListAccountsError(t *testing.T) {
	accounts, _, _, _, svc := inboxFixture(t)
	accounts.listErr = errBoom
	if _, err := svc.SyncInboxes(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestSyncInboxesListRulesError(t *testing.T) {
	_, _, _, rules, svc := inboxFixture(t)
	rules.listErr = errBoom
	if _, err := svc.SyncInboxes(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}
