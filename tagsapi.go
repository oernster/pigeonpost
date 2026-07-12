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

// SaveTag creates or updates a coloured tag. A blank ID mints a new one. Each tag carries a stable IMAP
// keyword: it is derived from the name once when the tag is created, then frozen, so a later rename
// keeps the same keyword (renaming must not orphan the tag's server-side assignments). A create refuses a
// name whose keyword is already in use, so no two tags ever collide on one keyword.
func (a *App) SaveTag(req TagRequest) error {
	name := strings.TrimSpace(req.Name)
	colour, err := domain.NewColour(req.Colour)
	if err != nil {
		return fmt.Errorf("invalid tag colour: %w", err)
	}
	existing, err := a.tags.List(a.ctx)
	if err != nil {
		return fmt.Errorf("load tags: %w", err)
	}
	id := strings.TrimSpace(req.ID)
	keyword := ""
	if id != "" {
		// Update: preserve the current tag's frozen keyword so a rename never changes it.
		for _, t := range existing {
			if t.ID() == id {
				keyword = t.Keyword()
				break
			}
		}
	}
	if keyword == "" {
		// Create (or an update of an unknown id): derive the keyword from the name and refuse it if another
		// tag already uses it, so the two could not collide on the same server keyword.
		keyword = domain.KeywordForName(name)
		for _, t := range existing {
			if t.ID() != id && t.Keyword() == keyword {
				return domain.ErrDuplicateTag
			}
		}
		if id == "" {
			id = uuid.NewString()
		}
	}
	tag, err := domain.NewTag(id, name, colour, keyword)
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

// SetMessageTag attaches or detaches a tag from a message. It goes through the tag-sync service so the
// change is also rounded onto the server as an IMAP keyword (best-effort, replayed on the next sync).
func (a *App) SetMessageTag(messageID, tagID string, assigned bool) error {
	if assigned {
		return a.tagSync.Assign(a.ctx, messageID, tagID)
	}
	return a.tagSync.Unassign(a.ctx, messageID, tagID)
}
