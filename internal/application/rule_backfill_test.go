package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeBackfillActions records what a backfill asked for and can be made to refuse any of it, so the
// partial-failure paths are exercised rather than assumed.
type fakeBackfillActions struct {
	readIDs    []string
	flaggedIDs []string
	moves      []backfillMoveCall
	destroyed  []string
	permanent  []bool
	readErr    error
	flagErr    error
	// onRead and onMove fire inside the call, so a test can cancel part way through a run rather than
	// only before it starts.
	onRead func()
	onMove func()
	// moveCtxErr records what each move's own context reported AFTER the cancel fired inside it, which
	// is how the cache-write failure was actually observed.
	moveCtxErr []error
	moveErr    error
	destroyErr error
	// moveRefused and destroyRefused are the ids a failing call leaves behind, so a test can model a
	// server that took some of a batch and refused the rest.
	moveRefused    int
	destroyRefused int
}

// backfillMoveCall is one MoveMany the backfill issued.
type backfillMoveCall struct {
	ids  []string
	dest string
}

func (f *fakeBackfillActions) MarkRead(_ context.Context, messageID string, _ bool) error {
	if f.onRead != nil {
		f.onRead()
	}
	if f.readErr != nil {
		return f.readErr
	}
	f.readIDs = append(f.readIDs, messageID)
	return nil
}

func (f *fakeBackfillActions) MarkFlagged(_ context.Context, messageID string, _ bool) error {
	if f.flagErr != nil {
		return f.flagErr
	}
	f.flaggedIDs = append(f.flaggedIDs, messageID)
	return nil
}

func (f *fakeBackfillActions) MoveMany(ctx context.Context, messageIDs []string, destFolderID string) (
	[]string, map[string]string, error) {
	f.moves = append(f.moves, backfillMoveCall{ids: messageIDs, dest: destFolderID})
	if f.onMove != nil {
		f.onMove()
	}
	// Read after the hook, standing in for the cache writes MessageActionService.MoveMany makes once the
	// server has agreed: those are what failed when the run's own context was cancellable.
	f.moveCtxErr = append(f.moveCtxErr, ctx.Err())
	if f.moveErr != nil {
		return messageIDs[:len(messageIDs)-f.moveRefused], nil, f.moveErr
	}
	return messageIDs, nil, nil
}

func (f *fakeBackfillActions) DeleteMany(_ context.Context, messageIDs []string, permanent bool) (
	[]string, map[string]string, error) {
	f.destroyed = append(f.destroyed, messageIDs...)
	f.permanent = append(f.permanent, permanent)
	if f.destroyErr != nil {
		return messageIDs[:len(messageIDs)-f.destroyRefused], nil, f.destroyErr
	}
	return messageIDs, nil, nil
}

// backfillMessage builds a stored message in a named folder, from a named sender, with the given flags.
func backfillMessage(t *testing.T, id, folderID, sender string, flags domain.Flag) domain.MessageSummary {
	t.Helper()
	from, err := domain.NewEmailAddress("", sender)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: id, From: from, Subject: "s", Size: 1,
		Flags: domain.NewFlags(flags),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

// backfillFixture returns a service over one account holding an Inbox (f1) and an Archive (f2), with
// the given rules stored. The stores are returned so a test can add mail, folders and failures.
func backfillFixture(t *testing.T, rules ...domain.Rule) (
	*RuleBackfillService, *fakeMailStore, *fakeAccountStore, *fakeRuleStore, *fakeBackfillActions) {
	t.Helper()
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"),
		testFolder(t, "f2", "a1", "Archive"),
	}
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	ruleStore := &fakeRuleStore{rules: rules}
	actions := &fakeBackfillActions{}
	return NewRuleBackfillService(ruleStore, accounts, mail, actions), mail, accounts, ruleStore, actions
}

func TestRuleBackfillPreviewCountsWithoutActing(t *testing.T) {
	rule := execRule(t, "r1", "news@", execAction(t, domain.RuleMarkRead, ""))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "news@site.com", 0),
		backfillMessage(t, "m2", "f1", "friend@good.com", 0),
	}
	mail.messages["f2"] = []domain.MessageSummary{backfillMessage(t, "m3", "f2", "news@site.com", 0)}

	counts, err := svc.Preview(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both folders are scanned: a backfill is an explicit instruction about mail already filed, so it is
	// not narrowed to the Inbox the way the sync's arrival path is.
	if counts.Scanned != 3 || counts.MarkRead != 2 {
		t.Errorf("wrong counts: %+v", counts)
	}
	if !counts.Acts() {
		t.Error("counts with work in them report no work")
	}
	if len(actions.readIDs) != 0 {
		t.Errorf("preview acted on messages: %v", actions.readIDs)
	}
}

