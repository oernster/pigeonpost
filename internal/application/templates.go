package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TemplateInput carries the fields needed to create or update a message template. An empty ID means a
// new template.
//
// The two attachment fields are how an edit says what became of the files already stored. KeepFiles
// names the ones to carry over by their position on the stored template, in the order they should end
// up in; AddFiles carries newly chosen files, which follow. Anything not named in KeepFiles is dropped.
// Saying it this way rather than sending the whole set back means an edit that touches only the subject
// does not push megabytes through the bridge and back.
type TemplateInput struct {
	ID        string
	Name      string
	Subject   string
	Body      string
	KeepFiles []int
	AddFiles  []domain.Attachment
}

// TemplateService is the use-case boundary for managing message templates.
type TemplateService struct {
	templates TemplateStore
	newID     IDGenerator
}

// NewTemplateService constructs the service with its injected store and id generator.
func NewTemplateService(templates TemplateStore, newID IDGenerator) *TemplateService {
	return &TemplateService{templates: templates, newID: newID}
}

// List returns all templates, each describing its files rather than carrying them.
func (s *TemplateService) List(ctx context.Context) ([]domain.Template, error) {
	templates, err := s.templates.ListTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("templates: list: %w", err)
	}
	return templates, nil
}

// Files returns one template's attachments with their bytes, which is what inserting it into a message
// needs. It is a separate read from List because only the chosen template's content is wanted.
func (s *TemplateService) Files(ctx context.Context, templateID string) ([]domain.Attachment, error) {
	files, err := s.templates.TemplateFiles(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("templates: files of %q: %w", templateID, err)
	}
	return files, nil
}

// Save validates and persists a template, generating an id when one is not supplied (a new template).
// The stored files named by KeepFiles are read back and written again with the new ones, so the saved
// set is exactly what the editor was showing.
func (s *TemplateService) Save(ctx context.Context, in TemplateInput) error {
	id := strings.TrimSpace(in.ID)
	isNew := id == ""
	if isNew {
		id = s.newID()
	}
	files, err := s.resolveFiles(ctx, id, isNew, in)
	if err != nil {
		return err
	}
	template, err := domain.NewTemplateFromFiles(id, in.Name, in.Subject, in.Body, files)
	if err != nil {
		return fmt.Errorf("templates: build template: %w", err)
	}
	if err := s.templates.SaveTemplate(ctx, template, files); err != nil {
		return fmt.Errorf("templates: save: %w", err)
	}
	return nil
}

// resolveFiles assembles the files the save will store: the kept ones read back from the store, in the
// order the input names them, then the newly chosen ones.
func (s *TemplateService) resolveFiles(
	ctx context.Context, id string, isNew bool, in TemplateInput,
) ([]domain.Attachment, error) {
	if len(in.KeepFiles) == 0 {
		return in.AddFiles, nil
	}
	// A new template has nothing stored to keep, so a position naming one is a caller error rather than
	// something to resolve against another template's files.
	if isNew {
		return nil, fmt.Errorf("templates: a new template cannot keep stored files")
	}
	stored, err := s.templates.TemplateFiles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("templates: read stored files of %q: %w", id, err)
	}
	out := make([]domain.Attachment, 0, len(in.KeepFiles)+len(in.AddFiles))
	for _, position := range in.KeepFiles {
		if position < 0 || position >= len(stored) {
			return nil, fmt.Errorf("templates: template %q has no file at position %d", id, position)
		}
		out = append(out, stored[position])
	}
	return append(out, in.AddFiles...), nil
}

// Delete removes a template by id.
func (s *TemplateService) Delete(ctx context.Context, id string) error {
	if err := s.templates.DeleteTemplate(ctx, id); err != nil {
		return fmt.Errorf("templates: delete %q: %w", id, err)
	}
	return nil
}
