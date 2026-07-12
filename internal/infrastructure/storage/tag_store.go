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
	return queryRows(ctx, s.db, "tags", "SELECT id, name, colour, keyword FROM tag ORDER BY name;", scanTag)
}

// SaveTag inserts or replaces a tag, including its frozen keyword.
func (s *Store) SaveTag(ctx context.Context, tag domain.Tag) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO tag (id, name, colour, keyword) VALUES (?, ?, ?, ?);",
		tag.ID(), tag.Name(), tag.Colour().Hex(), tag.Keyword())
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
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_tag_pending WHERE tag_id = ?;", id); err != nil {
			return fmt.Errorf("clear pending tag ops: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM tag WHERE id = ?;", id); err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}
		return nil
	})
}

// TagsForMessage returns the tags attached to a message, ordered by name.
func (s *Store) TagsForMessage(ctx context.Context, messageID string) ([]domain.Tag, error) {
	return queryRows(ctx, s.db, "message tags",
		`SELECT t.id, t.name, t.colour, t.keyword FROM tag t
		 JOIN message_tag mt ON mt.tag_id = t.id
		 WHERE mt.message_id = ? ORDER BY t.name;`, scanTag, messageID)
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

// AssignMessageTag attaches a tag to a message and (when recordPending is true, for an IMAP account) records
// the pending intent to round it onto the server, in one transaction so the local change and its intent can
// never drift apart (which would let a later reconcile mistake the tag for a server-cleared one and delete
// it). Re-attaching an existing pair is a no-op.
func (s *Store) AssignMessageTag(ctx context.Context, messageID, tagID string, recordPending bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO message_tag (message_id, tag_id) VALUES (?, ?);", messageID, tagID); err != nil {
			return fmt.Errorf("attach tag %q to message %q: %w", tagID, messageID, err)
		}
		if recordPending {
			if _, err := tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO message_tag_pending (message_id, tag_id, assigned) VALUES (?, ?, ?);",
				messageID, tagID, boolToInt(true)); err != nil {
				return fmt.Errorf("record pending assign for message %q tag %q: %w", messageID, tagID, err)
			}
		}
		return nil
	})
}

// UnassignMessageTag detaches a tag from a message and (when recordPending is true, for an IMAP account)
// records the pending intent to remove it on the server, in one transaction.
func (s *Store) UnassignMessageTag(ctx context.Context, messageID, tagID string, recordPending bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM message_tag WHERE message_id = ? AND tag_id = ?;", messageID, tagID); err != nil {
			return fmt.Errorf("detach tag %q from message %q: %w", tagID, messageID, err)
		}
		if recordPending {
			if _, err := tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO message_tag_pending (message_id, tag_id, assigned) VALUES (?, ?, ?);",
				messageID, tagID, boolToInt(false)); err != nil {
				return fmt.Errorf("record pending unassign for message %q tag %q: %w", messageID, tagID, err)
			}
		}
		return nil
	})
}

// SetPendingTagOp records the intended assigned/removed state of a (message, tag) pair that has not yet
// been confirmed on the server, replacing any existing intent for that pair.
func (s *Store) SetPendingTagOp(ctx context.Context, messageID, tagID string, assigned bool) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO message_tag_pending (message_id, tag_id, assigned) VALUES (?, ?, ?);",
		messageID, tagID, boolToInt(assigned))
	if err != nil {
		return fmt.Errorf("record pending tag op for message %q tag %q: %w", messageID, tagID, err)
	}
	return nil
}

// ClearPendingTagOp removes the pending intent for a (message, tag) pair, called once the server agrees.
func (s *Store) ClearPendingTagOp(ctx context.Context, messageID, tagID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM message_tag_pending WHERE message_id = ? AND tag_id = ?;", messageID, tagID)
	if err != nil {
		return fmt.Errorf("clear pending tag op for message %q tag %q: %w", messageID, tagID, err)
	}
	return nil
}

// PendingTagOps returns the pending intents for one message, keyed by tag id (true for a pending
// assignment, false for a pending removal).
func (s *Store) PendingTagOps(ctx context.Context, messageID string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT tag_id, assigned FROM message_tag_pending WHERE message_id = ?;", messageID)
	if err != nil {
		return nil, fmt.Errorf("query pending tag ops for message %q: %w", messageID, err)
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var tagID string
		var assigned int
		if err := rows.Scan(&tagID, &assigned); err != nil {
			return nil, fmt.Errorf("scan pending tag op: %w", err)
		}
		result[tagID] = assigned != 0
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending tag ops: %w", err)
	}
	return result, nil
}

// ListPendingTagOps returns every pending tag operation across all messages, used to replay unsynced
// intents to the server on a sync.
func (s *Store) ListPendingTagOps(ctx context.Context) ([]domain.PendingTagOp, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT message_id, tag_id, assigned FROM message_tag_pending;")
	if err != nil {
		return nil, fmt.Errorf("query pending tag ops: %w", err)
	}
	defer rows.Close()
	ops := make([]domain.PendingTagOp, 0)
	for rows.Next() {
		var messageID, tagID string
		var assigned int
		if err := rows.Scan(&messageID, &tagID, &assigned); err != nil {
			return nil, fmt.Errorf("scan pending tag op: %w", err)
		}
		ops = append(ops, domain.NewPendingTagOp(messageID, tagID, assigned != 0))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending tag ops: %w", err)
	}
	return ops, nil
}

// scanTag reads one tag row (id, name, colour, keyword) into a validated domain tag.
func scanTag(row scanner) (domain.Tag, error) {
	var id, name, colourHex, keyword string
	if err := row.Scan(&id, &name, &colourHex, &keyword); err != nil {
		return domain.Tag{}, fmt.Errorf("scan tag: %w", err)
	}
	colour, err := domain.NewColour(colourHex)
	if err != nil {
		return domain.Tag{}, fmt.Errorf("rebuild tag %q colour: %w", id, err)
	}
	tag, err := domain.NewTag(id, name, colour, keyword)
	if err != nil {
		return domain.Tag{}, fmt.Errorf("rebuild tag %q: %w", id, err)
	}
	return tag, nil
}
