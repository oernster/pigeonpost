package main

import (
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// RuleDTO is the JSON-serialisable view of a filter rule. Field and action are stable string tokens
// ("from"/"subject" and "markRead"/"flag") so the front end does not depend on the domain enum values.
type RuleDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Field    string `json:"field"`
	Contains string `json:"contains"`
	Action   string `json:"action"`
}

// RuleRequest is the front-end payload for creating or updating a rule. An empty id means a new rule.
type RuleRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Field    string `json:"field"`
	Contains string `json:"contains"`
	Action   string `json:"action"`
}

// ListRules returns all filter rules.
func (a *App) ListRules() ([]RuleDTO, error) {
	rules, err := a.rules.List(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RuleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, RuleDTO{
			ID:       r.ID(),
			Name:     r.Name(),
			Field:    r.Field().String(),
			Contains: r.Contains(),
			Action:   r.Action().String(),
		})
	}
	return out, nil
}

// SaveRule creates or updates a filter rule.
func (a *App) SaveRule(req RuleRequest) error {
	field, err := parseRuleField(req.Field)
	if err != nil {
		return err
	}
	action, err := parseRuleAction(req.Action)
	if err != nil {
		return err
	}
	return a.rules.Save(a.ctx, application.RuleInput{
		ID:       req.ID,
		Name:     req.Name,
		Field:    field,
		Contains: req.Contains,
		Action:   action,
	})
}

// DeleteRule removes a filter rule by id.
func (a *App) DeleteRule(ruleID string) error {
	return a.rules.Delete(a.ctx, ruleID)
}

func parseRuleField(s string) (domain.RuleField, error) {
	switch s {
	case "from":
		return domain.RuleFieldFrom, nil
	case "subject":
		return domain.RuleFieldSubject, nil
	default:
		return 0, fmt.Errorf("unknown rule field %q", s)
	}
}

func parseRuleAction(s string) (domain.RuleAction, error) {
	switch s {
	case "markRead":
		return domain.RuleMarkRead, nil
	case "flag":
		return domain.RuleFlag, nil
	default:
		return 0, fmt.Errorf("unknown rule action %q", s)
	}
}
