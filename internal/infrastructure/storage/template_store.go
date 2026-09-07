package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// templateRow is a template's own columns, before its attachments are joined on.
type templateRow struct {
	id      string
	name    string
	subject string
	body    string
}

// templateChild pairs a child row with the template it belongs to, so one query serves every template.
type templateChild struct {
	templateID string
	value      domain.TemplateAttachment
}

// ListTemplates returns every defined template with its attachments, ordered by name for a stable
// display. The attachments are read in one query and grouped, rather than one query per template.
func (s *Store) ListTemplates(ctx context.Context) ([]domain.Template, error) {
	rows, err := queryRows(ctx, s.db, "templates",
		"SELECT id, name, subject, body FROM template ORDER BY name;", scanTemplateRow)
	if err != nil {
		return nil, err
	}
	attachments, err := s.listTemplateAttachments(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Template, 0, len(rows))
	for _, row := range rows {
		template, err := domain.NewTemplateWithAttachments(row.id, row.name, row.subject, row.body,
			attachments[row.id])
		if err != nil {
			return nil, fmt.Errorf("rebuild template %q: %w", row.id, err)
		}
		out = append(out, template)
	}
	return out, nil
}

// listTemplateAttachments returns every template's attachment DESCRIPTIONS keyed by template id, each
// in stored position order. It reads LENGTH(content) rather than the content itself, so listing the
// templates costs nothing in proportion to the bytes they carry.
func (s *Store) listTemplateAttachments(ctx context.Context) (map[string][]domain.TemplateAttachment, error) {
	rows, err := queryRows(ctx, s.db, "template attachments",
		`SELECT template_id, filename, content_type, LENGTH(content) FROM template_attachment
		 ORDER BY template_id, position;`,
		func(row scanner) (templateChild, error) {
			var (
				templateID, filename, contentType string
				size                              int
			)
			if err := row.Scan(&templateID, &filename, &contentType, &size); err != nil {
				return templateChild{}, fmt.Errorf("scan template attachment: %w", err)
			}
			described, err := domain.NewTemplateAttachment(filename, contentType, size)
			if err != nil {
				return templateChild{}, fmt.Errorf("rebuild attachment of template %q: %w", templateID, err)
			}
			return templateChild{templateID: templateID, value: described}, nil
		})
	if err != nil {
		return nil, err
	}
	out := make(map[string][]domain.TemplateAttachment)
	for _, r := range rows {
		out[r.templateID] = append(out[r.templateID], r.value)
	}
	return out, nil
}

// TemplateFiles returns one template's attachments with their bytes, in stored order. It is the read
// for the two moments a template's content is actually wanted: inserting it into a message and loading
// it for editing.
func (s *Store) TemplateFiles(ctx context.Context, templateID string) ([]domain.Attachment, error) {
	return queryRows(ctx, s.db, "template files",
		`SELECT filename, content_type, content FROM template_attachment
		 WHERE template_id = ? ORDER BY position;`,
		func(row scanner) (domain.Attachment, error) {
			var (
				filename, contentType string
				content               []byte
			)
			if err := row.Scan(&filename, &contentType, &content); err != nil {
				return domain.Attachment{}, fmt.Errorf("scan template file: %w", err)
			}
			attachment, err := domain.NewAttachment(filename, contentType, content)
			if err != nil {
				return domain.Attachment{}, fmt.Errorf("rebuild file of template %q: %w", templateID, err)
			}
			return attachment, nil
		}, templateID)
}

// SaveTemplate inserts or replaces a template with its attachments in one transaction, so a template
// can never be left holding a half-written set of them. The attachments are replaced outright rather
// than merged, because their positions define the stored order.
func (s *Store) SaveTemplate(ctx context.Context, template domain.Template, files []domain.Attachment) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO template (id, name, subject, body) VALUES (?, ?, ?, ?);",
			template.ID(), template.Name(), template.Subject(), template.Body()); err != nil {
			return fmt.Errorf("save template %q: %w", template.ID(), err)
		}
		if err := deleteTemplateAttachments(ctx, tx, template.ID()); err != nil {
			return err
		}
		for i, a := range files {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO template_attachment (template_id, position, filename, content_type, content)
				 VALUES (?, ?, ?, ?, ?);`,
				template.ID(), i, a.Filename(), a.ContentType(), a.Content()); err != nil {
				return fmt.Errorf("save attachment %d of template %q: %w", i, template.ID(), err)
			}
		}
		return nil
	})
}

// DeleteTemplate removes a template and its attachments in one transaction, so deleting a template
// cannot leave its files behind with nothing referencing them.
func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := deleteTemplateAttachments(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM template WHERE id = ?;", id); err != nil {
			return fmt.Errorf("delete template %q: %w", id, err)
		}
		return nil
	})
}

// deleteTemplateAttachments clears one template's attachments.
func deleteTemplateAttachments(ctx context.Context, tx *sql.Tx, templateID string) error {
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM template_attachment WHERE template_id = ?;", templateID); err != nil {
		return fmt.Errorf("clear attachments of template %q: %w", templateID, err)
	}
	return nil
}

// scanTemplateRow reads one template row (id, name, subject, body); its attachments are joined on by
// the caller.
func scanTemplateRow(row scanner) (templateRow, error) {
	var out templateRow
	if err := row.Scan(&out.id, &out.name, &out.subject, &out.body); err != nil {
		return templateRow{}, fmt.Errorf("scan template: %w", err)
	}
	return out, nil
}
