package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ruleRow is one rule's own columns, before its conditions and actions are attached.
type ruleRow struct {
	id             string
	name           string
	enabled        bool
	position       int
	matchMode      domain.RuleMatchMode
	stopProcessing bool
}

// ListRules returns all filter rules with their conditions and actions, in evaluation order (position
// first, then name so rules sharing a position have a stable display). A rule whose stored parts no
// longer build a valid rule fails the read rather than being silently dropped, so a corrupt row is
// visible instead of quietly disarming a filter.
func (s *Store) ListRules(ctx context.Context) ([]domain.Rule, error) {
	rows, err := s.listRuleRows(ctx)
	if err != nil {
		return nil, err
	}
	conditions, err := s.listRuleConditions(ctx)
	if err != nil {
		return nil, err
	}
	actions, err := s.listRuleActions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Rule, 0, len(rows))
	for _, row := range rows {
		rule, err := domain.NewRule(domain.RuleSpec{
			ID:             row.id,
			Name:           row.name,
			Enabled:        row.enabled,
			Position:       row.position,
			MatchMode:      row.matchMode,
			StopProcessing: row.stopProcessing,
			Conditions:     conditions[row.id],
			Actions:        actions[row.id],
		})
		if err != nil {
			return nil, fmt.Errorf("rebuild rule %q: %w", row.id, err)
		}
		out = append(out, rule)
	}
	return out, nil
}

// listRuleRows reads the rule table itself.
func (s *Store) listRuleRows(ctx context.Context) ([]ruleRow, error) {
	return queryRows(ctx, s.db, "rules",
		`SELECT id, name, enabled, position, match_mode, stop_processing
		 FROM rule ORDER BY position, name;`,
		func(row scanner) (ruleRow, error) {
			var (
				r                   ruleRow
				enabled, stop, mode int
			)
			if err := row.Scan(&r.id, &r.name, &enabled, &r.position, &mode, &stop); err != nil {
				return ruleRow{}, fmt.Errorf("scan rule: %w", err)
			}
			r.enabled = enabled != 0
			r.stopProcessing = stop != 0
			r.matchMode = domain.RuleMatchMode(mode)
			return r, nil
		})
}

// ruleChild pairs a child row with the rule it belongs to, so one query serves every rule.
type ruleChild[T any] struct {
	ruleID string
	value  T
}

// listRuleConditions returns every rule's conditions keyed by rule id, each in stored position order.
func (s *Store) listRuleConditions(ctx context.Context) (map[string][]domain.RuleCondition, error) {
	rows, err := queryRows(ctx, s.db, "rule conditions",
		`SELECT rule_id, field, operator, match_text, case_sensitive FROM rule_condition
		 ORDER BY rule_id, position;`,
		func(row scanner) (ruleChild[domain.RuleCondition], error) {
			var (
				ruleID, text                   string
				field, operator, caseSensitive int
			)
			if err := row.Scan(&ruleID, &field, &operator, &text, &caseSensitive); err != nil {
				return ruleChild[domain.RuleCondition]{}, fmt.Errorf("scan rule condition: %w", err)
			}
			cond, err := domain.NewRuleConditionCased(domain.RuleField(field), domain.RuleOperator(operator),
				text, caseSensitive != 0)
			if err != nil {
				return ruleChild[domain.RuleCondition]{}, fmt.Errorf("rebuild condition of rule %q: %w", ruleID, err)
			}
			return ruleChild[domain.RuleCondition]{ruleID: ruleID, value: cond}, nil
		})
	if err != nil {
		return nil, err
	}
	return groupByRule(rows), nil
}

// listRuleActions returns every rule's actions keyed by rule id, each in stored position order.
func (s *Store) listRuleActions(ctx context.Context) (map[string][]domain.RuleAction, error) {
	rows, err := queryRows(ctx, s.db, "rule actions",
		`SELECT rule_id, kind, folder_id FROM rule_action ORDER BY rule_id, position;`,
		func(row scanner) (ruleChild[domain.RuleAction], error) {
			var (
				ruleID, folderID string
				kind             int
			)
			if err := row.Scan(&ruleID, &kind, &folderID); err != nil {
				return ruleChild[domain.RuleAction]{}, fmt.Errorf("scan rule action: %w", err)
			}
			action, err := domain.NewRuleAction(domain.RuleActionKind(kind), folderID)
			if err != nil {
				return ruleChild[domain.RuleAction]{}, fmt.Errorf("rebuild action of rule %q: %w", ruleID, err)
			}
			return ruleChild[domain.RuleAction]{ruleID: ruleID, value: action}, nil
		})
	if err != nil {
		return nil, err
	}
	return groupByRule(rows), nil
}

// groupByRule collects child rows under their rule id, preserving the query's order.
func groupByRule[T any](rows []ruleChild[T]) map[string][]T {
	out := make(map[string][]T)
	for _, r := range rows {
		out[r.ruleID] = append(out[r.ruleID], r.value)
	}
	return out
}

// SaveRule inserts or updates a rule with its conditions and actions in one transaction, so a rule can
// never be left holding a half-written set of either. The children are replaced outright rather than
// merged, because their positions define the stored order.
func (s *Store) SaveRule(ctx context.Context, rule domain.Rule) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rule (id, name, enabled, position, match_mode, stop_processing)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, enabled = excluded.enabled,
			     position = excluded.position, match_mode = excluded.match_mode,
			     stop_processing = excluded.stop_processing;`,
			rule.ID(), rule.Name(), boolToInt(rule.Enabled()), rule.Position(),
			int(rule.MatchMode()), boolToInt(rule.StopProcessing())); err != nil {
			return fmt.Errorf("save rule %q: %w", rule.ID(), err)
		}
		if err := deleteRuleChildren(ctx, tx, rule.ID()); err != nil {
			return err
		}
		for i, c := range rule.Conditions() {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO rule_condition (rule_id, position, field, operator, match_text, case_sensitive)
				 VALUES (?, ?, ?, ?, ?, ?);`,
				rule.ID(), i, int(c.Field()), int(c.Operator()), c.Text(),
				boolToInt(c.CaseSensitive())); err != nil {
				return fmt.Errorf("save condition %d of rule %q: %w", i, rule.ID(), err)
			}
		}
		for i, a := range rule.Actions() {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO rule_action (rule_id, position, kind, folder_id) VALUES (?, ?, ?, ?);`,
				rule.ID(), i, int(a.Kind()), a.FolderID()); err != nil {
				return fmt.Errorf("save action %d of rule %q: %w", i, rule.ID(), err)
			}
		}
		return nil
	})
}

// DeleteRule removes a rule and its conditions and actions in one transaction.
func (s *Store) DeleteRule(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := deleteRuleChildren(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM rule WHERE id = ?;", id); err != nil {
			return fmt.Errorf("delete rule %q: %w", id, err)
		}
		return nil
	})
}

// deleteRuleChildren clears one rule's conditions and actions.
func deleteRuleChildren(ctx context.Context, tx *sql.Tx, ruleID string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM rule_condition WHERE rule_id = ?;", ruleID); err != nil {
		return fmt.Errorf("clear conditions of rule %q: %w", ruleID, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM rule_action WHERE rule_id = ?;", ruleID); err != nil {
		return fmt.Errorf("clear actions of rule %q: %w", ruleID, err)
	}
	return nil
}
