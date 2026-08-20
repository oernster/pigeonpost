package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// buildRule assembles a rule for the wire tests.
func buildRule(t *testing.T, accountIDs []string) domain.Rule {
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

// TestRuleDTOEmitsEmptyArraysNotNull is the guard for a defect that took the whole window down: a Go
// nil slice encodes as JSON null, the front end's type says string[], and reading .length off null
// threw during render. React has no error boundary above the app, so one unscoped rule blanked
// PigeonPost entirely. Every list on this DTO must therefore reach the wire as an array, empty or not.
func TestRuleDTOEmitsEmptyArraysNotNull(t *testing.T) {
	encoded, err := json.Marshal(ruleToDTO(buildRule(t, nil)))
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

// TestRuleDTOCarriesTheScope pins that a scoped rule still sends its accounts.
func TestRuleDTOCarriesTheScope(t *testing.T) {
	dto := ruleToDTO(buildRule(t, []string{"a1", "a2"}))
	if len(dto.AccountIDs) != 2 || dto.AccountIDs[0] != "a1" || dto.AccountIDs[1] != "a2" {
		t.Errorf("scope lost on the way to the wire: %v", dto.AccountIDs)
	}
}
