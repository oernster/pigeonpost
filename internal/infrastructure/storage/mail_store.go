package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT f.id, f.account_id, f.path, f.separator, f.kind, %s AS unread, %s AS total
		 FROM folder f WHERE f.account_id = ? ORDER BY f.path;`, unreadCountExpr, totalCountExpr),
		accountID)
	if err != nil {
		return nil, fmt.Errorf("query folders: %w", err)
	}
	defer rows.Close()

	var folders []domain.Folder
	for rows.Next() {
		var (
			id, accID, path, sep string
			kind, unread, total  int
		)
		if err := rows.Scan(&id, &accID, &path, &sep, &kind, &unread, &total); err != nil {
			return nil, fmt.Errorf("scan folder: %w", err)
		}
		folder, err := domain.NewFolderWithSeparator(id, accID, path, sep, domain.FolderKind(kind), unread, total)
		if err != nil {
			return nil, fmt.Errorf("rebuild folder %q: %w", id, err)
		}
		folders = append(folders, folder)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate folders: %w", err)
	}
	return folders, nil
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
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, folder_id, uid, message_id, from_display, from_address, to_json, cc_json, subject,
		        date_ms, size, flags, has_attachments, snippet
		 FROM message WHERE folder_id = ? ORDER BY date_ms DESC;`, folderID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []domain.MessageSummary
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}
	return messages, nil
}

// DeleteMessage removes a cached message and everything derived from it (body, tags, index row) in a
// single transaction.
func (s *Store) DeleteMessage(ctx context.Context, messageID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, stmt := range []string{
			"DELETE FROM message_body WHERE message_id = ?;",
			"DELETE FROM message_tag WHERE message_id = ?;",
			"DELETE FROM message_fts WHERE message_id = ?;",
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
// is clear, matching the per-folder count.
func (s *Store) UnreadByAccount(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT f.account_id, COUNT(*) FROM message m JOIN folder f ON f.id = m.folder_id
		 WHERE (m.flags & %d) = 0 GROUP BY f.account_id;`, int(domain.FlagSeen)))
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
			"DELETE FROM message_fts WHERE message_id IN (SELECT id FROM message WHERE folder_id = ?);",
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
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO message_fts (message_id, subject, snippet, from_address) VALUES (?, ?, ?, ?);",
				m.ID(), m.Subject(), m.Snippet(), address); err != nil {
				return fmt.Errorf("index message %q: %w", m.ID(), err)
			}
		}
		return nil
	})
}

// SearchMessages returns the cached messages matching a free-text query across subject, sender and
// snippet, most relevant first. An empty query returns no results.
func (s *Store) SearchMessages(ctx context.Context, query string) ([]domain.MessageSummary, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.folder_id, m.uid, m.message_id, m.from_display, m.from_address, m.to_json,
		        m.cc_json, m.subject, m.date_ms, m.size, m.flags, m.has_attachments, m.snippet
		 FROM message m JOIN message_fts f ON f.message_id = m.id
		 WHERE message_fts MATCH ? ORDER BY rank;`, ftsQuery)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var messages []domain.MessageSummary
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}
	return messages, nil
}

// buildFTSQuery turns free user input into a safe FTS5 MATCH expression: each whitespace-separated
// term is quoted (so punctuation cannot be read as FTS syntax) and given a prefix wildcard, and the
// terms are combined with implicit AND.
func buildFTSQuery(raw string) string {
	fields := strings.Fields(raw)
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		cleaned := strings.ReplaceAll(field, `"`, "")
		if cleaned == "" {
			continue
		}
		terms = append(terms, `"`+cleaned+`"*`)
	}
	return strings.Join(terms, " ")
}

func scanMessage(row scanner) (domain.MessageSummary, error) {
	var (
		id, folderID, messageID       string
		fromDisplay, fromAddress      string
		toJSON, ccJSON                string
		subject, snippet              string
		uid                           string
		dateMS                        int64
		size, flags, hasAttachmentInt int
	)
	if err := row.Scan(&id, &folderID, &uid, &messageID, &fromDisplay, &fromAddress, &toJSON, &ccJSON,
		&subject, &dateMS, &size, &flags, &hasAttachmentInt, &snippet); err != nil {
		return domain.MessageSummary{}, fmt.Errorf("scan message: %w", err)
	}

	var from domain.EmailAddress
	if fromAddress != "" {
		parsed, err := domain.NewEmailAddress(fromDisplay, fromAddress)
		if err != nil {
			return domain.MessageSummary{}, fmt.Errorf("rebuild sender for %q: %w", id, err)
		}
		from = parsed
	}
	to, err := unmarshalAddrs(toJSON)
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("rebuild recipients for %q: %w", id, err)
	}
	cc, err := unmarshalAddrs(ccJSON)
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("rebuild cc for %q: %w", id, err)
	}

	message, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: uid, MessageID: messageID, From: from, To: to, Cc: cc,
		Subject: subject, Date: time.UnixMilli(dateMS).UTC(), Size: size,
		Flags: domain.NewFlags(domain.Flag(flags)), HasAttachments: hasAttachmentInt != 0,
		Snippet: snippet,
	})
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("rebuild message %q: %w", id, err)
	}
	return message, nil
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
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_tag WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear cached message tags: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM message_fts WHERE "+bodyOrTagFilter, accountID); err != nil {
			return fmt.Errorf("clear message index: %w", err)
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
