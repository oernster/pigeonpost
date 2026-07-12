package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TagService is the use-case boundary for managing coloured tags and their assignment to messages.
type TagService struct {
	tags TagStore
}

// NewTagService constructs the service with its injected tag store.
func NewTagService(tags TagStore) *TagService {
	return &TagService{tags: tags}
}

// List returns all defined tags.
func (s *TagService) List(ctx context.Context) ([]domain.Tag, error) {
	tags, err := s.tags.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}

// Save creates or updates a tag.
func (s *TagService) Save(ctx context.Context, tag domain.Tag) error {
	if err := s.tags.SaveTag(ctx, tag); err != nil {
		return fmt.Errorf("save tag %q: %w", tag.ID(), err)
	}
	return nil
}

// Delete removes a tag and detaches it from every message.
func (s *TagService) Delete(ctx context.Context, id string) error {
	if err := s.tags.DeleteTag(ctx, id); err != nil {
		return fmt.Errorf("delete tag %q: %w", id, err)
	}
	return nil
}

// ForMessage returns the tags attached to a message.
func (s *TagService) ForMessage(ctx context.Context, messageID string) ([]domain.Tag, error) {
	tags, err := s.tags.TagsForMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("tags for message %q: %w", messageID, err)
	}
	return tags, nil
}

// ColoursForMessages returns the hex colours of the tags on each of the given messages, keyed by message
// id. A message with no tags is absent from the map. It backs the tag colours shown in the message list,
// fetched in one query rather than per row.
func (s *TagService) ColoursForMessages(ctx context.Context, messageIDs []string) (map[string][]string, error) {
	colours, err := s.tags.TagColoursForMessages(ctx, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("tag colours for messages: %w", err)
	}
	return colours, nil
}
