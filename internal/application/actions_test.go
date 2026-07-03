package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func trashFolder(t *testing.T, id, accountID string) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder(id, accountID, "Trash", domain.FolderTrash, 0, 0)
	if err != nil {
		t.Fatalf("build trash folder: %v", err)
	}
	return folder
}

func newActionService() (*MessageActionService, *fakeMailStore, *fakeAccountStore, *fakeMailActions) {
	store := newFakeMailStore()
	accounts := newFakeAccountStore()
	remote := &fakeMailActions{}
	return NewMessageActionService(store, accounts, remote), store, accounts, remote
}

func TestMarkReadWritesServerThenCache(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)

	if err := svc.MarkRead(context.Background(), "m1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.seenCalls) != 1 || remote.seenCalls[0] != true {
		t.Errorf("server SetSeen calls = %v, want [true]", remote.seenCalls)
	}
	if !store.messages["f1"][0].IsRead() {
		t.Error("local cache was not marked read")
	}
}

func TestMarkFlaggedWritesServerThenCache(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)

	if err := svc.MarkFlagged(context.Background(), "m1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.flaggedCalls) != 1 || remote.flaggedCalls[0] != true {
		t.Errorf("server SetFlagged calls = %v, want [true]", remote.flaggedCalls)
	}
	if !store.messages["f1"][0].IsFlagged() {
		t.Error("local cache was not flagged")
	}
}

func TestMarkFlaggedGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.MarkFlagged(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkFlagged error = %v, want wrapped boom", err)
	}
}

func TestMarkFlaggedGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if err := svc.MarkFlagged(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkFlagged error = %v, want wrapped boom", err)
	}
}

func TestMarkFlaggedGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if err := svc.MarkFlagged(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkFlagged error = %v, want wrapped boom", err)
	}
}

func TestMarkFlaggedServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	remote.flaggedErr = errBoom
	if err := svc.MarkFlagged(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkFlagged error = %v, want wrapped boom", err)
	}
	if store.messages["f1"][0].IsFlagged() {
		t.Error("cache changed despite a server failure")
	}
}

func TestMarkFlaggedCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.setFlaggedErr = errBoom
	if err := svc.MarkFlagged(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkFlagged error = %v, want wrapped boom", err)
	}
}

func TestMarkReadGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.MarkRead(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkRead error = %v, want wrapped boom", err)
	}
}

func TestMarkReadGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if err := svc.MarkRead(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkRead error = %v, want wrapped boom", err)
	}
}

func TestMarkReadGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if err := svc.MarkRead(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkRead error = %v, want wrapped boom", err)
	}
}

func TestMarkReadServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	remote.setSeenErr = errBoom
	if err := svc.MarkRead(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkRead error = %v, want wrapped boom", err)
	}
	// A failed server write must not update the local cache.
	if store.messages["f1"][0].IsRead() {
		t.Error("cache changed despite a server failure")
	}
}

func TestMarkReadCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.setSeenErr = errBoom
	if err := svc.MarkRead(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkRead error = %v, want wrapped boom", err)
	}
}

func TestDeleteMovesToTrash(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], trashFolder(t, "ft", "a1"))

	if err := svc.Delete(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.deleteTrashPaths) != 1 || remote.deleteTrashPaths[0] != "Trash" {
		t.Errorf("expected move to Trash, got %v", remote.deleteTrashPaths)
	}
	if len(store.deletedMessages) != 1 || store.deletedMessages[0] != "m1" {
		t.Errorf("expected local delete of m1, got %v", store.deletedMessages)
	}
}

func TestDeletePermanentWhenNoTrash(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts) // only an inbox folder, no Trash

	if err := svc.Delete(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.deleteTrashPaths) != 1 || remote.deleteTrashPaths[0] != "" {
		t.Errorf("expected permanent delete (empty trash path), got %v", remote.deleteTrashPaths)
	}
}

func TestDeleteFromTrashIsPermanent(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{trashFolder(t, "ft", "a1")}
	store.messages["ft"] = []domain.MessageSummary{testMessage(t, "m1", "ft")}

	if err := svc.Delete(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.deleteTrashPaths) != 1 || remote.deleteTrashPaths[0] != "" {
		t.Errorf("deleting from Trash should be permanent, got %v", remote.deleteTrashPaths)
	}
}

func TestDeletePermanentSkipsTrash(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], trashFolder(t, "ft", "a1"))

	if err := svc.DeletePermanent(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.deleteTrashPaths) != 1 || remote.deleteTrashPaths[0] != "" {
		t.Errorf("expected permanent delete (empty trash path) even with a Trash folder, got %v", remote.deleteTrashPaths)
	}
	if len(store.deletedMessages) != 1 || store.deletedMessages[0] != "m1" {
		t.Errorf("expected local delete of m1, got %v", store.deletedMessages)
	}
}

func TestDeleteGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.Delete(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Delete error = %v, want wrapped boom", err)
	}
}

func TestDeleteGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if err := svc.Delete(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Delete error = %v, want wrapped boom", err)
	}
}

func TestDeleteGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if err := svc.Delete(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Delete error = %v, want wrapped boom", err)
	}
}

func TestDeleteTrashResolveError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.listFoldersErr = errBoom
	if err := svc.Delete(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Delete error = %v, want wrapped boom", err)
	}
}

func TestDeleteServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	remote.deleteErr = errBoom
	if err := svc.Delete(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Delete error = %v, want wrapped boom", err)
	}
	if len(store.deletedMessages) != 0 {
		t.Error("cache deleted despite a server failure")
	}
}

func TestDeleteCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.deleteMessageErr = errBoom
	if err := svc.Delete(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Delete error = %v, want wrapped boom", err)
	}
}

func TestMoveSuccess(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], testFolder(t, "f2", "a1", "Archive"))

	if err := svc.Move(context.Background(), "m1", "f2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.moveDestPaths) != 1 || remote.moveDestPaths[0] != "Archive" {
		t.Errorf("expected move to Archive, got %v", remote.moveDestPaths)
	}
	if len(store.deletedMessages) != 1 || store.deletedMessages[0] != "m1" {
		t.Errorf("expected local removal of m1, got %v", store.deletedMessages)
	}
}

func TestMoveGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.Move(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Move error = %v, want wrapped boom", err)
	}
}

func TestMoveSourceFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if err := svc.Move(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Move error = %v, want wrapped boom", err)
	}
}

func TestMoveGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if err := svc.Move(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Move error = %v, want wrapped boom", err)
	}
}

func TestMoveDestFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts) // f2 does not exist
	if err := svc.Move(context.Background(), "m1", "f2"); err == nil {
		t.Error("expected an error for a missing destination folder")
	}
}

func TestMoveCrossAccountRejected(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a2"] = []domain.Folder{testFolder(t, "f2", "a2", "Other")}
	if err := svc.Move(context.Background(), "m1", "f2"); err == nil {
		t.Error("expected an error moving across accounts")
	}
	if len(remote.moveDestPaths) != 0 || len(store.deletedMessages) != 0 {
		t.Error("cross-account move must not touch the server or cache")
	}
}

func TestMoveServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], testFolder(t, "f2", "a1", "Archive"))
	remote.moveErr = errBoom
	if err := svc.Move(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Move error = %v, want wrapped boom", err)
	}
	if len(store.deletedMessages) != 0 {
		t.Error("cache changed despite a server failure")
	}
}

func TestCopySuccess(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], testFolder(t, "f2", "a1", "Archive"))

	if err := svc.Copy(context.Background(), "m1", "f2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.copyDestPaths) != 1 || remote.copyDestPaths[0] != "Archive" {
		t.Errorf("expected copy to Archive, got %v", remote.copyDestPaths)
	}
	// Unlike Move, Copy must leave the original in the local cache.
	if len(store.deletedMessages) != 0 {
		t.Errorf("copy must not remove the original, deleted=%v", store.deletedMessages)
	}
}

func TestCopyGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.Copy(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Copy error = %v, want wrapped boom", err)
	}
}

func TestCopySourceFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if err := svc.Copy(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Copy error = %v, want wrapped boom", err)
	}
}

func TestCopyGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if err := svc.Copy(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Copy error = %v, want wrapped boom", err)
	}
}

func TestCopyDestFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts) // f2 does not exist
	if err := svc.Copy(context.Background(), "m1", "f2"); err == nil {
		t.Error("expected an error for a missing destination folder")
	}
}

func TestCopyCrossAccountRejected(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a2"] = []domain.Folder{testFolder(t, "f2", "a2", "Other")}
	if err := svc.Copy(context.Background(), "m1", "f2"); err == nil {
		t.Error("expected an error copying across accounts")
	}
	if len(remote.copyDestPaths) != 0 {
		t.Error("cross-account copy must not touch the server")
	}
}

func TestCopyServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], testFolder(t, "f2", "a1", "Archive"))
	remote.copyErr = errBoom
	if err := svc.Copy(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Copy error = %v, want wrapped boom", err)
	}
}

func TestMoveCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], testFolder(t, "f2", "a1", "Archive"))
	store.deleteMessageErr = errBoom
	if err := svc.Move(context.Background(), "m1", "f2"); !errors.Is(err, errBoom) {
		t.Errorf("Move error = %v, want wrapped boom", err)
	}
}
