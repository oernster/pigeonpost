package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// errBackfill stands for whatever a store or a server refused with; the tests care that it survives to
// the caller, not what it says.
var errBackfill = errors.New("backfill boom")

func TestRuleBackfillReportsStoreFailures(t *testing.T) {
	cases := []struct {
		name string
		// break disables one of the reads the plan makes.
		set func(mail *fakeMailStore, accounts *fakeAccountStore, rules *fakeRuleStore)
	}{
		{"list rules", func(_ *fakeMailStore, _ *fakeAccountStore, rules *fakeRuleStore) {
			rules.listErr = errBackfill
		}},
		{"list accounts", func(_ *fakeMailStore, accounts *fakeAccountStore, _ *fakeRuleStore) {
			accounts.listErr = errBackfill
		}},
		{"list folders", func(mail *fakeMailStore, _ *fakeAccountStore, _ *fakeRuleStore) {
			mail.listFoldersErr = errBackfill
		}},
		{"list messages", func(mail *fakeMailStore, _ *fakeAccountStore, _ *fakeRuleStore) {
			mail.listMessagesErr = errBackfill
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := execRule(t, "r1", "news@", execAction(t, domain.RuleMarkRead, ""))
			svc, mail, accounts, rules, _ := backfillFixture(t, rule)
			mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "news@site.com", 0)}
			tc.set(mail, accounts, rules)

			if _, err := svc.Preview(context.Background(), "r1", nil); !errors.Is(err, errBackfill) {
				t.Errorf("failure did not reach the caller: %v", err)
			}
		})
	}
}

func TestRuleBackfillReportsAnUnreadableFolderYetKeepsGoing(t *testing.T) {
	// A folder whose contents cannot be read must not cancel the rest of the backfill, so the failure is
	// reported ALONGSIDE the work that was still possible, never instead of it.
	rule := execRule(t, "r1", "shop.com", execAction(t, domain.RuleMoveTo, "missing"))
	svc, mail, _, _, _ := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "billing@shop.com", 0)}
	mail.getFolderErr = errBackfill

	counts, err := svc.Preview(context.Background(), "r1", nil)
	if !errors.Is(err, errBackfill) {
		t.Errorf("unresolvable destination not reported: %v", err)
	}
	if counts.Scanned != 1 || counts.Move != 0 {
		t.Errorf("wrong counts after a failed resolve: %+v", counts)
	}
}

func TestRuleBackfillRunReportsAPlanningFailureAlongsideTheWorkItDid(t *testing.T) {
	// The rule marks mail read and moves it. The destination cannot be resolved, so the move is lost and
	// reported; the mark still lands; the run returns both.
	rule := execRule(t, "r1", "shop.com",
		execAction(t, domain.RuleMarkRead, ""), execAction(t, domain.RuleMoveTo, "missing"))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "billing@shop.com", 0)}
	mail.getFolderErr = errBackfill

	counts, err := svc.Run(context.Background(), "r1", nil)
	if !errors.Is(err, errBackfill) {
		t.Errorf("planning failure not reported by the run: %v", err)
	}
	if counts.MarkRead != 1 || counts.Move != 0 {
		t.Errorf("wrong counts: %+v", counts)
	}
	if len(actions.readIDs) != 1 {
		t.Errorf("the work that was still possible did not happen: %v", actions.readIDs)
	}
}

func TestRuleBackfillRunReportsWhatSucceededWhenActionsFail(t *testing.T) {
	rule := execRule(t, "r1", "news@",
		execAction(t, domain.RuleMarkRead, ""), execAction(t, domain.RuleFlag, ""))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "news@site.com", 0)}
	actions.readErr = errBackfill

	counts, err := svc.Run(context.Background(), "r1", nil)
	if !errors.Is(err, errBackfill) {
		t.Errorf("failed mark did not reach the caller: %v", err)
	}
	// The flag still landed, so the count reports one and not the other: the counts describe work that
	// succeeded, never the plan.
	if counts.MarkRead != 0 || counts.Flag != 1 {
		t.Errorf("wrong counts after a partial run: %+v", counts)
	}
}

func TestRuleBackfillRunReportsAPartiallyRefusedMoveAndDestroy(t *testing.T) {
	move := execRule(t, "mv", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))
	svc, mail, _, _, actions := backfillFixture(t, move)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "billing@shop.com", 0),
		backfillMessage(t, "m2", "f1", "orders@shop.com", 0),
	}
	actions.moveErr = errBackfill
	actions.moveRefused = 1

	counts, err := svc.Run(context.Background(), "mv", nil)
	if !errors.Is(err, errBackfill) {
		t.Errorf("refused move not reported: %v", err)
	}
	if counts.Move != 1 {
		t.Errorf("move count did not follow what the server took: %+v", counts)
	}
}

func TestRuleBackfillRunReportsARefusedDestroy(t *testing.T) {
	rule := execRule(t, "r1", "bad.com", execAction(t, domain.RuleDestroy, ""))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "spam@bad.com", 0)}
	actions.destroyErr = errBackfill
	actions.destroyRefused = 1

	counts, err := svc.Run(context.Background(), "r1", nil)
	if !errors.Is(err, errBackfill) {
		t.Errorf("refused destroy not reported: %v", err)
	}
	if counts.Destroy != 0 {
		t.Errorf("destroy count did not follow what the server took: %+v", counts)
	}
}

func TestRuleBackfillRunFailsWholeOnAMissingRule(t *testing.T) {
	svc, _, _, _, actions := backfillFixture(t)

	counts, err := svc.Run(context.Background(), "gone", nil)
	if !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("wrong error: %v", err)
	}
	if counts.Acts() || counts.Scanned != 0 {
		t.Errorf("a rejected run reported work: %+v", counts)
	}
	if len(actions.readIDs) != 0 || len(actions.moves) != 0 || len(actions.destroyed) != 0 {
		t.Error("a rejected run touched messages")
	}
}

// TestRuleBackfillDrivesTheRealMessageActions proves the composition rather than the fake: the same
// MessageActionService the manual move and delete go through carries the backfill's work to the server
// and the cache, so the two cannot drift apart.
func TestRuleBackfillDrivesTheRealMessageActions(t *testing.T) {
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"),
		testFolder(t, "f2", "a1", "Receipts"),
	}
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "billing@shop.com", 0)}
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	remote := &fakeMailActions{}
	rule := execRule(t, "r1", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))
	svc := NewRuleBackfillService(&fakeRuleStore{rules: []domain.Rule{rule}}, accounts, mail,
		NewMessageActionService(mail, accounts, remote))

	counts, err := svc.Run(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Move != 1 {
		t.Errorf("wrong move count: %+v", counts)
	}
	if len(remote.moveManyBatches) != 1 || len(remote.moveManyBatches[0]) != 1 {
		t.Errorf("server was not asked to move the message: %v", remote.moveManyBatches)
	}
	if len(remote.moveManyDest) != 1 || !strings.Contains(remote.moveManyDest[0], "Receipts") {
		t.Errorf("wrong destination mailbox: %v", remote.moveManyDest)
	}
}
