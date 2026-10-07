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
	return accounts, mail, source, rules, NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{}, NewRuleExecutor(mail, &fakeMailActions{}))
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
	rules.rules = []domain.Rule{newMarkReadRule(t, "r1", "news@")}

	fresh, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("SyncInboxes: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none: a rule-read message should not notify", inboxIDs(fresh))
	}
}

// twoInboxFixture adds a second account a2, with inbox f2, to inboxFixture, each inbox holding one new
// message on the server.
func twoInboxFixture(t *testing.T) (*fakeMailStore, *SyncService) {
	t.Helper()
	accounts, mail, source, _, svc := inboxFixture(t)
	accounts.accounts["a2"] = testAccount(t, "a2")
	mail.folders["a2"] = []domain.Folder{testFolder(t, "f2", "a2", "INBOX")}
	source.messagesByFolder["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	source.messagesByFolder["f2"] = []domain.MessageSummary{testMessage(t, "m2", "f2")}
	return mail, svc
}

// An IDLE push names one account, so only that account's inbox is fetched; the other account's inbox is
// neither reported nor saved.
func TestSyncAccountInboxSyncsOnlyThatAccount(t *testing.T) {
	mail, svc := twoInboxFixture(t)
	fresh, err := svc.SyncAccountInbox(context.Background(), "a2")
	if err != nil {
		t.Fatalf("SyncAccountInbox: %v", err)
	}
	if got := inboxIDs(fresh); len(got) != 1 || got[0] != "m2" {
		t.Errorf("fresh = %v, want [m2]", got)
	}
	if len(mail.messages["f1"]) != 0 {
		t.Errorf("a1's inbox was synced: %d messages saved, want 0", len(mail.messages["f1"]))
	}
}

// A tag or flag replay that fails does not stop the pass: the inbox is still synced and its arrivals
// answered, while both failures are reported for the caller to record.
func TestSyncInboxesReportsFailedReplaysAndCarriesOn(t *testing.T) {
	accounts, mail, source, rules, _ := inboxFixture(t)
	tagFailure, flagFailure := errors.New("tag replay"), errors.New("flag replay")
	svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{flushErr: tagFailure},
		&fakeFlagSyncer{flushErr: flagFailure}, NewRuleExecutor(mail, &fakeMailActions{}))
	source.messagesByFolder["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}

	fresh, err := svc.SyncInboxes(context.Background())
	if !errors.Is(err, tagFailure) || !errors.Is(err, flagFailure) {
		t.Errorf("err = %v, want both failed replays reported", err)
	}
	if got := inboxIDs(fresh); len(got) != 1 || got[0] != "m1" {
		t.Errorf("fresh = %v, want [m1]: a failed replay must not stop the pass", got)
	}
}

// An id naming no account syncs nothing rather than falling back to every account.
func TestSyncAccountInboxUnknownAccountSyncsNothing(t *testing.T) {
	mail, svc := twoInboxFixture(t)
	fresh, err := svc.SyncAccountInbox(context.Background(), "gone")
	if err != nil {
		t.Fatalf("SyncAccountInbox: %v", err)
	}
	if len(fresh) != 0 || len(mail.messages["f1"]) != 0 || len(mail.messages["f2"]) != 0 {
		t.Errorf("fresh = %v; saved f1 %d, f2 %d: want nothing", inboxIDs(fresh), len(mail.messages["f1"]), len(mail.messages["f2"]))
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
	// The account is skipped so the pass goes on; the skip is reported rather than lost.
	if !errors.Is(err, errBoom) {
		t.Fatalf("the skipped account must be reported, got %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none", inboxIDs(fresh))
	}
}

func TestSyncInboxesListMessagesErrorSkipsFolder(t *testing.T) {
	_, mail, _, _, svc := inboxFixture(t)
	mail.listMessagesErr = errBoom
	fresh, err := svc.SyncInboxes(context.Background())
	if !errors.Is(err, errBoom) {
		t.Fatalf("the skipped folder must be reported, got %v", err)
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
	if !errors.Is(err, errBoom) {
		t.Fatalf("the skipped folder must be reported, got %v", err)
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
	if !errors.Is(err, errBoom) {
		t.Fatalf("the skipped folder must be reported, got %v", err)
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
