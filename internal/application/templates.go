package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TemplateInput carries the fields needed to create or update a message template. An empty ID means a
// new template.
type TemplateInput struct {
	ID      string
	Name    string
	Subject string
	Body    string
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

// List returns all templates.
func (s *TemplateService) List(ctx context.Context) ([]domain.Template, error) {
	templates, err := s.templates.ListTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("templates: list: %w", err)
	}
	return templates, nil
}

// Save validates and persists a template, generating an id when one is not supplied (a new template).
func (s *TemplateService) Save(ctx context.Context, in TemplateInput) error {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.newID()
	}
	template, err := domain.NewTemplate(id, in.Name, in.Subject, in.Body)
	if err != nil {
		return fmt.Errorf("templates: build template: %w", err)
	}
	if err := s.templates.SaveTemplate(ctx, template); err != nil {
		return fmt.Errorf("templates: save: %w", err)
	}
	return nil
}

// Delete removes a template by id.
func (s *TemplateService) Delete(ctx context.Context, id string) error {
	if err := s.templates.DeleteTemplate(ctx, id); err != nil {
		return fmt.Errorf("templates: delete %q: %w", id, err)
	}
	return nil
}
