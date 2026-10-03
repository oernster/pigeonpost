package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The per-folder baseline mark: whether a folder has completed the one sync that records what it
// already holds. The filter rules read it to tell a folder they have never seen from one the user has
// simply emptied, a distinction the rule engine cannot make from the cached messages alone, since both
// look like a folder with nothing in it.
//
// The mark is kept apart from the folder row on purpose. SaveFolders clears and rewrites every folder
// for an account on each sync, so a column there would be dropped and re-created every pass and the
// protection would re-arm forever.

// FolderBaselined reports whether the folder has had its baseline sync. A folder with no mark has not,
// so its next pass is the baseline: it records what the folder holds without acting destructively on it.
func (s *Store) FolderBaselined(ctx context.Context, folderID string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM folder_baseline WHERE folder_id = ?;", folderID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read folder baseline %q: %w", folderID, err)
	}
	return true, nil
}

// MarkFolderBaselined records that the folder has had its baseline sync. The caller marks it only once
// the fetched messages are safely saved: marking a folder whose save then failed would leave the next
// pass treating its whole backlog as arrivals, which is the one outcome the baseline exists to prevent.
// Marking an already-marked folder is a no-op, so it is safe to call on every sync.
func (s *Store) MarkFolderBaselined(ctx context.Context, folderID string) error {
	if _, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO folder_baseline (folder_id) VALUES (?);", folderID); err != nil {
		return fmt.Errorf("mark folder baseline %q: %w", folderID, err)
	}
	return nil
}

// FolderUIDValidity reports the UIDVALIDITY the folder was last synced under. It reports false when none
// has been recorded: a first sight, which the sync records rather than treating as a renumbering. It is
// part of the application.MailStore port.
func (s *Store) FolderUIDValidity(ctx context.Context, folderID string) (uint32, bool, error) {
	var validity int64
	err := s.db.QueryRowContext(ctx,
		"SELECT uid_validity FROM folder_uidvalidity WHERE folder_id = ?;", folderID).Scan(&validity)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read folder UIDVALIDITY %q: %w", folderID, err)
	}
	return uint32(validity), true, nil
}

// SetFolderUIDValidity records the UIDVALIDITY the folder's cached UIDs now belong to, replacing any
// earlier value. The sync calls it only after those messages are saved, so the record never runs ahead
// of the cache it describes.
func (s *Store) SetFolderUIDValidity(ctx context.Context, folderID string, validity uint32) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO folder_uidvalidity (folder_id, uid_validity) VALUES (?, ?)
		 ON CONFLICT(folder_id) DO UPDATE SET uid_validity = excluded.uid_validity;`,
		folderID, int64(validity)); err != nil {
		return fmt.Errorf("record folder UIDVALIDITY %q: %w", folderID, err)
	}
	return nil
}
