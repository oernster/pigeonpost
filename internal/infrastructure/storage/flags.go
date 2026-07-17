package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// SetFlag sets or clears one flag on a cached message: Seen drives the read/unread (bold) state, Flagged
// the star, Answered and Forwarded the replied and forwarded indicators. When recordPending is true the
// intended state is also recorded as a pending flag operation in the same transaction, so the local
// change and the intent to land it on the server cannot drift apart (mirroring AssignMessageTag): a
// change with no intent would be silently undone by the next sync mirroring the server's stale view.
func (s *Store) SetFlag(ctx context.Context, messageID string, flag domain.Flag, value bool, recordPending bool) error {
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
		if value {
			flags = flags.With(flag)
		} else {
			flags = flags.Without(flag)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE message SET flags = ? WHERE id = ?;", int(flags.Raw()), messageID); err != nil {
			return fmt.Errorf("set flag: update %q: %w", messageID, err)
		}
		if recordPending {
			if _, err := tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO message_flag_pending (message_id, flag, value) VALUES (?, ?, ?);",
				messageID, int(flag), boolToInt(value)); err != nil {
				return fmt.Errorf("set flag: record pending for %q: %w", messageID, err)
			}
		}
		return nil
	})
}

// ClearPendingFlagOp removes the pending intent for a (message, flag) pair, called once a sync sees the
// server agree with it.
func (s *Store) ClearPendingFlagOp(ctx context.Context, messageID string, flag domain.Flag) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM message_flag_pending WHERE message_id = ? AND flag = ?;", messageID, int(flag))
	if err != nil {
		return fmt.Errorf("clear pending flag op for message %q: %w", messageID, err)
	}
	return nil
}

// PendingFlagOps returns the pending intents for one message, keyed by flag (the value is the intended
// state), read during a sync reconcile to guard unconfirmed local changes against a stale server view.
func (s *Store) PendingFlagOps(ctx context.Context, messageID string) (map[domain.Flag]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT flag, value FROM message_flag_pending WHERE message_id = ?;", messageID)
	if err != nil {
		return nil, fmt.Errorf("query pending flag ops for message %q: %w", messageID, err)
	}
	defer rows.Close()
	result := map[domain.Flag]bool{}
	for rows.Next() {
		var flag, value int
		if err := rows.Scan(&flag, &value); err != nil {
			return nil, fmt.Errorf("scan pending flag op: %w", err)
		}
		result[domain.Flag(flag)] = value != 0
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending flag ops: %w", err)
	}
	return result, nil
}

// ListPendingFlagOps returns every pending flag operation across all messages, used to replay unsynced
// intents to the server on a sync.
func (s *Store) ListPendingFlagOps(ctx context.Context) ([]domain.PendingFlagOp, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT message_id, flag, value FROM message_flag_pending;")
	if err != nil {
		return nil, fmt.Errorf("query pending flag ops: %w", err)
	}
	defer rows.Close()
	ops := make([]domain.PendingFlagOp, 0)
	for rows.Next() {
		var messageID string
		var flag, value int
		if err := rows.Scan(&messageID, &flag, &value); err != nil {
			return nil, fmt.Errorf("scan pending flag op: %w", err)
		}
		ops = append(ops, domain.NewPendingFlagOp(messageID, domain.Flag(flag), value != 0))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending flag ops: %w", err)
	}
	return ops, nil
}
