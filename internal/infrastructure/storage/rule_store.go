package storage

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListRules returns all filter rules, ordered by name for a stable display.
func (s *Store) ListRules(ctx context.Context) ([]domain.Rule, error) {
	return queryRows(ctx, s.db, "rules",
		"SELECT id, name, field, operator, contains, action FROM rule ORDER BY name;",
		func(row scanner) (domain.Rule, error) {
			var (
				id, name, contains      string
				field, operator, action int
			)
			if err := row.Scan(&id, &name, &field, &operator, &contains, &action); err != nil {
				return domain.Rule{}, fmt.Errorf("scan rule: %w", err)
			}
			rule, err := domain.NewRule(id, name, domain.RuleField(field),
				domain.RuleOperator(operator), contains, domain.RuleAction(action))
			if err != nil {
				return domain.Rule{}, fmt.Errorf("rebuild rule %q: %w", id, err)
			}
			return rule, nil
		})
}

// SaveRule inserts or updates a rule by id.
func (s *Store) SaveRule(ctx context.Context, rule domain.Rule) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO rule (id, name, field, operator, contains, action) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, field = excluded.field,
		     operator = excluded.operator, contains = excluded.contains, action = excluded.action;`,
		rule.ID(), rule.Name(), int(rule.Field()), int(rule.Operator()), rule.Contains(), int(rule.Action())); err != nil {
		return fmt.Errorf("save rule %q: %w", rule.ID(), err)
	}
	return nil
}

// DeleteRule removes a rule by id.
func (s *Store) DeleteRule(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM rule WHERE id = ?;", id); err != nil {
		return fmt.Errorf("delete rule %q: %w", id, err)
	}
	return nil
}
