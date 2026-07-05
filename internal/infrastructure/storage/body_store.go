package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// GetMessageBody returns the cached full body for a message, or application.ErrBodyNotCached when the
// body has not been fetched yet.
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
	return body.WithInvite([]byte(invite)), nil
}

// SaveMessageBody inserts or replaces a cached message body, including any text/calendar payload.
func (s *Store) SaveMessageBody(ctx context.Context, body domain.MessageBody) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO message_body (message_id, plain, html, invite) VALUES (?, ?, ?, ?);",
		body.MessageID(), body.Plain(), body.HTML(), string(body.Invite()))
	if err != nil {
		return fmt.Errorf("save message body %q: %w", body.MessageID(), err)
	}
	return nil
}
