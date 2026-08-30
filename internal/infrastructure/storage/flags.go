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
//
// The change is applied to every cached copy of the same message in that account, not just the row the
// caller named. A server can present one message in several mailboxes: Gmail does it for every label,
// where a message with the Work label sits in Work, in the Inbox and in All Mail as three rows with three
// UIDs. Reading it in one place used to leave the others bold in the cache until each folder happened to
// be opened and synced, so the app showed mail as unread that the user had just read.
//
// Sameness is (message_id, date_ms, from_address, subject) within the one account. Message-ID alone is
// not enough: measured across two real accounts, more than 900 messages share one with another row. Those
// collisions were checked rather than assumed and they are the same message stored twice, differing only
// in UID and stored size, so updating each of them is right. An empty Message-ID matches nothing, since
// otherwise every message lacking one would be treated as the same message.
//
// Only the cache is propagated, never the pending intent: the intent belongs to the message the caller
// named. Where a server does share flags between the copies, as Gmail does, its own push covers them all;
// where one does not, the next sync of that folder replaces its rows and the server's truth wins.
func (s *Store) SetFlag(ctx context.Context, messageID string, flag domain.Flag, value bool, recordPending bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var (
			raw                                 int
			dateMS                              int64
			msgID, fromAddress, subject, folder string
		)
		err := tx.QueryRowContext(ctx,
			"SELECT flags, message_id, date_ms, from_address, subject, folder_id FROM message WHERE id = ?;",
			messageID).Scan(&raw, &msgID, &dateMS, &fromAddress, &subject, &folder)
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
		if msgID != "" {
			if _, err := tx.ExecContext(ctx,
				`UPDATE message SET flags = ?
				 WHERE id != ?
				   AND message_id = ? AND date_ms = ? AND from_address = ? AND subject = ?
				   AND folder_id IN (SELECT id FROM folder WHERE account_id =
				         (SELECT account_id FROM folder WHERE id = ?));`,
				int(flags.Raw()), messageID, msgID, dateMS, fromAddress, subject, folder); err != nil {
				return fmt.Errorf("set flag: update other copies of %q: %w", messageID, err)
			}
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
