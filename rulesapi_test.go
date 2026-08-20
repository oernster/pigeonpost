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
// nil Go slice encodes as JSON null, the front end's type says string[], and reading the length off
// null threw during render. React has no error boundary above the app, so one unscoped rule took
// PigeonPost down entirely rather than breaking one dialog. Every list on this DTO must therefore
// reach the wire as an array, empty or not, and the check is on the encoded bytes because that is
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
