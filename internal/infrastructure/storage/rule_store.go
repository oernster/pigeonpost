package storage

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListRules returns all filter rules, ordered by name for a stable display.
func (s *Store) ListRules(ctx context.Context) ([]domain.Rule, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, field, contains, action FROM rule ORDER BY name;")
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.Rule
	for rows.Next() {
		var (
			id, name, contains string
			field, action      int
		)
		if err := rows.Scan(&id, &name, &field, &contains, &action); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		rule, err := domain.NewRule(id, name, domain.RuleField(field), contains, domain.RuleAction(action))
		if err != nil {
			return nil, fmt.Errorf("rebuild rule %q: %w", id, err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rules: %w", err)
	}
	return rules, nil
}

// SaveRule inserts or updates a rule by id.
func (s *Store) SaveRule(ctx context.Context, rule domain.Rule) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO rule (id, name, field, contains, action) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, field = excluded.field,
		     contains = excluded.contains, action = excluded.action;`,
		rule.ID(), rule.Name(), int(rule.Field()), rule.Contains(), int(rule.Action())); err != nil {
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
