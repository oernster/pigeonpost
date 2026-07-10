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