func TestRuleBackfillSkipsMessagesAlreadyInTheWantedState(t *testing.T) {
	rule := execRule(t, "r1", "news@",
		execAction(t, domain.RuleMarkRead, ""), execAction(t, domain.RuleFlag, ""))
	svc, mail, _, _, _ := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "news@site.com", domain.FlagSeen|domain.FlagFlagged),
		backfillMessage(t, "m2", "f1", "news@site.com", 0),
	}

	counts, err := svc.Preview(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.MarkRead != 1 || counts.Flag != 1 {
		t.Errorf("already-satisfied message counted as work: %+v", counts)
	}
	if counts.Acts() != true {
		t.Error("expected work")
	}
}

func TestRuleBackfillCountsNothingWhenNoMessageMatches(t *testing.T) {
	rule := execRule(t, "r1", "news@", execAction(t, domain.RuleMarkRead, ""))
	svc, mail, _, _, _ := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "friend@good.com", 0)}

	counts, err := svc.Preview(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Acts() {
		t.Errorf("expected no work: %+v", counts)
	}
}

func TestRuleBackfillMovesGroupedByDestination(t *testing.T) {
	rule := execRule(t, "r1", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "billing@shop.com", 0),
		backfillMessage(t, "m2", "f1", "orders@shop.com", 0),
	}
	// A message already in the destination has nowhere to go, so it is not work.
	mail.messages["f2"] = []domain.MessageSummary{backfillMessage(t, "m3", "f2", "billing@shop.com", 0)}

	counts, err := svc.Run(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Move != 2 {
		t.Errorf("wrong move count: %+v", counts)
	}
	if len(actions.moves) != 1 || actions.moves[0].dest != "f2" || len(actions.moves[0].ids) != 2 {
		t.Errorf("moves not batched by destination: %+v", actions.moves)
	}
}

func TestRuleBackfillDestroysPermanentlyAndSkipsItsFlagActions(t *testing.T) {
	rule := execRule(t, "r1", "bad.com",
		execAction(t, domain.RuleMarkRead, ""), execAction(t, domain.RuleDestroy, ""))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "spam@bad.com", 0)}

	counts, err := svc.Run(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Destroy != 1 || counts.MarkRead != 0 {
		t.Errorf("destroy did not take the message whole: %+v", counts)
	}
	// Permanent, with no Trash hop: a destroying rule is documented as irreversible and a backfill must
	// not quietly soften it into something recoverable.
	if len(actions.permanent) != 1 || !actions.permanent[0] {
		t.Errorf("destroy was not permanent: %v", actions.permanent)
	}
	if len(actions.destroyed) != 1 || actions.destroyed[0] != "m1" {
		t.Errorf("wrong destroy set: %v", actions.destroyed)
	}
}

func TestRuleBackfillRunsOnlyTheAccountsTheRuleCovers(t *testing.T) {
	rule := execScopedRule(t, "r1", "news@", []string{"a2"}, execAction(t, domain.RuleMarkRead, ""))
	svc, mail, accounts, _, _ := backfillFixture(t, rule)
	accounts.accounts["a2"] = testAccount(t, "a2")
	mail.folders["a2"] = []domain.Folder{testFolder(t, "f9", "a2", "INBOX")}
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "news@site.com", 0)}
	mail.messages["f9"] = []domain.MessageSummary{backfillMessage(t, "m9", "f9", "news@site.com", 0)}

	counts, err := svc.Preview(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Scanned != 1 || counts.MarkRead != 1 {
		t.Errorf("scoped rule reached another account: %+v", counts)
	}
}

func TestRuleBackfillSkipsMoveToAnotherAccount(t *testing.T) {
	rule := execRule(t, "r1", "news@", execAction(t, domain.RuleMoveTo, "f9"))
	svc, mail, accounts, _, actions := backfillFixture(t, rule)
	accounts.accounts["a2"] = testAccount(t, "a2")
	mail.folders["a2"] = []domain.Folder{testFolder(t, "f9", "a2", "INBOX")}
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "news@site.com", 0)}

	counts, err := svc.Run(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Move != 0 || len(actions.moves) != 0 {
		t.Errorf("moved across an account boundary: %+v", actions.moves)
	}
}

