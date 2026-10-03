package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The defect these exist for (audit P-11): Run re-planned on confirm, so the work it did was not the
// work the user had agreed to. Measured: a preview said one message would be destroyed; a second
// matching message arrived before the confirm; the run destroyed two. A run now acts on the previewed
// set alone and skips any of it that no longer wants the same action.

func TestRuleBackfillRunActsOnlyOnThePreviewedMessages(t *testing.T) {
	rule := execRule(t, "r1", "bad.com", execAction(t, domain.RuleDestroy, ""))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "spam@bad.com", 0)}

	preview, err := svc.Preview(context.Background(), "r1", nil)
	if err != nil || preview.Destroy != 1 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	// Mail arrives between the preview and the confirm. The user never agreed to destroy it.
	mail.messages["f1"] = append(mail.messages["f1"], backfillMessage(t, "m2", "f1", "junk@bad.com", 0))

	counts, err := svc.Run(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts.Destroy != 1 || len(actions.destroyed) != 1 || actions.destroyed[0] != "m1" {
		t.Errorf("run went beyond the previewed plan: %+v, destroyed %v", counts, actions.destroyed)
	}
}

func TestRuleBackfillRunSkipsPreviewedMessagesThatChangedOrWent(t *testing.T) {
	rule := execRule(t, "r1", "news@",
		execAction(t, domain.RuleMarkRead, ""), execAction(t, domain.RuleFlag, ""),
		execAction(t, domain.RuleMoveTo, "f2"))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{
		backfillMessage(t, "m1", "f1", "news@site.com", 0),
		backfillMessage(t, "m2", "f1", "news@site.com", 0),
		backfillMessage(t, "m3", "f1", "news@site.com", 0),
	}
	if _, err := svc.Preview(context.Background(), "r1", nil); err != nil {
		t.Fatalf("preview: %v", err)
	}
	// Before the confirm: m1 is deleted elsewhere; m2 is read, flagged and filed by hand. Only m3 is
	// still in the state the preview described.
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m3", "f1", "news@site.com", 0)}
	mail.messages["f2"] = []domain.MessageSummary{
		backfillMessage(t, "m2", "f2", "news@site.com", domain.FlagSeen|domain.FlagFlagged),
	}

	counts, err := svc.Run(context.Background(), "r1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The report counts what was done, so the skipped messages are simply absent from it.
	if counts.MarkRead != 1 || counts.Flag != 1 || counts.Move != 1 || counts.Scanned != 2 {
		t.Errorf("wrong counts: %+v", counts)
	}
	if len(actions.readIDs) != 1 || actions.readIDs[0] != "m3" {
		t.Errorf("marked messages the preview no longer describes: %v", actions.readIDs)
	}
	if len(actions.flaggedIDs) != 1 || actions.flaggedIDs[0] != "m3" {
		t.Errorf("flagged messages the preview no longer describes: %v", actions.flaggedIDs)
	}
	if len(actions.moves) != 1 || len(actions.moves[0].ids) != 1 || actions.moves[0].ids[0] != "m3" {
		t.Errorf("moved messages the preview no longer describes: %+v", actions.moves)
	}
}

func TestRuleBackfillRunRefusesWithoutAPreview(t *testing.T) {
	rule := execRule(t, "r1", "bad.com", execAction(t, domain.RuleDestroy, ""))
	svc, mail, _, _, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "spam@bad.com", 0)}

	counts, err := svc.Run(context.Background(), "r1", nil)
	if !errors.Is(err, ErrRuleBackfillNotPreviewed) {
		t.Errorf("wrong error: %v", err)
	}
	if counts.Acts() || len(actions.destroyed) != 0 {
		t.Errorf("an unpreviewed run acted: %+v, destroyed %v", counts, actions.destroyed)
	}
}

func TestRuleBackfillRunSpendsItsPreview(t *testing.T) {
	rule := execRule(t, "r1", "bad.com", execAction(t, domain.RuleDestroy, ""))
	svc, mail, _, _, _ := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "spam@bad.com", 0)}

	if _, err := previewThenRun(t, svc, context.Background(), "r1", nil); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := svc.Run(context.Background(), "r1", nil); !errors.Is(err, ErrRuleBackfillNotPreviewed) {
		t.Errorf("a second run reused the first one's preview: %v", err)
	}
}

// A preview that fails outright must not leave an earlier preview of the same rule waiting: the user
// last saw an error, not those counts.
func TestRuleBackfillFailedPreviewDropsTheEarlierOne(t *testing.T) {
	rule := execRule(t, "r1", "bad.com", execAction(t, domain.RuleDestroy, ""))
	svc, mail, _, rules, actions := backfillFixture(t, rule)
	mail.messages["f1"] = []domain.MessageSummary{backfillMessage(t, "m1", "f1", "spam@bad.com", 0)}

	if _, err := svc.Preview(context.Background(), "r1", nil); err != nil {
		t.Fatalf("preview: %v", err)
	}
	rules.listErr = errBackfill
	if _, err := svc.Preview(context.Background(), "r1", nil); !errors.Is(err, errBackfill) {
		t.Fatalf("second preview should fail: %v", err)
	}
	rules.listErr = nil

	if _, err := svc.Run(context.Background(), "r1", nil); !errors.Is(err, ErrRuleBackfillNotPreviewed) {
		t.Errorf("ran on a preview older than the failure: %v", err)
	}
	if len(actions.destroyed) != 0 {
		t.Errorf("destroyed after a failed preview: %v", actions.destroyed)
	}
}

// previewThenRun is the Now button's own sequence, a preview the user confirms and then the run. The
// preview's errors are asserted by the preview tests; a planning failure also reaches the run's own
// error, which is what these callers check.
func previewThenRun(t *testing.T, svc *RuleBackfillService, ctx context.Context, ruleID string,
	to RuleBackfillReport) (RuleBackfillCounts, error) {
	t.Helper()
	_, _ = svc.Preview(context.Background(), ruleID, nil)
	return svc.Run(ctx, ruleID, to)
}
