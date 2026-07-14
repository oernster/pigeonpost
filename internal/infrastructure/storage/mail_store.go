package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Folder unread and total counts are computed live from the cached messages rather than read from the
// stored folder.unread / folder.total columns, because servers report those inconsistently (unread
// arrives as 0 from a plain LIST); the local message cache is the reliable source. Both are counted
// from the same table so total is always the superset of unread, satisfying the domain invariant that
// unread never exceeds total. Each subquery refers to the folder as f in the surrounding query.
var (
	// unreadCountExpr counts a folder's unread messages: those whose Seen bit is clear. The bit value
	// is taken from the domain flag so the query can never drift from the domain definition of "read".
	unreadCountExpr = fmt.Sprintf(
		"(SELECT COUNT(*) FROM message m WHERE m.folder_id = f.id AND (m.flags & %d) = 0)",
		int(domain.FlagSeen))
	// totalCountExpr counts all of a folder's cached messages.
	totalCountExpr = "(SELECT COUNT(*) FROM message m WHERE m.folder_id = f.id)"
)

// ListFolders returns the cached folders for an account, ordered by path. Each folder's unread and
// total counts are computed live from the cached messages rather than read from the stored columns.
func (s *Store) ListFolders(ctx context.Context, accountID string) ([]domain.Folder, error) {
	return queryRows(ctx, s.db, "folders", fmt.Sprintf(
		`SELECT f.id, f.account_id, f.path, f.separator, f.kind, %s AS unread, %s AS total
		 FROM folder f WHERE f.account_id = ? ORDER BY f.path;`, unreadCountExpr, totalCountExpr),
		func(row scanner) (domain.Folder, error) {
			var (
				id, accID, path, sep string
				kind, unread, total  int
			)
			if err := row.Scan(&id, &accID, &path, &sep, &kind, &unread, &total); err != nil {
				return domain.Folder{}, fmt.Errorf("scan folder: %w", err)
			}
			folder, err := domain.NewFolderWithSeparator(id, accID, path, sep, domain.FolderKind(kind), unread, total)
			if err != nil {
				return domain.Folder{}, fmt.Errorf("rebuild folder %q: %w", id, err)
			}
			return folder, nil
		}, accountID)
}

// SaveFolders replaces the cached folder set for an account in a single transaction.
func (s *Store) SaveFolders(ctx context.Context, accountID string, folders []domain.Folder) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM folder WHERE account_id = ?;", accountID); err != nil {
			return fmt.Errorf("clear folders: %w", err)
		}
		for _, f := range folders {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO folder (id, account_id, path, separator, kind, unread, total)
				 VALUES (?, ?, ?, ?, ?, ?, ?);`,
				f.ID(), f.AccountID(), f.Path(), f.Separator(), int(f.Kind()), f.Unread(), f.Total()); err != nil {
				return fmt.Errorf("insert folder %q: %w", f.ID(), err)
			}
		}
		return nil
	})
}

// ListMessages returns the cached message summaries for a folder, newest first.
func (s *Store) ListMessages(ctx context.Context, folderID string) ([]domain.MessageSummary, error) {
	return queryRows(ctx, s.db, "messages",
		`SELECT id, folder_id, uid, message_id, from_display, from_address, to_json, cc_json, subject,
		        date_ms, size, flags, has_attachments, snippet
		 FROM message WHERE folder_id = ? ORDER BY date_ms DESC;`, scanMessage, folderID)
}

// ListMessagesPage returns one keyset page of a folder's cached message summaries, ordered by date then
// id (newest first or oldest first when ascending). The first page passes hasCursor false; each later
// page passes the date and id of the previous page's last row so the walk resumes strictly after it. It
// reads at most limit rows, letting the reading list load a huge folder incrementally instead of all at
// once. The (date_ms, id) tie-break gives a total order, so no row is skipped or repeated when several
// share a timestamp.
func (s *Store) ListMessagesPage(ctx context.Context, folderID string, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool) ([]domain.MessageSummary, error) {
	return s.listMessagesPage(ctx, folderID, hasCursor, cursorDateMs, cursorID, limit, ascending, false, 0)
}

// DeleteMessage removes a cached message and everything derived from it (body, tags, index row) in a
// single transaction.
func (s *Store) DeleteMessage(ctx context.Context, messageID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, stmt := range []string{
			"DELETE FROM message_body WHERE message_id = ?;",
			"DELETE FROM message_attachment WHERE message_id = ?;",
			"DELETE FROM message_tag WHERE message_id = ?;",
			"DELETE FROM message_tag_pending WHERE message_id = ?;",
			"DELETE FROM message_search WHERE message_id = ?;",
			"DELETE FROM message_snooze WHERE message_id = ?;",
			"DELETE FROM message WHERE id = ?;",
		} {
			if _, err := tx.ExecContext(ctx, stmt, messageID); err != nil {
				return fmt.Errorf("delete message %q: %w", messageID, err)
			}
		}
		return nil
	})
}

// GetMessage returns a single cached message summary by its local id.
func (s *Store) GetMessage(ctx context.Context, messageID string) (domain.MessageSummary, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, folder_id, uid, message_id, from_display, from_address, to_json, cc_json, subject,
		        date_ms, size, flags, has_attachments, snippet
		 FROM message WHERE id = ?;`, messageID)
	msg, err := scanMessage(row)
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("get message %q: %w", messageID, err)
	}
	return msg, nil
}

