package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// RuleConditionInput is one field-operator-text test in a rule being saved.
type RuleConditionInput struct {
	Field    domain.RuleField
	Operator domain.RuleOperator
	Text     string
	// CaseSensitive makes the comparison exact. The default (false) is the case-insensitive matching
	// every rule had before the flag existed.
	CaseSensitive bool
}

// RuleActionInput is one action in a rule being saved. FolderID is the destination of a move and is
// ignored by every other kind.
type RuleActionInput struct {
	Kind     domain.RuleActionKind
	FolderID string
}

// RuleInput carries the fields needed to create or update a filter rule. An empty ID means a new rule.
type RuleInput struct {
	ID             string
	Name           string
	Enabled        bool
	Position       int
	MatchMode      domain.RuleMatchMode
	StopProcessing bool
	// AccountIDs limits the rule to the named accounts; empty means every account.
	AccountIDs []string
	Conditions []RuleConditionInput
	Actions    []RuleActionInput
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

// List returns all rules in evaluation order.
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
	conditions, err := buildConditions(in.Conditions)
	if err != nil {
		return err
	}
	actions, err := buildActions(in.Actions)
	if err != nil {
		return err
	}
	rule, err := domain.NewRule(domain.RuleSpec{
		ID:             id,
		Name:           in.Name,
		Enabled:        in.Enabled,
		Position:       in.Position,
		MatchMode:      in.MatchMode,
		StopProcessing: in.StopProcessing,
		AccountIDs:     in.AccountIDs,
		Conditions:     conditions,
		Actions:        actions,
	})
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

// Reorder writes the evaluation order: the rule at index i in orderedIDs is given position i. A rule
// the list does not name is left where it is.
func (s *RuleService) Reorder(ctx context.Context, orderedIDs []string) error {
	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return fmt.Errorf("rules: reorder: list: %w", err)
	}
	byID := make(map[string]domain.Rule, len(rules))
	for _, r := range rules {
		byID[r.ID()] = r
	}
	for position, id := range orderedIDs {
		rule, ok := byID[id]
		if !ok || rule.Position() == position {
			continue
		}
		if err := s.rules.SaveRule(ctx, rule.WithPosition(position)); err != nil {
			return fmt.Errorf("rules: reorder save %q: %w", id, err)
		}
	}
	return nil
}

// buildConditions validates every condition in the input, so a rule is rejected whole rather than
// saved with a condition quietly dropped.
func buildConditions(in []RuleConditionInput) ([]domain.RuleCondition, error) {
	out := make([]domain.RuleCondition, 0, len(in))
	for i, c := range in {
		cond, err := domain.NewRuleConditionCased(c.Field, c.Operator, c.Text, c.CaseSensitive)
		if err != nil {
			return nil, fmt.Errorf("rules: condition %d: %w", i+1, err)
		}
		out = append(out, cond)
	}
	return out, nil
}

// buildActions validates every action in the input.
func buildActions(in []RuleActionInput) ([]domain.RuleAction, error) {
	out := make([]domain.RuleAction, 0, len(in))
	for i, a := range in {
		action, err := domain.NewRuleAction(a.Kind, a.FolderID)
		if err != nil {
			return nil, fmt.Errorf("rules: action %d: %w", i+1, err)
		}
		out = append(out, action)
	}
	return out, nil
}
