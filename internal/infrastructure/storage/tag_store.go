package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListTags returns every defined tag, ordered by name.
func (s *Store) ListTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, colour FROM tag ORDER BY name;")
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

// SaveTag inserts or replaces a tag.
func (s *Store) SaveTag(ctx context.Context, tag domain.Tag) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO tag (id, name, colour) VALUES (?, ?, ?);",
		tag.ID(), tag.Name(), tag.Colour().Hex())
	if err != nil {
		return fmt.Errorf("save tag %q: %w", tag.ID(), err)
	}
	return nil
}

// DeleteTag removes a tag and detaches it from every message, in one transaction.
func (s *Store) DeleteTag(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_tag WHERE tag_id = ?;", id); err != nil {
			return fmt.Errorf("detach tag: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM tag WHERE id = ?;", id); err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}
		return nil
	})
}

// TagsForMessage returns the tags attached to a message, ordered by name.
func (s *Store) TagsForMessage(ctx context.Context, messageID string) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.colour FROM tag t
		 JOIN message_tag mt ON mt.tag_id = t.id
		 WHERE mt.message_id = ? ORDER BY t.name;`, messageID)
	if err != nil {
		return nil, fmt.Errorf("query message tags: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

// TagColoursForMessages returns the hex tag colours of each of the given messages in one query, keyed by
// message id, ordered by tag name. A message with no tags is absent from the map. It backs the tag colours
// shown in the message list without a query per row.
func (s *Store) TagColoursForMessages(ctx context.Context, messageIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(messageIDs))
	if len(messageIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(
		`SELECT mt.message_id, t.colour FROM tag t
		 JOIN message_tag mt ON mt.tag_id = t.id
		 WHERE mt.message_id IN (%s) ORDER BY t.name;`, strings.Join(placeholders, ","))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query message tag colours: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var messageID, colour string
		if err := rows.Scan(&messageID, &colour); err != nil {
			return nil, fmt.Errorf("scan message tag colour: %w", err)
		}
		result[messageID] = append(result[messageID], colour)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate message tag colours: %w", err)
	}
	return result, nil
}

// AddMessageTag attaches a tag to a message. Re-attaching an existing pair is a no-op.
func (s *Store) AddMessageTag(ctx context.Context, messageID, tagID string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO message_tag (message_id, tag_id) VALUES (?, ?);", messageID, tagID)
	if err != nil {
		return fmt.Errorf("attach tag %q to message %q: %w", tagID, messageID, err)
	}
	return nil
}

// RemoveMessageTag detaches a tag from a message.
func (s *Store) RemoveMessageTag(ctx context.Context, messageID, tagID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM message_tag WHERE message_id = ? AND tag_id = ?;", messageID, tagID)
	if err != nil {
		return fmt.Errorf("detach tag %q from message %q: %w", tagID, messageID, err)
	}
	return nil
}

// scanTags reads a set of tag rows (id, name, colour) into validated domain tags.
func scanTags(rows *sql.Rows) ([]domain.Tag, error) {
	var tags []domain.Tag
	for rows.Next() {
		var id, name, colourHex string
		if err := rows.Scan(&id, &name, &colourHex); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		colour, err := domain.NewColour(colourHex)
		if err != nil {
			return nil, fmt.Errorf("rebuild tag %q colour: %w", id, err)
		}
		tag, err := domain.NewTag(id, name, colour)
		if err != nil {
			return nil, fmt.Errorf("rebuild tag %q: %w", id, err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return tags, nil
}