func TestRuleBackfillRejectsMissingAndDisabledRules(t *testing.T) {
	disabled, err := domain.NewRule(domain.RuleSpec{
		ID: "off", Name: "off", Enabled: false,
		Conditions: []domain.RuleCondition{ruleCondition(t, "news@")},
		Actions:    []domain.RuleAction{execAction(t, domain.RuleMarkRead, "")},
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	svc, _, _, _, _ := backfillFixture(t, disabled)

	if _, err := svc.Preview(context.Background(), "missing", nil); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("wrong error for a missing rule: %v", err)
	}
	if _, err := svc.Run(context.Background(), "off", nil); !errors.Is(err, ErrRuleDisabled) {
		t.Errorf("wrong error for a disabled rule: %v", err)
	}
}

func TestRuleBackfillReportsProgressThroughBothPhases(t *testing.T) {
	// Two folders to scan and three messages to act on, so each phase has a total a bar can divide by.
	rule := execRule(t, "r1", "shop.com",
		execAction(t, domain.RuleMarkRead, ""), execAction(t, domain.RuleMoveTo, "f2"))
	svc, mail, _, _, _ := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "billing@shop.com", 0),
		backfillMessage(t, "m2", "f1", "orders@shop.com", 0),
	}

	var seen []RuleBackfillProgress
	if _, err := svc.Run(context.Background(), "r1", func(p RuleBackfillProgress) {
		seen = append(seen, p)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The scan counts folders and knows its total from the first reading, before any folder is read.
	if seen[0] != (RuleBackfillProgress{Phase: RuleBackfillScanning, Done: 0, Total: 2}) {
		t.Errorf("scan did not open with its total: %+v", seen[0])
	}
	// Two marks plus two moves is four attempts; the run ends filled rather than short.
	last := seen[len(seen)-1]
	if last != (RuleBackfillProgress{Phase: RuleBackfillFinished, Done: 4, Total: 4}) {
		t.Errorf("run did not finish filled: %+v", last)
	}
	assertMonotonic(t, seen, RuleBackfillScanning)
	assertMonotonic(t, seen, RuleBackfillApplying)
}

// assertMonotonic checks one phase's readings never go backwards and never exceed their total, the two
// ways a bar can visibly lie about where a run has got to.
func assertMonotonic(t *testing.T, seen []RuleBackfillProgress, phase string) {
	t.Helper()
	previous, found := -1, false
	for _, p := range seen {
		if p.Phase != phase {
			continue
		}
		found = true
		if p.Done < previous {
			t.Errorf("%s went backwards: %d after %d", phase, p.Done, previous)
		}
		if p.Done > p.Total {
			t.Errorf("%s ran past its total: %+v", phase, p)
		}
		previous = p.Done
	}
	if !found {
		t.Errorf("no %s readings at all", phase)
	}
}

func TestRuleBackfillReportsAnEmptyApplyingPhaseRatherThanNothing(t *testing.T) {
	// A rule that matches nothing still opens and closes the applying phase, so the dialog is never left
	// showing a scan that finished with no word of what followed.
	rule := execRule(t, "r1", "news@", execAction(t, domain.RuleMarkRead, ""))
	svc, mail, _, _, _ := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "friend@good.com", 0)}

	var seen []RuleBackfillProgress
	if _, err := svc.Run(context.Background(), "r1", func(p RuleBackfillProgress) {
		seen = append(seen, p)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen[len(seen)-1] != (RuleBackfillProgress{Phase: RuleBackfillFinished, Done: 0, Total: 0}) {
		t.Errorf("empty run did not finish: %+v", seen[len(seen)-1])
	}
}

func TestRuleBackfillSendsBigMovesInBatchesSoTheBarMoves(t *testing.T) {
	// The defect this exists for: a rule that filed 2414 messages issued ONE move and reported once, so
	// the bar sat at its opening reading for the whole operation and read as a hang.
	const messages = actionBatchSize*2 + 5
	rule := execRule(t, "r1", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	stored := make([]domain.MessageSummary, 0, messages)
	for i := 0; i < messages; i++ {
		stored = append(stored, backfillMessage(t, fmt.Sprintf("m%d", i), "f1", "billing@shop.com", 0))
	}
	mail.messages["f1"] = stored

	var applying []RuleBackfillProgress
	counts, err := svc.Run(context.Background(), "r1", func(p RuleBackfillProgress) {
		if p.Phase == RuleBackfillApplying {
			applying = append(applying, p)
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Move != messages {
		t.Errorf("wrong move count: %+v", counts)
	}
	// Three batches: two full and the remainder, each its own server call.
	if len(actions.moves) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(actions.moves))
	}
	if len(actions.moves[0].ids) != actionBatchSize || len(actions.moves[2].ids) != 5 {
		t.Errorf("wrong batch sizes: %d, %d", len(actions.moves[0].ids), len(actions.moves[2].ids))
	}
	// The opening reading plus one per batch: the bar moves during the run rather than only at its end.
	if len(applying) != 4 {
		t.Errorf("expected 4 applying readings, got %d: %+v", len(applying), applying)
	}
}

func TestRuleBackfillStopsWhenCancelledAndReportsWhatLanded(t *testing.T) {
	const messages = actionBatchSize * 3
	rule := execRule(t, "r1", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	stored := make([]domain.MessageSummary, 0, messages)
	for i := 0; i < messages; i++ {
		stored = append(stored, backfillMessage(t, fmt.Sprintf("m%d", i), "f1", "billing@shop.com", 0))
	}
	mail.messages["f1"] = stored

	// Cancelled from inside the first batch, so the run stops between batches rather than at the end.
	ctx, cancel := context.WithCancel(context.Background())
	actions.onMove = func() { cancel() }

	counts, err := svc.Run(ctx, "r1", nil)
	if err != nil {
		t.Fatalf("a cancel is not an error: %v", err)
	}
	if !counts.Cancelled {
		t.Error("a cancelled run did not say so")
	}
	// One batch was issued and seen through; the rest never went.
	if len(actions.moves) != 1 {
		t.Errorf("expected 1 batch before the stop, got %d", len(actions.moves))
	}
	// The work that landed is reported rather than hidden: those messages really did move.
	if counts.Move != actionBatchSize {
		t.Errorf("cancelled run misreported what landed: %+v", counts)
	}
}

func TestRuleBackfillStopsScanningWhenCancelled(t *testing.T) {
	rule := execRule(t, "r1", "news@", execAction(t, domain.RuleMarkRead, ""))
	svc, mail, _, _, _ := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "news@site.com", 0)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	counts, err := svc.Preview(ctx, "r1", nil)
	if err != nil {
		t.Fatalf("a cancel is not an error: %v", err)
	}
	if !counts.Cancelled || counts.Scanned != 0 {
		t.Errorf("scan did not stop on cancel: %+v", counts)
	}
}

func TestRuleBackfillStopsFlagsAndDestroysWhenCancelled(t *testing.T) {
	rule := execRule(t, "r1", "bad.com", execAction(t, domain.RuleMarkRead, ""))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "spam@bad.com", 0),
		backfillMessage(t, "m2", "f1", "junk@bad.com", 0),
	}
	ctx, cancel := context.WithCancel(context.Background())
	actions.onRead = func() { cancel() }

	counts, err := svc.Run(ctx, "r1", nil)
	if err != nil {
		t.Fatalf("a cancel is not an error: %v", err)
	}
	if counts.MarkRead != 1 || !counts.Cancelled {
		t.Errorf("flags did not stop on cancel: %+v", counts)
	}

	// A destroy set gathered by the scan and then stopped before it runs: the mail must still be there.
	// The cancel is driven from the reporter, which fires once per folder scanned, so the first folder's
	// matches are planned and the second is never read.
	destroy := execRule(t, "d1", "bad.com", execAction(t, domain.RuleDestroy, ""))
	svc2, mail2, _, _, actions2 := backfillFixture(t, destroy)
	mail2.messages["f1"] = mail.messages["f1"]
	ctx2, cancel2 := context.WithCancel(context.Background())
	counts2, err := svc2.Run(ctx2, "d1", func(p RuleBackfillProgress) {
		if p.Phase == RuleBackfillScanning && p.Done == 1 {
			cancel2()
		}
	})
	if err != nil {
		t.Fatalf("a cancel is not an error: %v", err)
	}
	if counts2.Destroy != 0 || len(actions2.destroyed) != 0 {
		t.Errorf("destroy ran after a cancel: %+v", counts2)
	}
	if !counts2.Cancelled {
		t.Error("a cancelled destroy run did not say so")
	}
}

func TestRuleBackfillCancelDoesNotAbortTheCallInFlight(t *testing.T) {
	// The defect: cancelling passed straight down into MessageActionService.MoveMany, landing AFTER the
	// server had moved 200 messages and BEFORE their rows were dropped from the cache. Every cache delete
	// then failed with "context canceled", so the mail had moved on the server while the cache still
	// listed it where it was; the user got two hundred joined errors.
	rule := execRule(t, "r1", "shop.com", execAction(t, domain.RuleMoveTo, "f2"))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "billing@shop.com", 0),
		backfillMessage(t, "m2", "f1", "orders@shop.com", 0),
	}
	ctx, cancel := context.WithCancel(context.Background())
	actions.onMove = func() { cancel() }

	counts, err := svc.Run(ctx, "r1", nil)
	if err != nil {
		t.Fatalf("a cancel must not surface as an error: %v", err)
	}
	// The batch's own context is still live after the cancel, so the work it does afterwards succeeds.
	if len(actions.moveCtxErr) != 1 || actions.moveCtxErr[0] != nil {
		t.Errorf("the call in flight saw the cancel: %v", actions.moveCtxErr)
	}
	if counts.Move != 2 || !counts.Cancelled {
		t.Errorf("wrong outcome: %+v", counts)
	}
}

// ruleCondition builds a from-contains condition for the backfill tests.
func ruleCondition(t *testing.T, match string) domain.RuleCondition {
	t.Helper()
	cond, err := domain.NewRuleCondition(domain.RuleFieldFrom, domain.RuleOpContains, match)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	return cond
}
