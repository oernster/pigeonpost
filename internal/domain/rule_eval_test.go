package domain

import "testing"

// evalRule is a rule builder for the evaluation tests: enabled, matching every message whose sender
// contains the given text, at the given position.
func evalRule(t *testing.T, id string, position int, match string, actions ...RuleAction) Rule {
	t.Helper()
	return mustRule(t, RuleSpec{
		ID: id, Name: id, Enabled: true, Position: position,
		Conditions: []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, match)},
		Actions:    actions,
	})
}

func TestEvaluateRulesFlags(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	rules := []Rule{
		evalRule(t, "r1", 0, "acme", mustAction(t, RuleMarkRead, "")),
		evalRule(t, "r2", 1, "news@", mustAction(t, RuleFlag, "")),
		evalRule(t, "r3", 2, "nomatch", mustAction(t, RuleDestroy, "")),
	}
	out := EvaluateRules([]MessageSummary{m}, rules)
	if len(out) != 1 {
		t.Fatalf("got %d outcomes, want 1", len(out))
	}
	if !out[0].Message.IsRead() || !out[0].Message.IsFlagged() {
		t.Errorf("both flag actions should have applied: %+v", out[0].Message.Flags())
	}
	if out[0].Acted() {
		t.Errorf("no side effect was asked for yet Acted() is true")
	}
}

func TestEvaluateRulesDestroyStopsEvaluation(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	rules := []Rule{
		evalRule(t, "destroy", 0, "acme", mustAction(t, RuleDestroy, "")),
		evalRule(t, "move", 1, "acme", mustAction(t, RuleMoveTo, "f2")),
	}
	out := EvaluateRules([]MessageSummary{m}, rules)
	if !out[0].Destroy {
		t.Fatalf("destroy not decided")
	}
	if out[0].MoveToFolderID != "" {
		t.Errorf("a later rule ran after a destroy: %q", out[0].MoveToFolderID)
	}
	if !out[0].Acted() {
		t.Errorf("Acted() should be true for a destroy")
	}
}

func TestEvaluateRulesFirstMoveWins(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	rules := []Rule{
		evalRule(t, "second", 1, "acme", mustAction(t, RuleMoveTo, "later")),
		evalRule(t, "first", 0, "acme", mustAction(t, RuleMoveTo, "earlier")),
	}
	out := EvaluateRules([]MessageSummary{m}, rules)
	if out[0].MoveToFolderID != "earlier" {
		t.Errorf("got %q, want the destination of the lower-positioned rule", out[0].MoveToFolderID)
	}
}

func TestEvaluateRulesStopProcessing(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	stop := mustRule(t, RuleSpec{
		ID: "stop", Name: "stop", Enabled: true, Position: 0, StopProcessing: true,
		Conditions: []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "acme")},
		Actions:    []RuleAction{mustAction(t, RuleMarkRead, "")},
	})
	later := evalRule(t, "later", 1, "acme", mustAction(t, RuleFlag, ""))
	out := EvaluateRules([]MessageSummary{m}, []Rule{stop, later})
	if !out[0].Message.IsRead() {
		t.Errorf("the stopping rule's own action did not apply")
	}
	if out[0].Message.IsFlagged() {
		t.Errorf("a rule after StopProcessing still ran")
	}
}

func TestEvaluateRulesSkipsDisabled(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	disabled := mustRule(t, RuleSpec{
		ID: "off", Name: "off", Enabled: false,
		Conditions: []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "acme")},
		Actions:    []RuleAction{mustAction(t, RuleDestroy, "")},
	})
	out := EvaluateRules([]MessageSummary{m}, []Rule{disabled})
	if out[0].Destroy {
		t.Errorf("a disabled rule destroyed a message")
	}
}

// TestEvaluateRulesDoesNotReorderCaller pins that evaluation leaves the caller's slice alone, since it
// sorts a copy into position order.
func TestEvaluateRulesDoesNotReorderCaller(t *testing.T) {
	rules := []Rule{
		evalRule(t, "b", 5, "acme", mustAction(t, RuleFlag, "")),
		evalRule(t, "a", 1, "acme", mustAction(t, RuleFlag, "")),
	}
	EvaluateRules([]MessageSummary{ruleMessage(t, "Acme", "news@acme.com", "s")}, rules)
	if rules[0].ID() != "b" || rules[1].ID() != "a" {
		t.Errorf("caller's slice was reordered: %q, %q", rules[0].ID(), rules[1].ID())
	}
}

func TestEvaluateRulesPerMessage(t *testing.T) {
	hit := ruleMessage(t, "Acme", "news@acme.com", "s")
	other, err := NewMessageSummary(MessageSummaryInput{
		ID: "m2", FolderID: "f1", UID: "2", From: ruleAddress(t, "Bob", "bob@other.com"),
		Subject: "s", Size: 1, Flags: NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	out := EvaluateRules([]MessageSummary{hit, other},
		[]Rule{evalRule(t, "r", 0, "acme", mustAction(t, RuleDestroy, ""))})
	if !out[0].Destroy || out[1].Destroy {
		t.Errorf("destroy applied to the wrong messages: %v / %v", out[0].Destroy, out[1].Destroy)
	}
}

func TestApplyRuleFlags(t *testing.T) {
	m := ruleMessage(t, "Acme", "news@acme.com", "s")
	// With no rules the input is handed straight back.
	if got := ApplyRuleFlags([]MessageSummary{m}, nil); len(got) != 1 || got[0].IsRead() {
		t.Errorf("no-rule path changed the messages")
	}
	// Side-effecting actions are ignored: only the flag lands.
	rules := []Rule{evalRule(t, "r", 0, "acme",
		mustAction(t, RuleMarkRead, ""), mustAction(t, RuleMoveTo, "f2"))}
	got := ApplyRuleFlags([]MessageSummary{m}, rules)
	if !got[0].IsRead() {
		t.Errorf("flag action did not apply")
	}
	if got[0].FolderID() != m.FolderID() {
		t.Errorf("the pure path moved a message")
	}
}