// GetFolder returns a single cached folder by its local id, with its unread and total counts computed
// live from the cached messages rather than read from the stored columns.
func (s *Store) GetFolder(ctx context.Context, folderID string) (domain.Folder, error) {
	var (
		id, accountID, path, sep string
		kind, unread, total      int
	)
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT f.id, f.account_id, f.path, f.separator, f.kind, %s AS unread, %s AS total FROM folder f WHERE f.id = ?;",
		unreadCountExpr, totalCountExpr), folderID).
		Scan(&id, &accountID, &path, &sep, &kind, &unread, &total)
	if err != nil {
		return domain.Folder{}, fmt.Errorf("get folder %q: %w", folderID, err)
	}
	folder, err := domain.NewFolderWithSeparator(id, accountID, path, sep, domain.FolderKind(kind), unread, total)
	if err != nil {
		return domain.Folder{}, fmt.Errorf("rebuild folder %q: %w", folderID, err)
	}
	return folder, nil
}

// UnreadByAccount returns each account's unread message count summed across all of its folders, keyed
// by account id. An account with no unread messages is absent from the map. Unread means the Seen bit
// is clear, matching the per-folder count. A message hidden by a snooze not yet due at visibleAt is
// not counted: hidden mail must not badge the folder it is hidden from.
func (s *Store) UnreadByAccount(ctx context.Context, visibleAt time.Time) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT f.account_id, COUNT(*) FROM message m JOIN folder f ON f.id = m.folder_id
		 WHERE (m.flags & %d) = 0
		   AND NOT EXISTS (SELECT 1 FROM message_snooze sn WHERE sn.message_id = m.id AND sn.until_ms > ?)
		 GROUP BY f.account_id;`, int(domain.FlagSeen)), visibleAt.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query unread by account: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var (
			accountID string
			count     int
		)
		if err := rows.Scan(&accountID, &count); err != nil {
			return nil, fmt.Errorf("scan unread count: %w", err)
		}
		counts[accountID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unread counts: %w", err)
	}
	return counts, nil
}

// SaveMessages replaces the cached message set for a folder in a single transaction, keeping the
// full-text index in step.
func (s *Store) SaveMessages(ctx context.Context, folderID string, messages []domain.MessageSummary) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		// Clear the index rows for this folder before the messages they mirror are removed.
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM message_search WHERE message_id IN (SELECT id FROM message WHERE folder_id = ?);",
			folderID); err != nil {
			return fmt.Errorf("clear message index: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message WHERE folder_id = ?;", folderID); err != nil {
			return fmt.Errorf("clear messages: %w", err)
		}
		for _, m := range messages {
			display, address := senderColumns(m.From())
			toJSON, err := marshalAddrs(m.To())
			if err != nil {
				return fmt.Errorf("encode recipients for %q: %w", m.ID(), err)
			}
			ccJSON, err := marshalAddrs(m.Cc())
			if err != nil {
				return fmt.Errorf("encode cc for %q: %w", m.ID(), err)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO message (id, folder_id, uid, message_id, from_display, from_address,
				        to_json, cc_json, subject, date_ms, size, flags, has_attachments, snippet)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
				m.ID(), m.FolderID(), m.UID(), m.MessageID(), display, address,
				toJSON, ccJSON, m.Subject(), m.Date().UnixMilli(), m.Size(), int(m.Flags().Raw()),
				boolToInt(m.HasAttachments()), m.Snippet()); err != nil {
				return fmt.Errorf("insert message %q: %w", m.ID(), err)
			}
		}
		// Index the replaced folder in one pass from the searchable-text view. A replaced message whose
		// body is already cached re-indexes with that body, so a re-sync never narrows what search covers.
		if _, err := tx.ExecContext(ctx,
			searchInsertSQL+" WHERE message_id IN (SELECT id FROM message WHERE folder_id = ?);",
			folderID); err != nil {
			return fmt.Errorf("index messages for folder %q: %w", folderID, err)
		}
		// A folder replace drops the rows of any message expunged on the server without going through
		// DeleteMessage, so it never cleans their pending tag ops. Sweep any that now have no message left,
		// so an expunged-while-pending message cannot leak an unreachable, forever-retried pending row.
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM message_tag_pending WHERE message_id NOT IN (SELECT id FROM message);"); err != nil {
			return fmt.Errorf("sweep orphaned pending tags: %w", err)
		}
		// The same sweep for snoozes: a snoozed message expunged (or moved, which changes its id) on the
		// server leaves its snooze orphaned; drop it so it neither lingers nor resurfaces as a ghost.
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM message_snooze WHERE message_id NOT IN (SELECT id FROM message);"); err != nil {
			return fmt.Errorf("sweep orphaned snoozes: %w", err)
		}
		return nil
	})
}

// messageRow holds one scanned message-summary row before its domain rebuild, shared by the plain
// message scanner and the search-hit scanner (which reads the same columns plus a snippet).
type messageRow struct {
	id, folderID, uid, messageID  string
	fromDisplay, fromAddress      string
	toJSON, ccJSON                string
	subject, snippet              string
	dateMS                        int64
	size, flags, hasAttachmentInt int
}

// scanFields returns the scan destinations in the shared summary column order.
func (r *messageRow) scanFields() []any {
	return []any{&r.id, &r.folderID, &r.uid, &r.messageID, &r.fromDisplay, &r.fromAddress, &r.toJSON,
		&r.ccJSON, &r.subject, &r.dateMS, &r.size, &r.flags, &r.hasAttachmentInt, &r.snippet}
}

// build rebuilds the validated domain summary from the scanned columns.
func (r *messageRow) build() (domain.MessageSummary, error) {
	var from domain.EmailAddress
	if r.fromAddress != "" {
		parsed, err := domain.NewEmailAddress(r.fromDisplay, r.fromAddress)
		if err != nil {
			return domain.MessageSummary{}, fmt.Errorf("rebuild sender for %q: %w", r.id, err)
		}
		from = parsed
	}
	to, err := unmarshalAddrs(r.toJSON)
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("rebuild recipients for %q: %w", r.id, err)
	}
	cc, err := unmarshalAddrs(r.ccJSON)
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("rebuild cc for %q: %w", r.id, err)
	}
	message, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: r.id, FolderID: r.folderID, UID: r.uid, MessageID: r.messageID, From: from, To: to, Cc: cc,
		Subject: r.subject, Date: time.UnixMilli(r.dateMS).UTC(), Size: r.size,
		Flags: domain.NewFlags(domain.Flag(r.flags)), HasAttachments: r.hasAttachmentInt != 0,
		Snippet: r.snippet,
	})
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("rebuild message %q: %w", r.id, err)
	}
	return message, nil
}

func scanMessage(row scanner) (domain.MessageSummary, error) {
	var r messageRow
	if err := row.Scan(r.scanFields()...); err != nil {
		return domain.MessageSummary{}, fmt.Errorf("scan message: %w", err)
	}
	return r.build()
}

func senderColumns(from domain.EmailAddress) (display, address string) {
	if from.IsZero() {
		return "", ""
	}
	return from.Display(), from.Address()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DeleteAccountData removes an account's cached folders and the messages within them, in one
// transaction. The messages are deleted first so no rows are orphaned if the folder delete fails.
func (s *Store) DeleteAccountData(ctx context.Context, accountID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		bodyOrTagFilter := `message_id IN (
			SELECT id FROM message WHERE folder_id IN (SELECT id FROM folder WHERE account_id = ?)
		)`
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_body WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear cached message bodies: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_attachment WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear cached message attachments: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_tag WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear cached message tags: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_tag_pending WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear pending message tags: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_search WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear message index: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_snooze WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear message snoozes: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM message WHERE folder_id IN (SELECT id FROM folder WHERE account_id = ?);`,
			accountID); err != nil {
			return fmt.Errorf("clear cached messages: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM folder WHERE account_id = ?;", accountID); err != nil {
			return fmt.Errorf("clear cached folders: %w", err)
		}
		return nil
	})
}

func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
