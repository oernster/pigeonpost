package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// GetMessageBody returns the cached full body for a message, including any text/calendar payload and its
// attachments, or application.ErrBodyNotCached when the body has not been fetched yet.
func (s *Store) GetMessageBody(ctx context.Context, messageID string) (domain.MessageBody, error) {
	var plain, html, invite string
	err := s.db.QueryRowContext(ctx,
		"SELECT plain, html, invite FROM message_body WHERE message_id = ?;", messageID).Scan(&plain, &html, &invite)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MessageBody{}, application.ErrBodyNotCached
	}
	if err != nil {
		return domain.MessageBody{}, fmt.Errorf("query message body %q: %w", messageID, err)
	}
	body, err := domain.NewMessageBody(messageID, plain, html)
	if err != nil {
		return domain.MessageBody{}, fmt.Errorf("rebuild message body %q: %w", messageID, err)
	}
	attachments, err := s.messageAttachments(ctx, messageID)
	if err != nil {
		return domain.MessageBody{}, err
	}
	return body.WithInvite([]byte(invite)).WithAttachments(attachments), nil
}

// messageAttachments loads a message's cached attachments, ordered as the sender arranged them.
func (s *Store) messageAttachments(ctx context.Context, messageID string) ([]domain.Attachment, error) {
	return queryRows(ctx, s.db, fmt.Sprintf("message attachments %q", messageID),
		`SELECT filename, content_type, content FROM message_attachment
		 WHERE message_id = ? ORDER BY position ASC;`,
		func(row scanner) (domain.Attachment, error) {
			var (
				filename, contentType string
				content               []byte
			)
			if err := row.Scan(&filename, &contentType, &content); err != nil {
				return domain.Attachment{}, fmt.Errorf("scan message attachment %q: %w", messageID, err)
			}
			attachment, err := domain.NewAttachment(filename, contentType, content)
			if err != nil {
				return domain.Attachment{}, fmt.Errorf("rebuild message attachment %q: %w", messageID, err)
			}
			return attachment, nil
		}, messageID)
}

// SaveMessageBody inserts or replaces a cached message body, including any text/calendar payload and its
// attachments. The body row and its attachment rows are written in one transaction so a cached body is
// never left with a stale attachment set.
func (s *Store) SaveMessageBody(ctx context.Context, body domain.MessageBody) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO message_body (message_id, plain, html, invite) VALUES (?, ?, ?, ?);",
			body.MessageID(), body.Plain(), body.HTML(), string(body.Invite())); err != nil {
			return fmt.Errorf("save message body %q: %w", body.MessageID(), err)
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM message_attachment WHERE message_id = ?;", body.MessageID()); err != nil {
			return fmt.Errorf("clear message attachments %q: %w", body.MessageID(), err)
		}
		for position, attachment := range body.Attachments() {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO message_attachment (message_id, position, filename, content_type, content)
				 VALUES (?, ?, ?, ?, ?);`,
				body.MessageID(), position, attachment.Filename(), attachment.ContentType(), attachment.Content()); err != nil {
				return fmt.Errorf("save message attachment %q: %w", body.MessageID(), err)
			}
		}
		// Re-index the message now that its body and attachment filenames are cached, in the same
		// transaction, so search coverage widens in lockstep with the cache. A body cached for a message
		// no longer in the summary table indexes nothing, which keeps the index free of dangling hits.
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM message_search WHERE message_id = ?;", body.MessageID()); err != nil {
			return fmt.Errorf("clear message index %q: %w", body.MessageID(), err)
		}
		if _, err := tx.ExecContext(ctx,
			searchInsertSQL+" WHERE message_id = ?;", body.MessageID()); err != nil {
			return fmt.Errorf("reindex message %q: %w", body.MessageID(), err)
		}
		return nil
	})
}
