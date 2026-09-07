package storage

// folder_store.go holds the cached-folder half of the mail store: listing an account's folders,
// replacing that set wholesale and reading one back, together with the sweep that keeps message rows
// from outliving the folder they belong to.

import (
	"context"
	"database/sql"
	"fmt"

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
	//
	// The archive always reports none. Archiving is the act of putting a message out of the way, so it
	// does not go on demanding attention, which is the same rule the account-level totals apply. On Gmail
	// the count would also be untrue rather than merely unwanted: the archive there is All Mail, which
	// holds a copy of every labelled message, so the number would be the whole mailbox's unread dressed
	// up as the archive's. Worse, the copies drift: reading a message in the inbox updates that row alone,
	// so the archive's twin stays unread in the cache and the badge stays lit until the archive is opened
	// and synced. A count nobody can trust is worth less than no count.
	unreadCountExpr = fmt.Sprintf(
		"(SELECT COUNT(*) FROM message m WHERE m.folder_id = f.id AND (m.flags & %d) = 0 AND f.kind != %d)",
		int(domain.FlagSeen), int(domain.FolderArchive))
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
		return deleteOrphanMessages(ctx, tx)
	})
}

// orphanMessageIDs selects the cached messages whose folder is no longer in the folder table. A
// message id carries the folder id as its prefix, so the two can never be reconciled after the fact:
// the row simply has no folder to be listed under.
const orphanMessageIDs = `SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id)`

// deleteOrphanMessages removes every cached message whose folder has gone, plus everything derived
// from it, so a folder set replaced by SaveFolders cannot leave rows behind.
//
// This is what a folder rename or move used to leave: the server carries the mailbox to its new path,
// the refreshed listing arrives under a new folder id (the id is account plus path) and the folder
// table is replaced, while the message rows keep the id of a folder that no longer exists. Nothing
// lists them, nothing counts them and nothing ever deletes them, so they accumulate for as long as the
// cache lives; one account had 2,439 of them across 18 dead folder ids. A message row is a cache of
// server data, so dropping it costs a re-fetch and nothing else.
//
// The sweep is deliberately not limited to the account being saved: a row is an orphan only when no
// folder anywhere references it; another account's folders are untouched by this transaction, so
// widening it cannot take a row that account still needs. It also means one folder refresh clears what
// earlier versions left behind, whichever account left it.
func deleteOrphanMessages(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{
		"message_body", "message_attachment", "message_tag", "message_tag_pending",
		"message_flag_pending", "message_search", "message_snooze",
	} {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE message_id IN (%s);", table, orphanMessageIDs)); err != nil {
			return fmt.Errorf("clear orphaned %s rows: %w", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM message WHERE id IN (%s);", orphanMessageIDs)); err != nil {
		return fmt.Errorf("clear orphaned messages: %w", err)
	}
	return nil
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
