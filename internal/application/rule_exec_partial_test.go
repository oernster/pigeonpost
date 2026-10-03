package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// partialRuleFixture runs one rule over two arrivals from bad.com (UIDs 11 and 22) against a remote that
// accepts only UID 11 before refusing the rest; it returns the ids the executor kept for saving.
func partialRuleFixture(t *testing.T, account domain.Account, remote *partialMailActions, action domain.RuleAction) []string {
	t.Helper()
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"), testFolder(t, "f2", "a1", "Receipts"), trashFolder(t, "ft", "a1"),
	}
	exec := NewRuleExecutor(mail, remote)
	fetched := []domain.MessageSummary{execMessage(t, "m1", "11", "a@bad.com"), execMessage(t, "m2", "22", "b@bad.com")}
	rules := []domain.Rule{execRule(t, "r", "bad.com", action)}

	saved, err := exec.Apply(context.Background(), account, testFolder(t, "f1", "a1", "INBOX"), fetched, primed(), true, rules)
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want the refusal to surface", err)
	}
	ids := make([]string, 0, len(saved))
	for _, m := range saved {
		ids = append(ids, m.ID())
	}
	return ids
}

// assertKeptOnlyTheRefused checks that the message the server accepted is not saved back into the folder
// it left, while the refused one is.
func assertKeptOnlyTheRefused(t *testing.T, saved []string) {
	t.Helper()
	if len(saved) != 1 || saved[0] != "m2" {
		t.Errorf("saved = %v, want [m2]: m1 (UID 11) left the folder before the refusal", saved)
	}
}

func newPartialRemote(refuseExpunge bool) *partialMailActions {
	return &partialMailActions{
		fakeMailActions: &fakeMailActions{}, landed: map[string]string{"11": "91"}, err: errBoom,
		refuseExpunge: refuseExpunge,
	}
}

func TestRuleDestroyOnGmailDropsWhatLeftBeforeARefusal(t *testing.T) {
	remote := newPartialRemote(false)
	saved := partialRuleFixture(t, gmailAccount(t, "a1"), remote, execAction(t, domain.RuleDestroy, ""))
	assertKeptOnlyTheRefused(t, saved)
}

func TestRuleDestroyDropsWhatTheServerExpungedBeforeARefusal(t *testing.T) {
	remote := newPartialRemote(true)
	saved := partialRuleFixture(t, testAccount(t, "a1"), remote, execAction(t, domain.RuleDestroy, ""))
	assertKeptOnlyTheRefused(t, saved)
}

func TestRuleMoveDropsWhatMovedBeforeARefusal(t *testing.T) {
	remote := newPartialRemote(false)
	saved := partialRuleFixture(t, testAccount(t, "a1"), remote, execAction(t, domain.RuleMoveTo, "f2"))
	assertKeptOnlyTheRefused(t, saved)
}
