package main

import (
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// RuleConditionDTO is the JSON-serialisable view of one rule condition. Field and operator are stable
// string tokens ("from", "anyRecipient", "contains", "endsWith" and so on) so the front end does not
// depend on the domain enum values.
type RuleConditionDTO struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Text     string `json:"text"`
	// CaseSensitive makes the comparison exact; the default is case-insensitive matching.
	CaseSensitive bool `json:"caseSensitive"`
}

// RuleActionDTO is the JSON-serialisable view of one rule action. Kind is a stable string token
// ("markRead", "flag", "moveTo", "destroy"); FolderID is the destination of a move and empty otherwise.
type RuleActionDTO struct {
	Kind     string `json:"kind"`
	FolderID string `json:"folderId"`
}

// RuleDTO is the JSON-serialisable view of a filter rule, carried in both directions: the front end
// lists rules as these and sends one back to save. An empty ID on a save means a new rule.
type RuleDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	Position       int    `json:"position"`
	MatchMode      string `json:"matchMode"`
	StopProcessing bool   `json:"stopProcessing"`
	// AccountIDs limits the rule to the named accounts; empty means every account.
	AccountIDs []string           `json:"accountIds"`
	Conditions []RuleConditionDTO `json:"conditions"`
	Actions    []RuleActionDTO    `json:"actions"`
}

// ListRules returns all filter rules in evaluation order.
func (a *App) ListRules() ([]RuleDTO, error) {
	rules, err := a.rules.List(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RuleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, ruleToDTO(r))
	}
	return out, nil
}

// SaveRule creates or updates a filter rule.
func (a *App) SaveRule(req RuleDTO) error {
	matchMode, err := parseRuleMatchMode(req.MatchMode)
	if err != nil {
		return err
	}
	conditions, err := parseRuleConditions(req.Conditions)
	if err != nil {
		return err
	}
	actions, err := parseRuleActions(req.Actions)
	if err != nil {
		return err
	}
	return a.rules.Save(a.ctx, application.RuleInput{
		ID:             req.ID,
		Name:           req.Name,
		Enabled:        req.Enabled,
		Position:       req.Position,
		MatchMode:      matchMode,
		StopProcessing: req.StopProcessing,
		AccountIDs:     req.AccountIDs,
		Conditions:     conditions,
		Actions:        actions,
	})
}

// DeleteRule removes a filter rule by id.
func (a *App) DeleteRule(ruleID string) error {
	return a.rules.Delete(a.ctx, ruleID)
}

// ReorderRules writes the evaluation order, the rule at index i taking position i.
func (a *App) ReorderRules(orderedIDs []string) error {
	return a.rules.Reorder(a.ctx, orderedIDs)
}

// ruleToDTO converts a domain rule to its wire view.
func ruleToDTO(r domain.Rule) RuleDTO {
	conditions := make([]RuleConditionDTO, 0, len(r.Conditions()))
	for _, c := range r.Conditions() {
		conditions = append(conditions, RuleConditionDTO{
			Field: c.Field().String(), Operator: c.Operator().String(), Text: c.Text(),
			CaseSensitive: c.CaseSensitive(),
		})
	}
	actions := make([]RuleActionDTO, 0, len(r.Actions()))
	for _, a := range r.Actions() {
		actions = append(actions, RuleActionDTO{Kind: a.Kind().String(), FolderID: a.FolderID()})
	}
	// A rule limited to no account has a nil AccountIDs, and encoding/json writes a nil slice as null
	// rather than []. The front end's type declares an array, so a null there is not a wrong value but
	// a crash when its length is read, and with no error boundary above the app that takes the whole
	// window down instead of one dialog. Conditions and actions are already built with make, so they
	// cannot be nil; this is the one list that can, and it is fixed here rather than in the front end
	// because the wire shape is this function's promise to keep.
	accountIDs := r.AccountIDs()
	if accountIDs == nil {
		accountIDs = []string{}
	}
	return RuleDTO{
		ID: r.ID(), Name: r.Name(), Enabled: r.Enabled(), Position: r.Position(),
		MatchMode: r.MatchMode().String(), StopProcessing: r.StopProcessing(),
		AccountIDs: accountIDs, Conditions: conditions, Actions: actions,
	}
}

// parseRuleConditions converts the wire conditions to their application inputs.
func parseRuleConditions(in []RuleConditionDTO) ([]application.RuleConditionInput, error) {
	out := make([]application.RuleConditionInput, 0, len(in))
	for _, c := range in {
		field, err := parseRuleField(c.Field)
		if err != nil {
			return nil, err
		}
		operator, err := parseRuleOperator(c.Operator)
		if err != nil {
			return nil, err
		}
		out = append(out, application.RuleConditionInput{
			Field: field, Operator: operator, Text: c.Text, CaseSensitive: c.CaseSensitive,
		})
	}
	return out, nil
}

// parseRuleActions converts the wire actions to their application inputs.
func parseRuleActions(in []RuleActionDTO) ([]application.RuleActionInput, error) {
	out := make([]application.RuleActionInput, 0, len(in))
	for _, a := range in {
		kind, err := parseRuleActionKind(a.Kind)
		if err != nil {
			return nil, err
		}
		out = append(out, application.RuleActionInput{Kind: kind, FolderID: a.FolderID})
	}
	return out, nil
}

func parseRuleField(s string) (domain.RuleField, error) {
	switch s {
	case "from":
		return domain.RuleFieldFrom, nil
	case "subject":
		return domain.RuleFieldSubject, nil
	case "to":
		return domain.RuleFieldTo, nil
	case "cc":
		return domain.RuleFieldCc, nil
	case "anyRecipient":
		return domain.RuleFieldAnyRecipient, nil
	case "senderDomain":
		return domain.RuleFieldSenderDomain, nil
	case "all":
		return domain.RuleFieldAll, nil
	default:
		return 0, fmt.Errorf("unknown rule field %q", s)
	}
}

func parseRuleOperator(s string) (domain.RuleOperator, error) {
	switch s {
	case "contains":
		return domain.RuleOpContains, nil
	case "notContains":
		return domain.RuleOpNotContains, nil
	case "equals":
		return domain.RuleOpEquals, nil
	case "startsWith":
		return domain.RuleOpStartsWith, nil
	case "endsWith":
		return domain.RuleOpEndsWith, nil
	default:
		return 0, fmt.Errorf("unknown rule operator %q", s)
	}
}

func parseRuleMatchMode(s string) (domain.RuleMatchMode, error) {
	switch s {
	case "all":
		return domain.RuleMatchAll, nil
	case "any":
		return domain.RuleMatchAny, nil
	default:
		return 0, fmt.Errorf("unknown rule match mode %q", s)
	}
}

func parseRuleActionKind(s string) (domain.RuleActionKind, error) {
	switch s {
	case "markRead":
		return domain.RuleMarkRead, nil
	case "flag":
		return domain.RuleFlag, nil
	case "moveTo":
		return domain.RuleMoveTo, nil
	case "destroy":
		return domain.RuleDestroy, nil
	default:
		return 0, fmt.Errorf("unknown rule action %q", s)
	}
}
