package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// RuleInput carries the fields needed to create or update a filter rule. An empty ID means a new rule.
type RuleInput struct {
	ID       string
	Name     string
	Field    domain.RuleField
	Operator domain.RuleOperator
	Contains string
	Action   domain.RuleAction
}

// RuleService is the use-case boundary for managing filter rules.
type RuleService struct {
	rules RuleStore
	newID IDGenerator
}

// NewRuleService constructs the service with its injected store and id generator.
func NewRuleService(rules RuleStore, newID IDGenerator) *RuleService {
	return &RuleService{rules: rules, newID: newID}
}

// List returns all rules.
func (s *RuleService) List(ctx context.Context) ([]domain.Rule, error) {
	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("rules: list: %w", err)
	}
	return rules, nil
}

// Save validates and persists a rule, generating an id when one is not supplied (a new rule).
func (s *RuleService) Save(ctx context.Context, in RuleInput) error {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.newID()
	}
	rule, err := domain.NewRule(id, in.Name, in.Field, in.Operator, in.Contains, in.Action)
	if err != nil {
		return fmt.Errorf("rules: build rule: %w", err)
	}
	if err := s.rules.SaveRule(ctx, rule); err != nil {
		return fmt.Errorf("rules: save: %w", err)
	}
	return nil
}

// Delete removes a rule by id.
func (s *RuleService) Delete(ctx context.Context, id string) error {
	if err := s.rules.DeleteRule(ctx, id); err != nil {
		return fmt.Errorf("rules: delete %q: %w", id, err)
	}
	return nil
}
