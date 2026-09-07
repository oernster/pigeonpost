package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// wireTestRule assembles a rule for the wire tests.
func wireTestRule(t *testing.T, accountIDs []string) domain.Rule {
	t.Helper()
	cond, err := domain.NewRuleCondition(domain.RuleFieldAll, domain.RuleOpContains, "x")
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	action, err := domain.NewRuleAction(domain.RuleMarkRead, "")
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	rule, err := domain.NewRule(domain.RuleSpec{
		ID: "r1", Name: "Rule", Enabled: true, AccountIDs: accountIDs,
		Conditions: []domain.RuleCondition{cond}, Actions: []domain.RuleAction{action},
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return rule
}

// TestRuleDTOSendsArraysNotNull guards the defect that made the rules dialog blank the whole window: a
// nil Go slice encodes as JSON null while the front end's type says string[], so reading the length
// off null threw during render. React has no error boundary above the app, so one unscoped rule took
// PigeonPost down entirely rather than breaking one dialog. Every list on this DTO must therefore
// reach the wire as an array, empty or not; the check is on the encoded bytes because that is
// what the front end actually receives.
func TestRuleDTOSendsArraysNotNull(t *testing.T) {
	encoded, err := json.Marshal(ruleToDTO(wireTestRule(t, nil)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(encoded)
	for _, field := range []string{"accountIds", "conditions", "actions"} {
		if strings.Contains(body, `"`+field+`":null`) {
			t.Errorf("%s reached the wire as null, which the front end reads as a crash: %s", field, body)
		}
	}
	if !strings.Contains(body, `"accountIds":[]`) {
		t.Errorf("an unscoped rule must send an empty array: %s", body)
	}
}

// TestRuleDTOKeepsTheScope pins that a scoped rule still sends the accounts it names.
func TestRuleDTOKeepsTheScope(t *testing.T) {
	dto := ruleToDTO(wireTestRule(t, []string{"a1", "a2"}))
	if len(dto.AccountIDs) != 2 || dto.AccountIDs[0] != "a1" || dto.AccountIDs[1] != "a2" {
		t.Errorf("scope lost on the way to the wire: %v", dto.AccountIDs)
	}
}

// TestRuleBackfillProgressWireShape pins the progress event's JSON, because that DTO is the one on the
// rules surface that Wails does not generate a TypeScript type for: it travels on an event rather than
// as a bound method's return, so nothing type-checks the front end's hand-written interface against it.
// The wire is therefore stated twice; this is what compares the two statements.
func TestRuleBackfillProgressWireShape(t *testing.T) {
	encoded, err := json.Marshal(RuleBackfillProgressDTO{Phase: "scanning", Done: 3, Total: 7})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"phase":"scanning","done":3,"total":7}`
	if string(encoded) != want {
		t.Errorf("progress wire shape changed:\n got %s\nwant %s", encoded, want)
	}
}
