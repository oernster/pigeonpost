package main

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TagRequest is the front-end payload for creating or updating a tag. An empty ID means create; a
// present ID updates that tag.
type TagRequest struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Colour string `json:"colour"`
}

// ListTags returns every defined tag.
func (a *App) ListTags() ([]TagDTO, error) {
	tags, err := a.tags.List(a.ctx)
	if err != nil {
		return nil, err
	}
	return toTagDTOs(tags), nil
}

// SaveTag creates or updates a coloured tag. A blank ID mints a new one.
func (a *App) SaveTag(req TagRequest) error {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = uuid.NewString()
	}
	colour, err := domain.NewColour(req.Colour)
	if err != nil {
		return fmt.Errorf("invalid tag colour: %w", err)
	}
	tag, err := domain.NewTag(id, req.Name, colour)
	if err != nil {
		return fmt.Errorf("build tag: %w", err)
	}
	return a.tags.Save(a.ctx, tag)
}

// DeleteTag removes a tag and detaches it from every message.
func (a *App) DeleteTag(tagID string) error {
	return a.tags.Delete(a.ctx, tagID)
}

// MessageTags returns the tags attached to a message.
func (a *App) MessageTags(messageID string) ([]TagDTO, error) {
	tags, err := a.tags.ForMessage(a.ctx, messageID)
	if err != nil {
		return nil, err
	}
	return toTagDTOs(tags), nil
}

// SetMessageTag attaches or detaches a tag from a message.
func (a *App) SetMessageTag(messageID, tagID string, assigned bool) error {
	if assigned {
		return a.tags.Assign(a.ctx, messageID, tagID)
	}
	return a.tags.Unassign(a.ctx, messageID, tagID)
}
