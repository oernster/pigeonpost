package storage

// The sweep that stops cached messages outliving their folder. A folder rename or move replaces the
// account's folder set under new ids (a folder id is its account plus its path), so every message row
// still carrying the old folder id has nothing to be listed under; SaveFolders clears them and
// everything derived from them.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

func saveFolder(t *testing.T, store *Store, accountID string, paths ...string) {
	t.Helper()
	folders := make([]domain.Folder, 0, len(paths))
	for _, path := range paths {
		folder, err := domain.NewFolder(accountID+"\x1f"+path, accountID, path, domain.FolderCustom, 0, 0)
		if err != nil {
			t.Fatalf("folder %q: %v", path, err)
		}
		folders = append(folders, folder)
	}
	if err := store.SaveFolders(context.Background(), accountID, folders); err != nil {
		t.Fatalf("save folders: %v", err)
	}
}

func TestSaveFoldersClearsMessagesWhoseFolderHasGone(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	const account = "a1"
	kept, gone := account+"\x1fKeep", account+"\x1fOld"

	saveFolder(t, store, account, "Keep", "Old")
	if err := store.SaveMessages(ctx, kept, []domain.MessageSummary{buildMessageIn(t, kept+"\x1f1", kept, false)}); err != nil {
		t.Fatalf("save kept messages: %v", err)
	}
	orphan := gone + "\x1f1"
	if err := store.SaveMessages(ctx, gone, []domain.MessageSummary{buildMessageIn(t, orphan, gone, false)}); err != nil {
		t.Fatalf("save orphan messages: %v", err)
	}
	body, err := domain.NewMessageBody(orphan, "text", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}

	// The rename: the same mailbox comes back under a new path, so "Old" is not in the new set.
	saveFolder(t, store, account, "Keep", "Renamed")

	if _, err := store.GetMessage(ctx, orphan); err == nil {
		t.Fatal("expected the orphaned message to be gone from the cache")
	}
	if _, err := store.GetMessageBody(ctx, orphan); err == nil {
		t.Fatal("expected the orphaned message body to be gone from the cache")
	}
	remaining, err := store.ListMessages(ctx, kept)
	if err != nil {
		t.Fatalf("list kept: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected the surviving folder to keep its message, got %d", len(remaining))
	}
}

func TestSaveFoldersKeepsAnotherAccountsMessages(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	const other = "a2"
	otherFolder := other + "\x1fInbox"

	saveFolder(t, store, other, "Inbox")
	if err := store.SaveMessages(ctx, otherFolder,
		[]domain.MessageSummary{buildMessageIn(t, otherFolder+"\x1f1", otherFolder, false)}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	// A different account's folder set is replaced: the sweep runs; it must not touch a2's rows.
	saveFolder(t, store, "a1", "Keep")

	remaining, err := store.ListMessages(ctx, otherFolder)
	if err != nil {
		t.Fatalf("list other account: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected the other account to keep its message, got %d", len(remaining))
	}
}

// A message id the cache does not hold is a named condition, not a bare database miss: the facade
// turns it into a sentence and the interface recovers from it, neither of which can key on the text
// of a failed query.
func TestGetMessageReportsAnUncachedMessageAsSuch(t *testing.T) {
	store := openTestStore(t)

	_, err := store.GetMessage(context.Background(), "a1\x1fInbox\x1f545")

	if !errors.Is(err, application.ErrMessageNotCached) {
		t.Fatalf("expected ErrMessageNotCached, got %v", err)
	}
}

// TestOrphanMessageMigrationClearsWhatEarlierVersionsLeft covers the upgrade path. The sweep in
// SaveFolders only runs when a folder set is next replaced, which needs a full account sync; the rows
// already stranded in an installed cache are cleared the moment it updates instead.
func TestOrphanMessageMigrationClearsWhatEarlierVersionsLeft(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// preSweepVersion is the version schemaV54 upgrades FROM. It is a fixed number rather than an
	// offset from schemaVersion, so later migrations cannot quietly move this test off its step.
	const preSweepVersion = 53
	for _, step := range migrations[:preSweepVersion] {
		if _, err := db.ExecContext(ctx, step); err != nil {
			t.Fatalf("apply migrations up to %d: %v", preSweepVersion, err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", preSweepVersion)); err != nil {
		t.Fatalf("set version: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO folder (id, account_id, path, separator, kind, unread, total)
		 VALUES (?, ?, ?, ?, ?, ?, ?);`, "f1", "a1", "INBOX", "/", 0, 0, 0); err != nil {
		t.Fatalf("insert folder: %v", err)
	}
	insertMessage := func(id, folderID string) {
		t.Helper()
		if _, err := db.ExecContext(ctx,
			`INSERT INTO message (id, folder_id, uid, message_id, from_display, from_address, subject,
			        date_ms, size, flags, has_attachments, snippet, to_json, cc_json)
			 VALUES (?, ?, '1', '<m@x>', '', 'a@b.c', 'Subject', 0, 1, 0, 0, '', '[]', '[]');`,
			id, folderID); err != nil {
			t.Fatalf("insert message %q: %v", id, err)
		}
	}
	insertMessage("kept", "f1")
	insertMessage("stranded", "gone")
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open store (migration failed): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.GetMessage(ctx, "stranded"); !errors.Is(err, application.ErrMessageNotCached) {
		t.Fatalf("the stranded message survived the migration: %v", err)
	}
	if _, err := store.GetMessage(ctx, "kept"); err != nil {
		t.Fatalf("the migration took a message whose folder is still there: %v", err)
	}
}
