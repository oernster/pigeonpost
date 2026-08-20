package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TestFolderBaselineStartsUnsetAndMarks covers the pair: a folder nothing has been recorded about has
// not been baselined, marking it records that, marking it twice is a no-op rather than an error, so
// every sync can call it unconditionally.
func TestFolderBaselineStartsUnsetAndMarks(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	baselined, err := store.FolderBaselined(ctx, "f1")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if baselined {
		t.Fatal("an unknown folder reported as baselined, so its backlog would be treated as arrivals")
	}

	for i := 0; i < 2; i++ {
		if err := store.MarkFolderBaselined(ctx, "f1"); err != nil {
			t.Fatalf("mark baseline (call %d): %v", i+1, err)
		}
	}
	baselined, err = store.FolderBaselined(ctx, "f1")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !baselined {
		t.Error("a marked folder did not read back as baselined")
	}
	// The mark is per folder, so one folder's baseline never licenses another's.
	other, err := store.FolderBaselined(ctx, "f2")
	if err != nil {
		t.Fatalf("read other baseline: %v", err)
	}
	if other {
		t.Error("marking one folder baselined another")
	}
}

// TestFolderBaselineSurvivesAFolderRewrite is the reason the mark is not a column on the folder row.
// SaveFolders clears and rewrites every folder for an account on each sync, so a mark living there
// would be dropped every pass and the destructive-rule exemption would re-arm forever.
func TestFolderBaselineSurvivesAFolderRewrite(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	inbox, err := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("save folders: %v", err)
	}
	if err := store.MarkFolderBaselined(ctx, "f1"); err != nil {
		t.Fatalf("mark baseline: %v", err)
	}

	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("re-save folders: %v", err)
	}

	baselined, err := store.FolderBaselined(ctx, "f1")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !baselined {
		t.Error("a folder rewrite cleared the baseline mark")
	}
}

// TestFolderBaselineMigrationMarksExistingFolders covers the upgrade path, which is what decides how an
// existing installation behaves the moment it updates. Every folder already in the database has had its
// baseline established through ordinary use, so the migration marks it; without that, the first sync
// after updating would treat each folder as a first sight and exempt its arrivals from destructive
// rules all over again.
func TestFolderBaselineMigrationMarksExistingFolders(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// preBaselineVersion is the version schemaV53 upgrades FROM. It is a fixed number rather than an
	// offset from schemaVersion, so later migrations cannot quietly move this test off the step it
	// exists to cover.
	const preBaselineVersion = 52
	for _, step := range migrations[:preBaselineVersion] {
		if _, err := db.ExecContext(ctx, step); err != nil {
			t.Fatalf("apply migrations up to %d: %v", preBaselineVersion, err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", preBaselineVersion)); err != nil {
		t.Fatalf("set version: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO folder (id, account_id, path, separator, kind, unread, total)
		 VALUES (?, ?, ?, ?, ?, ?, ?);`, "f1", "a1", "INBOX", "/", 0, 0, 0); err != nil {
		t.Fatalf("insert folder: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open store (migration failed): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	baselined, err := store.FolderBaselined(ctx, "f1")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !baselined {
		t.Error("an existing folder was not baselined by the migration, so its next sync re-arms the exemption")
	}
}

// TestFolderBaselineFreshDatabaseMarksNothing is the counterpart: a new installation has established no
// baseline for anything, so a first account added to it still gets its one protected pass.
func TestFolderBaselineFreshDatabaseMarksNothing(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{freshInbox(t)}); err != nil {
		t.Fatalf("save folders: %v", err)
	}

	baselined, err := store.FolderBaselined(ctx, "f1")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if baselined {
		t.Error("a folder on a fresh database read as baselined, so a new account could be emptied")
	}
}

// freshInbox builds a plain inbox folder for the cases that only need one to exist.
func freshInbox(t *testing.T) domain.Folder {
	t.Helper()
	inbox, err := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	return inbox
}
