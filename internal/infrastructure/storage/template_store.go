package storage

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListTemplates returns every defined template, ordered by name for a stable display.
func (s *Store) ListTemplates(ctx context.Context) ([]domain.Template, error) {
	return queryRows(ctx, s.db, "templates",
		"SELECT id, name, subject, body FROM template ORDER BY name;", scanTemplate)
}

// SaveTemplate inserts or replaces a template.
func (s *Store) SaveTemplate(ctx context.Context, template domain.Template) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO template (id, name, subject, body) VALUES (?, ?, ?, ?);",
		template.ID(), template.Name(), template.Subject(), template.Body())
	if err != nil {
		return fmt.Errorf("save template %q: %w", template.ID(), err)
	}
	return nil
}

// DeleteTemplate removes a template by id.
func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM template WHERE id = ?;", id); err != nil {
		return fmt.Errorf("delete template %q: %w", id, err)
	}
	return nil
}

// scanTemplate reads one template row (id, name, subject, body) into a validated domain template.
func scanTemplate(row scanner) (domain.Template, error) {
	var id, name, subject, body string
	if err := row.Scan(&id, &name, &subject, &body); err != nil {
		return domain.Template{}, fmt.Errorf("scan template: %w", err)
	}
	template, err := domain.NewTemplate(id, name, subject, body)
	if err != nil {
		return domain.Template{}, fmt.Errorf("rebuild template %q: %w", id, err)
	}
	return template, nil
}
