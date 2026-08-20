package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// execMessage builds a message from the named sender, with an id and UID of its own.
func execMessage(t *testing.T, id, uid, sender string) domain.MessageSummary {
	t.Helper()
	from, err := domain.NewEmailAddress("", sender)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: "f1", UID: uid, From: from, Subject: "s", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

// execRule builds an enabled rule matching a sender substring, with the given actions.
func execRule(t *testing.T, id, match string, actions ...domain.RuleAction) domain.Rule {
	t.Helper()
	cond, err := domain.NewRuleCondition(domain.RuleFieldFrom, domain.RuleOpContains, match)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	rule, err := domain.NewRule(domain.RuleSpec{
		ID: id, Name: id, Enabled: true,
		Conditions: []domain.RuleCondition{cond}, Actions: actions,
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return rule
}

// execScopedRule is execRule limited to the named accounts.
func execScopedRule(t *testing.T, id, match string, accountIDs []string, actions ...domain.RuleAction) domain.Rule {
	t.Helper()
	cond, err := domain.NewRuleCondition(domain.RuleFieldFrom, domain.RuleOpContains, match)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	rule, err := domain.NewRule(domain.RuleSpec{
		ID: id, Name: id, Enabled: true, AccountIDs: accountIDs,
		Conditions: []domain.RuleCondition{cond}, Actions: actions,
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return rule
}

// execAction builds a rule action for the executor tests.
func execAction(t *testing.T, kind domain.RuleActionKind, folderID string) domain.RuleAction {
	t.Helper()
	a, err := domain.NewRuleAction(kind, folderID)
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	return a
}

// execFixture returns an executor over a store holding an inbox f1 and a destination folder f2, plus
// the recording remote it drives.
func execFixture(t *testing.T) (*RuleExecutor, *fakeMailStore, *fakeMailActions) {
	t.Helper()
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"),
		testFolder(t, "f2", "a1", "Receipts"),
	}
	remote := &fakeMailActions{}
	return NewRuleExecutor(mail, remote), mail, remote
}

// primed is the known-id set standing for a folder the sync has seen before, so destructive actions
// are not held back as they are on a folder's first sight.
func primed(ids ...string) map[string]struct{} {
	known := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		known[id] = struct{}{}
	}
	return known
}

func TestRuleExecutorDestroys(t *testing.T) {
	exec, _, remote := execFixture(t)
	junk := execMessage(t, "m2", "22", "spam@bad.com")
	fetched := []domain.MessageSummary{execMessage(t, "m1", "11", "friend@good.com"), junk}
	rules := []domain.Rule{execRule(t, "nuke", "bad.com", execAction(t, domain.RuleDestroy, ""))}

	saved, err := exec.Apply(context.Background(), testAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		fetched, primed("m1"), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 1 || saved[0].ID() != "m1" {
		t.Fatalf("destroyed message still saved: %v", saved)
	}
	if len(remote.deleteManyBatches) != 1 || len(remote.deleteManyBatches[0]) != 1 ||
		remote.deleteManyBatches[0][0] != "22" {
		t.Errorf("wrong destroy batch: %v", remote.deleteManyBatches)
	}
	// The empty trash path is what makes the deletion permanent rather than a move to Trash.
	if len(remote.deleteManyTrash) != 1 || remote.deleteManyTrash[0] != "" {
		t.Errorf("destroy used a trash path: %q", remote.deleteManyTrash)
	}
}

func TestRuleExecutorMoves(t *testing.T) {
	exec, _, remote := execFixture(t)
	fetched := []domain.MessageSummary{execMessage(t, "m2", "22", "billing@shop.com")}
	rules := []domain.Rule{execRule(t, "file", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))}

	saved, err := exec.Apply(context.Background(), testAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		fetched, primed("m0"), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 0 {
		t.Errorf("moved message left in the source folder: %v", saved)
	}
	if len(remote.moveManyDest) != 1 || remote.moveManyDest[0] != "Receipts" {
		t.Errorf("wrong destination: %v", remote.moveManyDest)
	}
}

// TestRuleExecutorArrivalsOnly is the guard that adding a rule never reaches back over mail already in
// the mailbox: a message the store already holds is not evaluated at all.
func TestRuleExecutorArrivalsOnly(t *testing.T) {
	exec, _, remote := execFixture(t)
	old := execMessage(t, "m1", "11", "spam@bad.com")
	rules := []domain.Rule{execRule(t, "nuke", "bad.com", execAction(t, domain.RuleDestroy, ""))}

	saved, err := exec.Apply(context.Background(), testAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		[]domain.MessageSummary{old}, primed("m1"), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 1 {
		t.Errorf("an already-known message was acted on: %v", saved)
	}
	if len(remote.deleteManyBatches) != 0 {
		t.Errorf("the server was asked to delete a known message: %v", remote.deleteManyBatches)
	}
}

// TestRuleExecutorFirstSightIsBaseline is the guard that a newly added account is not emptied: with
// nothing known locally every message looks like an arrival, so destructive actions are held back and
// only flags are applied.
func TestRuleExecutorFirstSightIsBaseline(t *testing.T) {
	exec, _, remote := execFixture(t)
	fetched := []domain.MessageSummary{execMessage(t, "m1", "11", "spam@bad.com")}
	rules := []domain.Rule{execRule(t, "nuke", "bad.com",
		execAction(t, domain.RuleMarkRead, ""), execAction(t, domain.RuleDestroy, ""))}

	saved, err := exec.Apply(context.Background(), testAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		fetched, map[string]struct{}{}, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("a first-sight message was destroyed: %v", saved)
	}
	if !saved[0].IsRead() {
		t.Errorf("flag actions should still apply on first sight")
	}
	if len(remote.deleteManyBatches) != 0 {
		t.Errorf("the server was asked to delete on first sight: %v", remote.deleteManyBatches)
	}
}

func TestRuleExecutorAppliesFlagsToArrivalsOnly(t *testing.T) {
	exec, _, _ := execFixture(t)
	known := execMessage(t, "m1", "11", "news@acme.com")
	arrival := execMessage(t, "m2", "22", "news@acme.com")
	rules := []domain.Rule{execRule(t, "read", "acme.com", execAction(t, domain.RuleMarkRead, ""))}

	saved, err := exec.Apply(context.Background(), testAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		[]domain.MessageSummary{known, arrival}, primed("m1"), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 2 {
		t.Fatalf("got %d messages, want 2", len(saved))
	}
	if saved[0].IsRead() {
		t.Errorf("a known message was marked read by a rule")
	}
	if !saved[1].IsRead() {
		t.Errorf("the arrival was not marked read")
	}
}

func TestRuleExecutorSkipsUnactionableMoves(t *testing.T) {
	inbox := testFolder(t, "f1", "a1", "INBOX")
	cases := map[string]struct {
		dest    string
		account string
	}{
		"already in the destination":     {"f1", "a1"},
		"destination in another account": {"other", "a1"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			exec, mail, remote := execFixture(t)
			mail.folders["a2"] = []domain.Folder{testFolder(t, "other", "a2", "Elsewhere")}
			rules := []domain.Rule{execRule(t, "file", "shop.com", execAction(t, domain.RuleMoveTo, c.dest))}
			saved, err := exec.Apply(context.Background(), testAccount(t, c.account), inbox,
				[]domain.MessageSummary{execMessage(t, "m2", "22", "billing@shop.com")}, primed("m0"), rules)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(saved) != 1 {
				t.Errorf("message was moved when it should have been left alone: %v", saved)
			}
			if len(remote.moveManyBatches) != 0 {
				t.Errorf("the server was asked to move: %v", remote.moveManyBatches)
			}
		})
	}
}

// TestRuleExecutorFailedBatchKeepsMessages pins that a batch the server refuses leaves its messages in
// the local cache and reports the failure, rather than dropping mail that still exists on the server.
func TestRuleExecutorFailedBatchKeepsMessages(t *testing.T) {
	cases := map[string]struct {
		inject func(*fakeMailActions)
		action domain.RuleActionKind
		dest   string
	}{
		"destroy fails": {func(f *fakeMailActions) { f.deleteManyErr = errBoom }, domain.RuleDestroy, ""},
		"move fails":    {func(f *fakeMailActions) { f.moveManyErr = errBoom }, domain.RuleMoveTo, "f2"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			exec, _, remote := execFixture(t)
			c.inject(remote)
			rules := []domain.Rule{execRule(t, "r", "bad.com", execAction(t, c.action, c.dest))}
			saved, err := exec.Apply(context.Background(), testAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
				[]domain.MessageSummary{execMessage(t, "m2", "22", "spam@bad.com")}, primed("m0"), rules)
			if !errors.Is(err, errBoom) {
				t.Errorf("error = %v, want wrapped boom", err)
			}
			if len(saved) != 1 {
				t.Errorf("message dropped despite the server refusing: %v", saved)
			}
		})
	}
}

func TestRuleExecutorUnresolvableDestination(t *testing.T) {
	exec, mail, _ := execFixture(t)
	mail.getFolderErr = errBoom
	rules := []domain.Rule{execRule(t, "file", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))}
	saved, err := exec.Apply(context.Background(), testAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		[]domain.MessageSummary{execMessage(t, "m2", "22", "billing@shop.com")}, primed("m0"), rules)
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(saved) != 1 {
		t.Errorf("message dropped despite an unresolvable destination: %v", saved)
	}
}

func TestRuleExecutorNoWork(t *testing.T) {
	exec, _, remote := execFixture(t)
	inbox := testFolder(t, "f1", "a1", "INBOX")
	account := testAccount(t, "a1")
	msg := []domain.MessageSummary{execMessage(t, "m1", "11", "a@b.com")}
	rules := []domain.Rule{execRule(t, "r", "b.com", execAction(t, domain.RuleDestroy, ""))}
	cases := map[string]struct {
		messages []domain.MessageSummary
		known    map[string]struct{}
		rules    []domain.Rule
	}{
		"no rules":    {msg, primed("m0"), nil},
		"no messages": {nil, primed("m0"), rules},
		"no arrivals": {msg, primed("m1"), rules},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := exec.Apply(context.Background(), account, inbox, c.messages, c.known, c.rules); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	if len(remote.deleteManyBatches) != 0 {
		t.Errorf("the server was touched with nothing to do: %v", remote.deleteManyBatches)
	}
}

// TestRuleExecutorHonoursAccountScope is the guard that a rule limited to one address cannot reach mail
// arriving on another: the same junk message is destroyed on the account the rule names and left alone
// on the one it does not.
func TestRuleExecutorHonoursAccountScope(t *testing.T) {
	inbox := testFolder(t, "f1", "a1", "INBOX")
	rules := []domain.Rule{
		execScopedRule(t, "nuke", "bad.com", []string{"a1"}, execAction(t, domain.RuleDestroy, "")),
	}
	cases := map[string]struct {
		account   string
		destroyed bool
	}{
		"account the rule names":         {"a1", true},
		"account the rule does not name": {"a2", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			exec, _, remote := execFixture(t)
			saved, err := exec.Apply(context.Background(), testAccount(t, c.account), inbox,
				[]domain.MessageSummary{execMessage(t, "m2", "22", "spam@bad.com")}, primed("m0"), rules)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := len(remote.deleteManyBatches) > 0; got != c.destroyed {
				t.Errorf("destroyed = %v, want %v", got, c.destroyed)
			}
			if want := 0; c.destroyed && len(saved) != want {
				t.Errorf("saved %d messages, want %d", len(saved), want)
			}
			if want := 1; !c.destroyed && len(saved) != want {
				t.Errorf("saved %d messages, want %d", len(saved), want)
			}
		})
	}
}

// TestRuleExecutorUnscopedRuleCoversEveryAccount pins the other half: a rule naming no account still
// runs on an account it was never told about.
func TestRuleExecutorUnscopedRuleCoversEveryAccount(t *testing.T) {
	exec, _, remote := execFixture(t)
	rules := []domain.Rule{execRule(t, "nuke", "bad.com", execAction(t, domain.RuleDestroy, ""))}
	if _, err := exec.Apply(context.Background(), testAccount(t, "a9"), testFolder(t, "f1", "a9", "INBOX"),
		[]domain.MessageSummary{execMessage(t, "m2", "22", "spam@bad.com")}, primed("m0"), rules); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.deleteManyBatches) != 1 {
		t.Errorf("an unscoped rule did not run on an unnamed account: %v", remote.deleteManyBatches)
	}
}
