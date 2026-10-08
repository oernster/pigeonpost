package storage

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Removing an account must take its folders' sync markers with it. A folder's id is its account plus its
// path, so the same address added again gets the same folder ids. Had the baseline and UIDVALIDITY rows
// survived, the re-added Inbox would read as already synced with an empty cache: every message on the
// server would then count as an arrival and a move or delete rule would act on the whole backlog.
func TestDeleteAccountDataClearsItsFoldersSyncMarkers(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	inbox, err := domain.NewFolder("a1\x1fINBOX", "a1", "INBOX", domain.FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	other, err := domain.NewFolder("a2\x1fINBOX", "a2", "INBOX", domain.FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	for _, f := range []domain.Folder{inbox, other} {
		if err := store.SaveFolders(ctx, f.AccountID(), []domain.Folder{f}); err != nil {
			t.Fatalf("save folders: %v", err)
		}
		if err := store.MarkFolderBaselined(ctx, f.ID()); err != nil {
			t.Fatalf("baseline: %v", err)
		}
		if err := store.SetFolderUIDValidity(ctx, f.ID(), 7); err != nil {
			t.Fatalf("validity: %v", err)
		}
	}

	if err := store.DeleteAccountData(ctx, "a1"); err != nil {
		t.Fatalf("DeleteAccountData: %v", err)
	}
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("save folders again: %v", err)
	}
	if baselined, err := store.FolderBaselined(ctx, inbox.ID()); err != nil || baselined {
		t.Errorf("re-added Inbox baselined = %v (err %v), want false", baselined, err)
	}
	if _, has, err := store.FolderUIDValidity(ctx, inbox.ID()); err != nil || has {
		t.Errorf("re-added Inbox has a UIDVALIDITY = %v (err %v), want none", has, err)
	}
	// Another account's markers are untouched.
	if baselined, _ := store.FolderBaselined(ctx, other.ID()); !baselined {
		t.Error("another account's baseline was removed")
	}
	if _, has, _ := store.FolderUIDValidity(ctx, other.ID()); !has {
		t.Error("another account's UIDVALIDITY was removed")
	}
}
