package storage

// Snooze state and the snooze-aware (visible) listing variants. A message with a message_snooze row is
// hidden from the visible listings until its until_ms passes, then reappears untouched; the plain
// listings in mail_store.go see everything (the sync's known-message sets and flag carry-over need
// snoozed rows). Kept apart from mail_store.go so each file stays within the module-size limit.

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// messageColumns is the shared summary column list of the message table, unaliased.
const messageColumns = `id, folder_id, uid, message_id, from_display, from_address, to_json, cc_json,
                        subject, date_ms, size, flags, has_attachments, snippet`

// snoozeHiddenFilter excludes messages hidden by an unexpired snooze; the placeholder is the visible-at
// instant in Unix milliseconds. It names the message table directly, so it composes with any query
// whose rows come FROM message unaliased.
const snoozeHiddenFilter = ` AND NOT EXISTS (
    SELECT 1 FROM message_snooze sn WHERE sn.message_id = message.id AND sn.until_ms > ?)`

// ListMessagesVisible returns a folder's cached message summaries, newest first, excluding messages
// hidden by a snooze that has not yet come due at visibleAt.
func (s *Store) ListMessagesVisible(ctx context.Context, folderID string, visibleAt time.Time) ([]domain.MessageSummary, error) {
	return queryRows(ctx, s.db, "messages", fmt.Sprintf(
		`SELECT %s FROM message WHERE folder_id = ?%s ORDER BY date_ms DESC;`,
		messageColumns, snoozeHiddenFilter),
		scanMessage, folderID, visibleAt.UnixMilli())
}

// ListMessagesPageVisible is ListMessagesPage with snoozed-and-not-yet-due messages excluded, for the
// reading list. The cursor mechanics and the (date_ms, id) total order are identical, so the walk still
// never skips or repeats a row.
func (s *Store) ListMessagesPageVisible(ctx context.Context, folderID string, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool, visibleAt time.Time) ([]domain.MessageSummary, error) {
	return s.listMessagesPage(ctx, folderID, hasCursor, cursorDateMs, cursorID, limit, ascending, true, visibleAt.UnixMilli())
}

// listMessagesPage is the shared keyset page query behind ListMessagesPage and ListMessagesPageVisible:
// ordered by (date_ms, id) in either direction, resuming strictly after the cursor when one is given,
// reading at most limit rows, and excluding snooze-hidden rows when hideSnoozed is set.
func (s *Store) listMessagesPage(ctx context.Context, folderID string, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool, hideSnoozed bool, visibleAtMs int64) ([]domain.MessageSummary, error) {
	order, cmp := "DESC", "<"
	if ascending {
		order, cmp = "ASC", ">"
	}
	where := "folder_id = ?"
	args := []any{folderID}
	if hasCursor {
		where += fmt.Sprintf(" AND (date_ms %s ? OR (date_ms = ? AND id %s ?))", cmp, cmp)
		args = append(args, cursorDateMs, cursorDateMs, cursorID)
	}
	if hideSnoozed {
		where += snoozeHiddenFilter
		args = append(args, visibleAtMs)
	}
	args = append(args, limit)
	query := fmt.Sprintf(`SELECT %s FROM message WHERE %s ORDER BY date_ms %s, id %s LIMIT ?;`,
		messageColumns, where, order, order)
	return queryRows(ctx, s.db, "messages", query, scanMessage, args...)
}

// SetSnooze hides a message until the given instant, replacing any snooze it already carries.
func (s *Store) SetSnooze(ctx context.Context, messageID string, until time.Time) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO message_snooze (message_id, until_ms) VALUES (?, ?)
		 ON CONFLICT(message_id) DO UPDATE SET until_ms = excluded.until_ms;`,
		messageID, until.UnixMilli()); err != nil {
		return fmt.Errorf("set snooze for %q: %w", messageID, err)
	}
	return nil
}

// ClearSnooze removes a message's snooze, so it is visible again at once. Clearing a message that
// carries no snooze is a no-op.
func (s *Store) ClearSnooze(ctx context.Context, messageID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM message_snooze WHERE message_id = ?;", messageID); err != nil {
		return fmt.Errorf("clear snooze for %q: %w", messageID, err)
	}
	return nil
}

// ListSnoozed returns every snoozed message with its due instant and owning account, soonest first,
// for the Snoozed view. A snooze orphaned by its message's deletion has no row to show and is simply
// absent; a missing folder row leaves the account id empty rather than dropping the message.
func (s *Store) ListSnoozed(ctx context.Context) ([]application.SnoozedMessage, error) {
	return queryRows(ctx, s.db, "snoozed messages", fmt.Sprintf(
		`SELECT %s, sn.until_ms, COALESCE(f.account_id, '')
		 FROM message m
		 JOIN message_snooze sn ON sn.message_id = m.id
		 LEFT JOIN folder f ON f.id = m.folder_id
		 ORDER BY sn.until_ms;`, aliasColumns("m")),
		func(row scanner) (application.SnoozedMessage, error) {
			var (
				r         messageRow
				untilMs   int64
				accountID string
			)
			if err := row.Scan(append(r.scanFields(), &untilMs, &accountID)...); err != nil {
				return application.SnoozedMessage{}, fmt.Errorf("scan snoozed message: %w", err)
			}
			summary, err := r.build()
			if err != nil {
				return application.SnoozedMessage{}, err
			}
			return application.SnoozedMessage{
				Summary:   summary,
				Until:     time.UnixMilli(untilMs).UTC(),
				AccountID: accountID,
			}, nil
		})
}

// PopDueSnoozed removes every snooze whose instant has passed and returns the messages that just
// resurfaced, soonest first, in one transaction so a resurfaced message can never be announced twice.
// A due snooze orphaned by its message's deletion is removed without being returned.
func (s *Store) PopDueSnoozed(ctx context.Context, now time.Time) ([]domain.MessageSummary, error) {
	nowMs := now.UnixMilli()
	var resurfaced []domain.MessageSummary
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, fmt.Sprintf(
			`SELECT %s FROM message m JOIN message_snooze sn ON sn.message_id = m.id
			 WHERE sn.until_ms <= ? ORDER BY sn.until_ms;`, aliasColumns("m")), nowMs)
		if err != nil {
			return fmt.Errorf("query due snoozes: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			message, err := scanMessage(rows)
			if err != nil {
				return err
			}
			resurfaced = append(resurfaced, message)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate due snoozes: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_snooze WHERE until_ms <= ?;", nowMs); err != nil {
			return fmt.Errorf("clear due snoozes: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resurfaced, nil
}

// NextSnooze returns the earliest pending snooze instant and whether one exists.
func (s *Store) NextSnooze(ctx context.Context) (time.Time, bool, error) {
	var untilMs int64
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MIN(until_ms), 0) FROM message_snooze;").Scan(&untilMs)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("query next snooze: %w", err)
	}
	if untilMs == 0 {
		return time.Time{}, false, nil
	}
	return time.UnixMilli(untilMs).UTC(), true, nil
}

// aliasColumns prefixes each shared summary column with a table alias, for queries that join message.
func aliasColumns(alias string) string {
	return fmt.Sprintf(`%[1]s.id, %[1]s.folder_id, %[1]s.uid, %[1]s.message_id, %[1]s.from_display,
	       %[1]s.from_address, %[1]s.to_json, %[1]s.cc_json, %[1]s.subject, %[1]s.date_ms, %[1]s.size,
	       %[1]s.flags, %[1]s.has_attachments, %[1]s.snippet`, alias)
}
