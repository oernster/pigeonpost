package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// newAccountInboxFixture is an account just added while the app runs: its inbox folder is listed in the
// store but has never been synced, so nothing is cached and it has no baseline mark. The server holds
// two existing messages from news@, which a mark-read rule matches.
func newAccountInboxFixture(t *testing.T) (*fakeMailStore, *fakeMailSource, *SyncService) {
	t.Helper()
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	inbox := testFolder(t, "f1", "a1", "INBOX")
	mail.folders["a1"] = []domain.Folder{inbox}
	source := &fakeMailSource{
		folders: []domain.Folder{inbox},
		messagesByFolder: map[string][]domain.MessageSummary{"f1": {
			newsArrival(t, "m1", "f1", "1"), newsArrival(t, "m2", "f1", "2"),
		}},
	}
	rules := &fakeRuleStore{rules: []domain.Rule{newMarkReadRule(t, "r1", "news@")}}
	remote := &fakeMailActions{}
	svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{},
		NewFlagSyncService(mail, accounts, remote), NewRuleExecutor(mail, remote))
	return mail, source, svc
}

// A background pass (the IDLE watcher or the poll) that reaches a newly added account's inbox before the
// front end does must not announce the mail already in it, nor let the rules act on it: a folder never
// baselined holds a starting point, not arrivals. Mail arriving after that first pass is announced and
// filtered as usual.
func TestSyncAccountInboxDoesNotAnnounceANewAccountsExistingMail(t *testing.T) {
	mail, source, svc := newAccountInboxFixture(t)
	ctx := context.Background()

	fresh, err := svc.SyncAccountInbox(ctx, "a1")
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("the existing inbox was announced as new mail: %v", inboxIDs(fresh))
	}
	saved := mail.messages["f1"]
	if len(saved) != 2 {
		t.Fatalf("saved %d messages, want the 2 already there", len(saved))
	}
	for _, m := range saved {
		if m.IsRead() {
			t.Errorf("a rule acted on %q, which was already in the inbox", m.ID())
		}
	}
	if len(mail.pendingFlags) != 0 {
		t.Errorf("rule marks recorded for existing mail: %v", mail.pendingFlags)
	}

	source.messagesByFolder["f1"] = append(source.messagesByFolder["f1"], newsArrival(t, "m3", "f1", "3"))
	fresh, err = svc.SyncAccountInbox(ctx, "a1")
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	// m3 is unread on the server and the rule marks it read, which silences its announcement; it must
	// still have been treated as an arrival, so the rule's mark lands on it.
	if len(fresh) != 0 {
		t.Errorf("fresh = %v, want none (the rule marked the arrival read)", inboxIDs(fresh))
	}
	if mail.pendingFlags["m3"][domain.FlagSeen] != true {
		t.Errorf("the rule did not act on the genuine arrival m3: pending %v", mail.pendingFlags)
	}
}

// A baseline the store cannot read skips the inbox and reports it, rather than guessing: guessing
// baselined would announce the whole inbox; guessing not would silence a real arrival.
func TestSyncAccountInboxReportsABaselineReadThatFailed(t *testing.T) {
	mail, _, svc := newAccountInboxFixture(t)
	mail.baselinedErr = errBoom
	fresh, err := svc.SyncAccountInbox(context.Background(), "a1")
	if !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want the failed baseline read reported", err)
	}
	if len(fresh) != 0 || len(mail.messages["f1"]) != 0 {
		t.Errorf("fresh = %v, saved %d: want the inbox skipped", inboxIDs(fresh), len(mail.messages["f1"]))
	}
}

// The same pass without a rule announces the genuine arrival after the first pass; nothing else.
func TestSyncAccountInboxAnnouncesMailArrivingAfterTheFirstPass(t *testing.T) {
	mail, source, svc := newAccountInboxFixture(t)
	svc.rules.(*fakeRuleStore).rules = nil
	ctx := context.Background()

	if fresh, err := svc.SyncAccountInbox(ctx, "a1"); err != nil || len(fresh) != 0 {
		t.Fatalf("first pass: fresh %v, err %v; want none", inboxIDs(fresh), err)
	}
	source.messagesByFolder["f1"] = append(source.messagesByFolder["f1"], newsArrival(t, "m3", "f1", "3"))
	fresh, err := svc.SyncAccountInbox(ctx, "a1")
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if got := inboxIDs(fresh); len(got) != 1 || got[0] != "m3" {
		t.Errorf("fresh = %v, want [m3]", got)
	}
	if len(mail.messages["f1"]) != 3 {
		t.Errorf("saved %d messages, want 3", len(mail.messages["f1"]))
	}
}
