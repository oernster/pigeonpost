package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newFolderService() (*FolderService, *fakeAccountStore, *fakeMailStore, *fakeMailSource, *fakeFolderActions) {
	accounts := newFakeAccountStore()
	store := newFakeMailStore()
	source := &fakeMailSource{}
	remote := &fakeFolderActions{}
	return NewFolderService(accounts, store, source, remote), accounts, store, source, remote
}

// seedFolder registers account a1 and a folder in the store, returning nothing (the ids are fixed).
func seedFolder(t *testing.T, accounts *fakeAccountStore, store *fakeMailStore, folderID, path string) {
	t.Helper()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, folderID, "a1", path)}
}

func TestFolderCreate(t *testing.T) {
	svc, accounts, store, _, remote := newFolderService()
	accounts.accounts["a1"] = testAccount(t, "a1")

	if err := svc.Create(context.Background(), "a1", "  Projects  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.created) != 1 || remote.created[0] != "Projects" {
		t.Errorf("expected trimmed create of Projects, got %v", remote.created)
	}
	if len(store.savedFolderKeys) != 1 {
		t.Errorf("expected the folder list to be refreshed, got %v", store.savedFolderKeys)
	}
}

func TestFolderCreateErrors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		svc, accounts, _, _, _ := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		if err := svc.Create(context.Background(), "a1", "   "); !errors.Is(err, ErrEmptyFolderName) {
			t.Errorf("error = %v, want ErrEmptyFolderName", err)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		svc, _, _, _, _ := newFolderService()
		if err := svc.Create(context.Background(), "missing", "X"); !errors.Is(err, ErrAccountNotFound) {
			t.Errorf("error = %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("remote failure", func(t *testing.T) {
		svc, accounts, _, _, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		remote.createErr = errBoom
		if err := svc.Create(context.Background(), "a1", "X"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("refresh fetch failure", func(t *testing.T) {
		svc, accounts, _, source, _ := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.fetchFoldersErr = errBoom
		if err := svc.Create(context.Background(), "a1", "X"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("refresh save failure", func(t *testing.T) {
		svc, accounts, store, _, _ := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		store.saveFoldersErr = errBoom
		if err := svc.Create(context.Background(), "a1", "X"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestFolderRename(t *testing.T) {
	svc, accounts, store, _, remote := newFolderService()
	seedFolder(t, accounts, store, "f1", "Parent/Old")

	if err := svc.Rename(context.Background(), "f1", "  New  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.renamed) != 1 || remote.renamed[0] != [2]string{"Parent/Old", "Parent/New"} {
		t.Errorf("expected rename Parent/Old -> Parent/New, got %v", remote.renamed)
	}
}

func TestFolderRenameErrors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		svc, accounts, store, _, _ := newFolderService()
		seedFolder(t, accounts, store, "f1", "Old")
		if err := svc.Rename(context.Background(), "f1", "  "); !errors.Is(err, ErrEmptyFolderName) {
			t.Errorf("error = %v, want ErrEmptyFolderName", err)
		}
	})

	t.Run("folder not found", func(t *testing.T) {
		svc, _, store, _, _ := newFolderService()
		store.getFolderErr = errBoom
		if err := svc.Rename(context.Background(), "f1", "New"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		svc, accounts, store, _, _ := newFolderService()
		seedFolder(t, accounts, store, "f1", "Old")
		accounts.getErr = errBoom
		if err := svc.Rename(context.Background(), "f1", "New"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("remote failure", func(t *testing.T) {
		svc, accounts, store, _, remote := newFolderService()
		seedFolder(t, accounts, store, "f1", "Old")
		remote.renameErr = errBoom
		if err := svc.Rename(context.Background(), "f1", "New"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestFolderDelete(t *testing.T) {
	svc, accounts, store, _, remote := newFolderService()
	seedFolder(t, accounts, store, "f1", "Junk")

	if err := svc.Delete(context.Background(), "f1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.deleted) != 1 || remote.deleted[0] != "Junk" {
		t.Errorf("expected delete of Junk, got %v", remote.deleted)
	}
	// The folder's cached messages are cleared (SaveMessages with an empty set for that folder).
	if len(store.savedMessageKeys) != 1 || store.savedMessageKeys[0] != "f1" {
		t.Errorf("expected cached messages cleared for f1, got %v", store.savedMessageKeys)
	}
}

func TestFolderDeleteErrors(t *testing.T) {
	t.Run("folder not found", func(t *testing.T) {
		svc, _, store, _, _ := newFolderService()
		store.getFolderErr = errBoom
		if err := svc.Delete(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		svc, accounts, store, _, _ := newFolderService()
		seedFolder(t, accounts, store, "f1", "Junk")
		accounts.getErr = errBoom
		if err := svc.Delete(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("remote failure", func(t *testing.T) {
		svc, accounts, store, _, remote := newFolderService()
		seedFolder(t, accounts, store, "f1", "Junk")
		remote.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("clear messages failure", func(t *testing.T) {
		svc, accounts, store, _, _ := newFolderService()
		seedFolder(t, accounts, store, "f1", "Junk")
		store.saveMessagesErr = errBoom
		if err := svc.Delete(context.Background(), "f1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

// reconcileFolder builds an a1 folder of the given kind for the sent-reconciliation tests.
func reconcileFolder(t *testing.T, id, path string, kind domain.FolderKind) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder(id, "a1", path, kind, 0, 0)
	if err != nil {
		t.Fatalf("build folder %q: %v", path, err)
	}
	return folder
}

func TestReconcileSent(t *testing.T) {
	t.Run("merges strays into the canonical sent and deletes them", func(t *testing.T) {
		svc, accounts, store, source, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.folders = []domain.Folder{
			reconcileFolder(t, "in", "INBOX", domain.FolderInbox),
			reconcileFolder(t, "sent", "Sent", domain.FolderSent),
			reconcileFolder(t, "sm", "Sent Messages", domain.FolderCustom),
			reconcileFolder(t, "money", "Money", domain.FolderCustom),
			reconcileFolder(t, "ms", "Money/Sent", domain.FolderCustom),
		}
		if err := svc.ReconcileSent(context.Background(), "a1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantMoves := [][2]string{{"Sent Messages", "Sent"}, {"Money/Sent", "Sent"}}
		if len(remote.movedAll) != len(wantMoves) || remote.movedAll[0] != wantMoves[0] || remote.movedAll[1] != wantMoves[1] {
			t.Errorf("moves = %v, want %v", remote.movedAll, wantMoves)
		}
		if len(remote.deleted) != 2 || remote.deleted[0] != "Sent Messages" || remote.deleted[1] != "Money/Sent" {
			t.Errorf("deleted = %v, want the two strays", remote.deleted)
		}
		if len(store.savedMessageKeys) != 2 || store.savedMessageKeys[0] != "sm" || store.savedMessageKeys[1] != "ms" {
			t.Errorf("cleared caches = %v, want sm and ms", store.savedMessageKeys)
		}
		if len(remote.renamed) != 0 {
			t.Errorf("expected no relocation for a top-level canonical, got %v", remote.renamed)
		}
		if len(store.savedFolderKeys) != 1 {
			t.Errorf("expected one folder-list refresh, got %v", store.savedFolderKeys)
		}
	})

	t.Run("no-op when one top-level sent already", func(t *testing.T) {
		svc, accounts, store, source, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.folders = []domain.Folder{
			reconcileFolder(t, "in", "INBOX", domain.FolderInbox),
			reconcileFolder(t, "sent", "Sent", domain.FolderSent),
		}
		if err := svc.ReconcileSent(context.Background(), "a1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(remote.movedAll) != 0 || len(remote.deleted) != 0 || len(remote.renamed) != 0 {
			t.Errorf("expected no server changes, got %v %v %v", remote.movedAll, remote.deleted, remote.renamed)
		}
		if len(store.savedFolderKeys) != 0 {
			t.Errorf("expected no refresh, got %v", store.savedFolderKeys)
		}
	})

	t.Run("no-op with no sent folder at all", func(t *testing.T) {
		svc, accounts, _, source, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.folders = []domain.Folder{reconcileFolder(t, "in", "INBOX", domain.FolderInbox)}
		if err := svc.ReconcileSent(context.Background(), "a1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(remote.movedAll) != 0 || len(remote.deleted) != 0 {
			t.Errorf("expected no server changes, got %v %v", remote.movedAll, remote.deleted)
		}
	})

	t.Run("skips a pop3 account", func(t *testing.T) {
		svc, accounts, _, source, remote := newFolderService()
		accounts.accounts["a1"] = pop3Account(t, "a1")
		source.folders = []domain.Folder{
			reconcileFolder(t, "sent", "Sent", domain.FolderSent),
			reconcileFolder(t, "sm", "Sent Messages", domain.FolderCustom),
		}
		if err := svc.ReconcileSent(context.Background(), "a1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(remote.movedAll) != 0 || len(remote.deleted) != 0 {
			t.Errorf("expected pop3 to be skipped, got %v %v", remote.movedAll, remote.deleted)
		}
	})

	t.Run("relocates a nested name-classified sent to the top level", func(t *testing.T) {
		svc, accounts, store, source, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.folders = []domain.Folder{
			reconcileFolder(t, "money", "Money", domain.FolderCustom),
			reconcileFolder(t, "ms", "Money/Sent", domain.FolderSent),
		}
		if err := svc.ReconcileSent(context.Background(), "a1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(remote.renamed) != 1 || remote.renamed[0] != [2]string{"Money/Sent", "Sent"} {
			t.Errorf("renamed = %v, want Money/Sent -> Sent", remote.renamed)
		}
		if len(store.savedFolderKeys) != 1 {
			t.Errorf("expected a refresh after relocation, got %v", store.savedFolderKeys)
		}
	})

	t.Run("respects a server-declared nested sent", func(t *testing.T) {
		svc, accounts, _, source, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.folders = []domain.Folder{
			reconcileFolder(t, "g", "[Gmail]", domain.FolderCustom),
			reconcileFolder(t, "gs", "[Gmail]/Sent Mail", domain.FolderSent).WithSpecialUse(true),
		}
		if err := svc.ReconcileSent(context.Background(), "a1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(remote.renamed) != 0 {
			t.Errorf("a special-use sent must not be relocated, got %v", remote.renamed)
		}
	})

	t.Run("respects a sent nested under INBOX", func(t *testing.T) {
		svc, accounts, _, source, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		nested, err := domain.NewFolderWithSeparator("ins", "a1", "INBOX.Sent", ".", domain.FolderSent, 0, 0)
		if err != nil {
			t.Fatalf("build nested sent: %v", err)
		}
		source.folders = []domain.Folder{
			reconcileFolder(t, "in", "INBOX", domain.FolderInbox),
			nested,
		}
		if err := svc.ReconcileSent(context.Background(), "a1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(remote.renamed) != 0 {
			t.Errorf("a sent under INBOX must not be relocated, got %v", remote.renamed)
		}
	})
}

func TestReconcileSentErrors(t *testing.T) {
	seed := func(svc *FolderService, accounts *fakeAccountStore, source *fakeMailSource) {
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.folders = []domain.Folder{
			reconcileFolder(t, "sent", "Sent", domain.FolderSent),
			reconcileFolder(t, "sm", "Sent Messages", domain.FolderCustom),
		}
		_ = svc
	}

	t.Run("account load failure", func(t *testing.T) {
		svc, accounts, _, _, _ := newFolderService()
		accounts.getErr = errBoom
		if err := svc.ReconcileSent(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("list failure", func(t *testing.T) {
		svc, accounts, _, source, _ := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.fetchFoldersErr = errBoom
		if err := svc.ReconcileSent(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("a failed move leaves the stray undeleted", func(t *testing.T) {
		svc, accounts, _, source, remote := newFolderService()
		seed(svc, accounts, source)
		remote.moveAllErr = errBoom
		if err := svc.ReconcileSent(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
		if len(remote.deleted) != 0 {
			t.Errorf("the stray must not be deleted when its messages did not move, got %v", remote.deleted)
		}
	})

	t.Run("clear cache failure", func(t *testing.T) {
		svc, accounts, store, source, _ := newFolderService()
		seed(svc, accounts, source)
		store.saveMessagesErr = errBoom
		if err := svc.ReconcileSent(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("delete failure", func(t *testing.T) {
		svc, accounts, _, source, remote := newFolderService()
		seed(svc, accounts, source)
		remote.deleteErr = errBoom
		if err := svc.ReconcileSent(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("relocation failure", func(t *testing.T) {
		svc, accounts, _, source, remote := newFolderService()
		accounts.accounts["a1"] = testAccount(t, "a1")
		source.folders = []domain.Folder{reconcileFolder(t, "ms", "Money/Sent", domain.FolderSent)}
		remote.renameErr = errBoom
		if err := svc.ReconcileSent(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("refresh failure", func(t *testing.T) {
		svc, accounts, store, source, _ := newFolderService()
		seed(svc, accounts, source)
		store.saveFoldersErr = errBoom
		if err := svc.ReconcileSent(context.Background(), "a1"); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}
