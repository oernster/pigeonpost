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
	return s.setFlag(ctx, messageID, domain.FlagSeen, seen)
}

// SetFlagged sets or clears the Flagged (starred) flag on a cached message.
func (s *Store) SetFlagged(ctx context.Context, messageID string, flagged bool) error {
	return s.setFlag(ctx, messageID, domain.FlagFlagged, flagged)
}

// SetAnswered sets or clears the Answered flag on a cached message, driving the replied indicator so the row
// shows it at once without waiting for a resync.
func (s *Store) SetAnswered(ctx context.Context, messageID string, answered bool) error {
	return s.setFlag(ctx, messageID, domain.FlagAnswered, answered)
}

// SetForwarded sets or clears the Forwarded flag on a cached message, driving the forwarded indicator.
func (s *Store) SetForwarded(ctx context.Context, messageID string, forwarded bool) error {
	return s.setFlag(ctx, messageID, domain.FlagForwarded, forwarded)
}

// setFlag reads a cached message's flag bitmask, toggles one flag and writes it back, in a transaction.
func (s *Store) setFlag(ctx context.Context, messageID string, flag domain.Flag, set bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var raw int
		err := tx.QueryRowContext(ctx, "SELECT flags FROM message WHERE id = ?;", messageID).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("set flag: message %q not found", messageID)
		}
		if err != nil {
			return fmt.Errorf("set flag: read flags: %w", err)
		}
		flags := domain.NewFlags(domain.Flag(raw))
		if set {
			flags = flags.With(flag)
		} else {
			flags = flags.Without(flag)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE message SET flags = ? WHERE id = ?;", int(flags.Raw()), messageID); err != nil {
			return fmt.Errorf("set flag: update %q: %w", messageID, err)
		}
		return nil
	})
}
