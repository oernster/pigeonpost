package main

import (
	"encoding/base64"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// TemplateRequest is the front-end payload for creating or updating a message template. An empty ID
// means a new template.
//
// KeepFilePositions and AddFilePaths are how an edit states what became of the files already stored:
// the positions to carry over, in the order they should end up in, then the paths of newly chosen
// files. A file already stored is named rather than resent, so editing a subject does not push the
// template's bytes across the bridge and back.
type TemplateRequest struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Subject           string   `json:"subject"`
	Body              string   `json:"body"`
	KeepFilePositions []int    `json:"keepFilePositions"`
	AddFilePaths      []string `json:"addFilePaths"`
}

// TemplateFileDTO is one of a template's files with its bytes, base64 encoded, in the shape the compose
// window already attaches pasted files in (AttachmentDataEntry in send.go). Inserting a template
// therefore adds its files through the path the composer has always used.
type TemplateFileDTO struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

// ListTemplates returns every defined message template, each describing its files without carrying
// them.
func (a *App) ListTemplates() ([]TemplateDTO, error) {
	templates, err := a.templates.List(a.ctx)
	if err != nil {
		return nil, err
	}
	return toTemplateDTOs(templates), nil
}

// TemplateFiles returns one template's attachments with their bytes, for inserting it into a message.
func (a *App) TemplateFiles(id string) ([]TemplateFileDTO, error) {
	files, err := a.templates.Files(a.ctx, id)
	if err != nil {
		return nil, err
	}
	return toTemplateFileDTOs(files), nil
}

// toTemplateFileDTOs encodes each file for the bridge. It is a function of its own so the wire shape
// can be asserted without a running app.
func toTemplateFileDTOs(files []domain.Attachment) []TemplateFileDTO {
	out := make([]TemplateFileDTO, 0, len(files))
	for _, f := range files {
		out = append(out, TemplateFileDTO{
			Name:        f.Filename(),
			ContentType: f.ContentType(),
			Content:     base64.StdEncoding.EncodeToString(f.Content()),
		})
	}
	return out
}

// SaveTemplate creates or updates a message template. A blank ID mints a new one. The newly chosen
// files are read here, since reading a path is the facade's business; the use case resolves them
// against the ones already stored.
func (a *App) SaveTemplate(req TemplateRequest) error {
	added, err := readAttachments(req.AddFilePaths)
	if err != nil {
		return fmt.Errorf("attach to template: %w", err)
	}
	return a.templates.Save(a.ctx, application.TemplateInput{
		ID:        req.ID,
		Name:      req.Name,
		Subject:   req.Subject,
		Body:      req.Body,
		KeepFiles: req.KeepFilePositions,
		AddFiles:  added,
	})
}

// DeleteTemplate removes a message template by id.
func (a *App) DeleteTemplate(id string) error {
	return a.templates.Delete(a.ctx, id)
}
