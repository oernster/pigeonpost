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
	return RuleInput{Name: "News", Field: domain.RuleFieldFrom, Operator: domain.RuleOpContains, Contains: "news@", Action: domain.RuleMarkRead}
}

func TestRuleList(t *testing.T) {
	svc, store := newRuleService()
	rule, _ := domain.NewRule("r1", "News", domain.RuleFieldFrom, domain.RuleOpContains, "news@", domain.RuleMarkRead)
	store.rules = []domain.Rule{rule}

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
