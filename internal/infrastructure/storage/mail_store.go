package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListFolders returns the cached folders for an account, ordered by path.
func (s *Store) ListFolders(ctx context.Context, accountID string) ([]domain.Folder, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, account_id, path, kind, unread, total FROM folder WHERE account_id = ? ORDER BY path;",
		accountID)
	if err != nil {
		return nil, fmt.Errorf("query folders: %w", err)
	}
	defer rows.Close()

	var folders []domain.Folder
	for rows.Next() {
		var (
			id, accID, path     string
			kind, unread, total int
		)
		if err := rows.Scan(&id, &accID, &path, &kind, &unread, &total); err != nil {
			return nil, fmt.Errorf("scan folder: %w", err)
		}
		folder, err := domain.NewFolder(id, accID, path, domain.FolderKind(kind), unread, total)
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
				`INSERT INTO folder (id, account_id, path, kind, unread, total)
				 VALUES (?, ?, ?, ?, ?, ?);`,
				f.ID(), f.AccountID(), f.Path(), int(f.Kind()), f.Unread(), f.Total()); err != nil {
				return fmt.Errorf("insert folder %q: %w", f.ID(), err)
			}
		}
		return nil
	})
}

// ListMessages returns the cached message summaries for a folder, newest first.
func (s *Store) ListMessages(ctx context.Context, folderID string) ([]domain.MessageSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, folder_id, uid, message_id, from_display, from_address, subject,
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

// SaveMessages replaces the cached message set for a folder in a single transaction.
func (s *Store) SaveMessages(ctx context.Context, folderID string, messages []domain.MessageSummary) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM message WHERE folder_id = ?;", folderID); err != nil {
			return fmt.Errorf("clear messages: %w", err)
		}
		for _, m := range messages {
			display, address := senderColumns(m.From())
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO message (id, folder_id, uid, message_id, from_display, from_address,
				        subject, date_ms, size, flags, has_attachments, snippet)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
				m.ID(), m.FolderID(), m.UID(), m.MessageID(), display, address,
				m.Subject(), m.Date().UnixMilli(), m.Size(), int(m.Flags().Raw()),
				boolToInt(m.HasAttachments()), m.Snippet()); err != nil {
				return fmt.Errorf("insert message %q: %w", m.ID(), err)
			}
		}
		return nil
	})
}

func scanMessage(row scanner) (domain.MessageSummary, error) {
	var (
		id, folderID, messageID       string
		fromDisplay, fromAddress      string
		subject, snippet              string
		uid                           uint32
		dateMS                        int64
		size, flags, hasAttachmentInt int
	)
	if err := row.Scan(&id, &folderID, &uid, &messageID, &fromDisplay, &fromAddress,
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

	message, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: uid, MessageID: messageID, From: from,
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
