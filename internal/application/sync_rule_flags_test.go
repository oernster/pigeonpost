package application

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// newsArrival is an unread message from news@ under the given id and UID, the shape a mark-read rule
// matches.
func newsArrival(t *testing.T, id, folderID, uid string) domain.MessageSummary {
	t.Helper()
	from, err := domain.NewEmailAddress("", "news@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: uid, From: from, Subject: "Weekly", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

// ruleFlagsFixture wires a sync over one IMAP inbox holding one news@ arrival, with a mark-read rule and
// the real FlagSyncService, so pending intents are replayed and guarded exactly as in the app.
func ruleFlagsFixture(t *testing.T, protocol domain.Protocol) (*fakeMailStore, *fakeMailActions, *SyncService) {
	t.Helper()
	accounts := newFakeAccountStore()
	account := testAccount(t, "a1")
	if protocol == domain.ProtocolPOP3 {
		account = pop3Account(t, "a1")
	}
	accounts.accounts["a1"] = account
	mail := newFakeMailStore()
	// An established inbox, so the message counts as an arrival.
	mail.baselined = map[string]bool{"f1": true}
	source := &fakeMailSource{
		folders:          []domain.Folder{testFolder(t, "f1", "a1", "INBOX")},
		messagesByFolder: map[string][]domain.MessageSummary{"f1": {newsArrival(t, "m9", "f1", "5")}},
	}
	rules := &fakeRuleStore{rules: []domain.Rule{newMarkReadRule(t, "r1", "news@")}}
	remote := &fakeMailActions{}
	flags := NewFlagSyncService(mail, accounts, remote)
	svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, flags, NewRuleExecutor(mail, remote))
	return mail, remote, svc
}

// A rule that marks arriving IMAP mail read must reach the server, not only the cache: the next sync
// replays it. While the server still reports the message unread the sync keeps it read locally. Before the
// fix nothing recorded the rule's mark, so the second sync pushed nothing and saved the server's unread
// state over it.
func TestSyncRuleMarkReadReachesServerAndSurvivesNextSync(t *testing.T) {
	mail, remote, svc := ruleFlagsFixture(t, domain.ProtocolIMAP)
	// The server keeps reporting UID 5 unread after the push, as a lazy STORE does.
	remote.pushFlagUnsettled = map[string]bool{"5": true}
	ctx := context.Background()

	if err := svc.SyncAccount(ctx, "a1"); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if err := svc.SyncAccount(ctx, "a1"); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	pushed := false
	for _, b := range remote.pushFlagBatches {
		if b.flag == domain.FlagSeen && b.set && len(b.uids) == 1 && b.uids[0] == "5" {
			pushed = true
		}
	}
	if !pushed {
		t.Errorf("the rule's mark read was never sent to the server; pushes = %+v", remote.pushFlagBatches)
	}
	saved := mail.messages["f1"]
	if len(saved) != 1 || !saved[0].IsRead() {
		t.Errorf("the second sync undid the rule's mark read: saved %+v", saved)
	}
}

// readAndFlagRule is an enabled rule that both marks read and flags any message from news@.
func readAndFlagRule(t *testing.T) domain.Rule {
	t.Helper()
	cond, err := domain.NewRuleCondition(domain.RuleFieldFrom, domain.RuleOpContains, "news@")
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	read, err := domain.NewRuleAction(domain.RuleMarkRead, "")
	if err != nil {
		t.Fatalf("read action: %v", err)
	}
	flag, err := domain.NewRuleAction(domain.RuleFlag, "")
	if err != nil {
		t.Fatalf("flag action: %v", err)
	}
	rule, err := domain.NewRule(domain.RuleSpec{
		ID: "r1", Name: "r1", Enabled: true,
		Conditions: []domain.RuleCondition{cond}, Actions: []domain.RuleAction{read, flag},
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return rule
}

// Each message gets an intent for exactly the flags the rule added to it: a message the server already
// reports read gains only the flag, so only that is recorded for it.
func TestSyncRuleMarksRecordOnlyTheFlagsTheRuleAdded(t *testing.T) {
	mail, _, svc := ruleFlagsFixture(t, domain.ProtocolIMAP)
	alreadyRead := newsArrival(t, "m8", "f1", "4")
	alreadyRead = alreadyRead.WithFlags(alreadyRead.Flags().With(domain.FlagSeen))
	svc.source.(*fakeMailSource).messagesByFolder["f1"] = []domain.MessageSummary{
		newsArrival(t, "m9", "f1", "5"), alreadyRead,
	}
	svc.rules.(*fakeRuleStore).rules = []domain.Rule{readAndFlagRule(t)}

	if err := svc.SyncAccount(context.Background(), "a1"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	want := map[string]map[domain.Flag]bool{
		"m9": {domain.FlagSeen: true, domain.FlagFlagged: true},
		"m8": {domain.FlagFlagged: true},
	}
	if !reflect.DeepEqual(mail.pendingFlags, want) {
		t.Errorf("pending intents = %v, want %v", mail.pendingFlags, want)
	}
}

// An intent the store refuses fails the sync after the save, as a failed baseline mark does, on both
// the full and the single-folder path.
func TestSyncRuleMarksRecordFailureIsReported(t *testing.T) {
	for name, run := range map[string]func(*SyncService) error{
		"account": func(s *SyncService) error { return s.SyncAccount(context.Background(), "a1") },
		"folder":  func(s *SyncService) error { return s.SyncFolder(context.Background(), "f1") },
	} {
		t.Run(name, func(t *testing.T) {
			mail, _, svc := ruleFlagsFixture(t, domain.ProtocolIMAP)
			mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
			mail.setFlagErr = errBoom
			err := run(svc)
			if err == nil || !strings.Contains(err.Error(), "record rule flags") {
				t.Fatalf("err = %v, want a record rule flags failure", err)
			}
		})
	}
}

// A POP3 account has no server flags, so a rule's mark stays local (preserveFlags carries it) and no
// pending intent is recorded for a server that could never take it.
func TestSyncRuleFlagsRecordNoIntentForPOP3(t *testing.T) {
	mail, _, svc := ruleFlagsFixture(t, domain.ProtocolPOP3)
	if err := svc.SyncAccount(context.Background(), "a1"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(mail.pendingFlags) != 0 {
		t.Errorf("POP3 recorded pending flag intents: %v", mail.pendingFlags)
	}
	if saved := mail.messages["f1"]; len(saved) != 1 || !saved[0].IsRead() {
		t.Errorf("the rule's mark read was not saved locally: %+v", saved)
	}
}
