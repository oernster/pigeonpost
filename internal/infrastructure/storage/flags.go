package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// SetSeen sets or clears the Seen flag on a cached message, driving the read/unread (bold) state.
func (s *Store) SetSeen(ctx context.Context, messageID string, seen bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var raw int
		err := tx.QueryRowContext(ctx, "SELECT flags FROM message WHERE id = ?;", messageID).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("set seen: message %q not found", messageID)
		}
		if err != nil {
			return fmt.Errorf("set seen: read flags: %w", err)
		}
		flags := domain.NewFlags(domain.Flag(raw))
		if seen {
			flags = flags.With(domain.FlagSeen)
		} else {
			flags = flags.Without(domain.FlagSeen)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE message SET flags = ? WHERE id = ?;", int(flags.Raw()), messageID); err != nil {
			return fmt.Errorf("set seen: update %q: %w", messageID, err)
		}
		return nil
	})
}
