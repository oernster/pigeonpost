package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
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

// The UIDVALIDITY record is part of the MailStore port; a drifted signature is a build failure here.
var _ application.MailStore = (*Store)(nil)

// The UIDVALIDITY values the record tests write: a first value and the one a renumbering replaces it with.
const (
	firstValidity  uint32 = 7
	secondValidity uint32 = 9
)

// readValidity reads a folder's stored UIDVALIDITY or fails the test.
func readValidity(t *testing.T, store *Store, folderID string) (uint32, bool) {
	t.Helper()
	validity, ok, err := store.FolderUIDValidity(context.Background(), folderID)
	if err != nil {
		t.Fatalf("read UIDVALIDITY %q: %v", folderID, err)
	}
	return validity, ok
}

// TestFolderUIDValidityStartsUnsetThenRecords covers the record: a folder synced never before has none
// (a first sight, not a change), a recorded value reads back, a later value replaces it and one
// folder's value never answers for another.
func TestFolderUIDValidityStartsUnsetThenRecords(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	if _, ok := readValidity(t, store, "f1"); ok {
		t.Fatal("an unsynced folder reported a stored UIDVALIDITY, so its first sync would read as a renumbering")
	}
	for _, want := range []uint32{firstValidity, secondValidity} {
		if err := store.SetFolderUIDValidity(ctx, "f1", want); err != nil {
			t.Fatalf("record UIDVALIDITY %d: %v", want, err)
		}
		if got, ok := readValidity(t, store, "f1"); !ok || got != want {
			t.Errorf("UIDVALIDITY = %d (stored %v), want %d", got, ok, want)
		}
	}
	if _, ok := readValidity(t, store, "f2"); ok {
		t.Error("recording one folder's UIDVALIDITY recorded another's")
	}
}

// TestFolderUIDValiditySurvivesAFolderRewrite pins why the value is not a column on the folder row:
// SaveFolders rewrites every folder each sync; a lost value would make every pass a first sight.
func TestFolderUIDValiditySurvivesAFolderRewrite(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{freshInbox(t)}); err != nil {
		t.Fatalf("save folders: %v", err)
	}
	if err := store.SetFolderUIDValidity(ctx, "f1", firstValidity); err != nil {
		t.Fatalf("record UIDVALIDITY: %v", err)
	}
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{freshInbox(t)}); err != nil {
		t.Fatalf("re-save folders: %v", err)
	}
	if got, ok := readValidity(t, store, "f1"); !ok || got != firstValidity {
		t.Errorf("UIDVALIDITY after a folder rewrite = %d (stored %v), want %d", got, ok, firstValidity)
	}
}

// TestFolderUIDValidityMigrationIsIdempotent re-runs schemaV58 over a migrated database. Steps run
// outside a transaction, so a step interrupted part way must be safe to run again; it must also keep
// the values already recorded.
func TestFolderUIDValidityMigrationIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.SetFolderUIDValidity(ctx, "f1", firstValidity); err != nil {
		t.Fatalf("record UIDVALIDITY: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, schemaV58); err != nil {
		t.Fatalf("re-run schemaV58: %v", err)
	}
	if got, ok := readValidity(t, store, "f1"); !ok || got != firstValidity {
		t.Errorf("UIDVALIDITY after re-running the step = %d (stored %v), want %d", got, ok, firstValidity)
	}
}

// TestFolderUIDValidityReportsAFailedStore pins that a store that cannot be read or written says so,
// so the sync stops the folder rather than guessing whether it was renumbered.
func TestFolderUIDValidityReportsAFailedStore(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, _, err := store.FolderUIDValidity(ctx, "f1"); err == nil {
		t.Error("a read from a closed store reported no error")
	}
	if err := store.SetFolderUIDValidity(ctx, "f1", firstValidity); err == nil {
		t.Error("a write to a closed store reported no error")
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
