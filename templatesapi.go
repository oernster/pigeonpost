package main

import (
	"github.com/oernster/pigeonpost/internal/application"
)

// TemplateRequest is the front-end payload for creating or updating a message template. An empty ID
// means a new template.
type TemplateRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// ListTemplates returns every defined message template.
func (a *App) ListTemplates() ([]TemplateDTO, error) {
	templates, err := a.templates.List(a.ctx)
	if err != nil {
		return nil, err
	}
	return toTemplateDTOs(templates), nil
}

// SaveTemplate creates or updates a message template. A blank ID mints a new one.
func (a *App) SaveTemplate(req TemplateRequest) error {
	return a.templates.Save(a.ctx, application.TemplateInput{
		ID:      req.ID,
		Name:    req.Name,
		Subject: req.Subject,
		Body:    req.Body,
	})
}

// DeleteTemplate removes a message template by id.
func (a *App) DeleteTemplate(id string) error {
	return a.templates.Delete(a.ctx, id)
}
