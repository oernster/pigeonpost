package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newRuleService() (*RuleService, *fakeRuleStore) {
	rules := &fakeRuleStore{}
	return NewRuleService(rules, func() string { return "generated-id" }), rules
}

func validRuleInput() RuleInput {
	return RuleInput{
		Name:       "News",
		Enabled:    true,
		Conditions: []RuleConditionInput{{Field: domain.RuleFieldFrom, Operator: domain.RuleOpContains, Text: "news@"}},
		Actions:    []RuleActionInput{{Kind: domain.RuleMarkRead}},
	}
}

// testRule builds a stored rule for the service tests.
func testRule(t *testing.T, id string, position int) domain.Rule {
	t.Helper()
	cond, err := domain.NewRuleCondition(domain.RuleFieldFrom, domain.RuleOpContains, "news@")
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	action, err := domain.NewRuleAction(domain.RuleMarkRead, "")
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	rule, err := domain.NewRule(domain.RuleSpec{
		ID: id, Name: "News", Enabled: true, Position: position,
		Conditions: []domain.RuleCondition{cond}, Actions: []domain.RuleAction{action},
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return rule
}

func TestRuleList(t *testing.T) {
	svc, store := newRuleService()
	store.rules = []domain.Rule{testRule(t, "r1", 0)}

	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID() != "r1" {
		t.Errorf("expected r1, got %+v", got)
	}

	store.listErr = errBoom
	if _, err := svc.List(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRuleSaveNew(t *testing.T) {
	svc, store := newRuleService()
	if err := svc.Save(context.Background(), validRuleInput()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.saved) != 1 || store.saved[0].ID() != "generated-id" {
		t.Errorf("expected a generated id, got %+v", store.saved)
	}
}

func TestRuleSaveExisting(t *testing.T) {
	svc, store := newRuleService()
	in := validRuleInput()
	in.ID = "r7"
	if err := svc.Save(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.saved) != 1 || store.saved[0].ID() != "r7" {
		t.Errorf("expected id r7 kept, got %+v", store.saved)
	}
}

func TestRuleSaveInvalid(t *testing.T) {
	svc, _ := newRuleService()
	in := validRuleInput()
	in.Name = "  "
	if err := svc.Save(context.Background(), in); !errors.Is(err, domain.ErrEmptyRuleName) {
		t.Errorf("error = %v, want ErrEmptyRuleName", err)
	}
}

func TestRuleSaveStoreError(t *testing.T) {
	svc, store := newRuleService()
	store.saveErr = errBoom
	if err := svc.Save(context.Background(), validRuleInput()); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRuleDelete(t *testing.T) {
	svc, store := newRuleService()
	if err := svc.Delete(context.Background(), "r1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "r1" {
		t.Errorf("expected delete of r1, got %v", store.deleted)
	}

	store.deleteErr = errBoom
	if err := svc.Delete(context.Background(), "r2"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestRuleSaveInvalidCondition(t *testing.T) {
	svc, _ := newRuleService()
	in := validRuleInput()
	in.Conditions[0].Text = "   "
	if err := svc.Save(context.Background(), in); !errors.Is(err, domain.ErrEmptyRuleMatch) {
		t.Errorf("error = %v, want ErrEmptyRuleMatch", err)
	}
}

func TestRuleSaveInvalidAction(t *testing.T) {
	svc, _ := newRuleService()
	in := validRuleInput()
	in.Actions = []RuleActionInput{{Kind: domain.RuleMoveTo}}
	if err := svc.Save(context.Background(), in); !errors.Is(err, domain.ErrMissingRuleFolder) {
		t.Errorf("error = %v, want ErrMissingRuleFolder", err)
	}
}

func TestRuleReorder(t *testing.T) {
	svc, store := newRuleService()
	store.rules = []domain.Rule{testRule(t, "a", 0), testRule(t, "b", 1)}
	if err := svc.Reorder(context.Background(), []string{"b", "a", "unknown"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	positions := make(map[string]int, len(store.saved))
	for _, r := range store.saved {
		positions[r.ID()] = r.Position()
	}
	if positions["b"] != 0 || positions["a"] != 1 {
		t.Errorf("positions not written: %v", positions)
	}
}

// TestRuleReorderSkipsUnchanged pins that a rule already at its target position is not rewritten.
func TestRuleReorderSkipsUnchanged(t *testing.T) {
	svc, store := newRuleService()
	store.rules = []domain.Rule{testRule(t, "a", 0), testRule(t, "b", 1)}
	if err := svc.Reorder(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("rewrote %d unchanged rules", len(store.saved))
	}
}

func TestRuleReorderErrors(t *testing.T) {
	svc, store := newRuleService()
	store.listErr = errBoom
	if err := svc.Reorder(context.Background(), []string{"a"}); !errors.Is(err, errBoom) {
		t.Errorf("list error = %v, want wrapped boom", err)
	}
	svc, store = newRuleService()
	store.rules = []domain.Rule{testRule(t, "a", 3)}
	store.saveErr = errBoom
	if err := svc.Reorder(context.Background(), []string{"a"}); !errors.Is(err, errBoom) {
		t.Errorf("save error = %v, want wrapped boom", err)
	}
}
