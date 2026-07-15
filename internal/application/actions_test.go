package application

import (
	"context"
	"errors"
	"reflect"
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

func TestMarkAnsweredWritesServerThenCache(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)

	if err := svc.MarkAnswered(context.Background(), "m1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.answeredCalls) != 1 || remote.answeredCalls[0] != true {
		t.Errorf("server SetAnswered calls = %v, want [true]", remote.answeredCalls)
	}
	if !store.messages["f1"][0].IsAnswered() {
		t.Error("local cache was not marked answered")
	}
}

func TestMarkAnsweredResolveError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.MarkAnswered(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkAnswered error = %v, want wrapped boom", err)
	}
}

func TestMarkAnsweredServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	remote.answeredErr = errBoom
	if err := svc.MarkAnswered(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkAnswered error = %v, want wrapped boom", err)
	}
	if store.messages["f1"][0].IsAnswered() {
		t.Error("cache changed despite a server failure")
	}
}

func TestMarkAnsweredCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.setAnsweredErr = errBoom
	if err := svc.MarkAnswered(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkAnswered error = %v, want wrapped boom", err)
	}
}

func TestMarkForwardedWritesServerThenCache(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)

	if err := svc.MarkForwarded(context.Background(), "m1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.forwardedCalls) != 1 || remote.forwardedCalls[0] != true {
		t.Errorf("server SetForwarded calls = %v, want [true]", remote.forwardedCalls)
	}
	if !store.messages["f1"][0].IsForwarded() {
		t.Error("local cache was not marked forwarded")
	}
}

func TestMarkForwardedResolveError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.MarkForwarded(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkForwarded error = %v, want wrapped boom", err)
	}
}

func TestMarkForwardedServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	remote.forwardedErr = errBoom
	if err := svc.MarkForwarded(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkForwarded error = %v, want wrapped boom", err)
	}
	if store.messages["f1"][0].IsForwarded() {
		t.Error("cache changed despite a server failure")
	}
}

func TestMarkForwardedCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.setForwardedErr = errBoom
	if err := svc.MarkForwarded(context.Background(), "m1", true); !errors.Is(err, errBoom) {
		t.Errorf("MarkForwarded error = %v, want wrapped boom", err)
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

func TestDeleteManyBatchesOneFolderToTrash(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1"), testMessage(t, "m2", "f1")}

	deleted, err := svc.DeleteMany(context.Background(), []string{"m1", "m2"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("deleted = %v, want two ids", deleted)
	}
	// One batched server call for the whole folder, not one per message, carrying the Trash path.
	if len(remote.deleteManyBatches) != 1 || len(remote.deleteManyBatches[0]) != 2 {
		t.Errorf("expected one batch of two uids, got %v", remote.deleteManyBatches)
	}
	if len(remote.deleteManyTrash) != 1 || remote.deleteManyTrash[0] != "Trash" {
		t.Errorf("expected batch move to Trash, got %v", remote.deleteManyTrash)
	}
	if len(store.deletedMessages) != 2 {
		t.Errorf("expected both cached rows removed, got %v", store.deletedMessages)
	}
}

func TestDeleteManyPermanentSkipsTrash(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}

	deleted, err := svc.DeleteMany(context.Background(), []string{"m1"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 1 {
		t.Errorf("deleted = %v, want one id", deleted)
	}
	if len(remote.deleteManyTrash) != 1 || remote.deleteManyTrash[0] != "" {
		t.Errorf("expected permanent batch (empty trash path), got %v", remote.deleteManyTrash)
	}
}

func TestDeleteManyGroupsByFolder(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"),
		testFolder(t, "f2", "a1", "Archive"),
	}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	store.messages["f2"] = []domain.MessageSummary{testMessage(t, "m2", "f2")}

	deleted, err := svc.DeleteMany(context.Background(), []string{"m1", "m2"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("deleted = %v, want two ids", deleted)
	}
	if len(remote.deleteManyBatches) != 2 {
		t.Errorf("expected two batches, one per folder, got %v", remote.deleteManyBatches)
	}
}

func TestDeleteManyGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	deleted, err := svc.DeleteMany(context.Background(), []string{"m1"}, false)
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(deleted) != 0 {
		t.Errorf("deleted = %v, want none", deleted)
	}
}

func TestDeleteManyGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	deleted, err := svc.DeleteMany(context.Background(), []string{"m1"}, false)
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(deleted) != 0 {
		t.Errorf("deleted = %v, want none", deleted)
	}
}

func TestDeleteManyGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if _, err := svc.DeleteMany(context.Background(), []string{"m1"}, false); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestDeleteManyTrashError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.listFoldersErr = errBoom
	if _, err := svc.DeleteMany(context.Background(), []string{"m1"}, false); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestDeleteManyServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	remote.deleteManyErr = errBoom
	deleted, err := svc.DeleteMany(context.Background(), []string{"m1"}, false)
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(deleted) != 0 {
		t.Errorf("deleted = %v, want none when the server batch fails", deleted)
	}
	if len(store.deletedMessages) != 0 {
		t.Errorf("cache must be untouched when the server batch fails, got %v", store.deletedMessages)
	}
}

func TestDeleteManyCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.deleteMessageErr = errBoom
	deleted, err := svc.DeleteMany(context.Background(), []string{"m1"}, false)
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	// The server delete succeeded, so the id is still reported so the UI drops it; the cache reconciles
	// on the next sync.
	if len(deleted) != 1 || deleted[0] != "m1" {
		t.Errorf("deleted = %v, want [m1] despite the cache error", deleted)
	}
}

func TestDeleteManyEmpty(t *testing.T) {
	svc, _, _, remote := newActionService()
	deleted, err := svc.DeleteMany(context.Background(), nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("deleted = %v, want none", deleted)
	}
	if len(remote.deleteManyBatches) != 0 {
		t.Errorf("no server call expected for an empty set, got %v", remote.deleteManyBatches)
	}
}

func TestMoveManyBatchesBySourceFolder(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"),
		testFolder(t, "fd", "a1", "Archive"),
	}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1"), testMessage(t, "m2", "f1")}

	moved, err := svc.MoveMany(context.Background(), []string{"m1", "m2"}, "fd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moved) != 2 {
		t.Errorf("moved = %v, want two ids", moved)
	}
	if len(remote.moveManyBatches) != 1 || len(remote.moveManyBatches[0]) != 2 {
		t.Errorf("expected one batch of two uids, got %v", remote.moveManyBatches)
	}
	if len(remote.moveManyDest) != 1 || remote.moveManyDest[0] != "Archive" {
		t.Errorf("expected move to Archive, got %v", remote.moveManyDest)
	}
	if len(store.deletedMessages) != 2 {
		t.Errorf("expected both cached rows removed, got %v", store.deletedMessages)
	}
}

func TestMoveManySkipsAlreadyInDestination(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "fd", "a1", "Archive")}
	store.messages["fd"] = []domain.MessageSummary{testMessage(t, "m1", "fd")}

	moved, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moved) != 0 {
		t.Errorf("moved = %v, want none (already in destination)", moved)
	}
	if len(remote.moveManyBatches) != 0 {
		t.Errorf("no server move expected, got %v", remote.moveManyBatches)
	}
}

func TestMoveManyGroupsBySource(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"),
		testFolder(t, "f2", "a1", "Spam"),
		testFolder(t, "fd", "a1", "Archive"),
	}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	store.messages["f2"] = []domain.MessageSummary{testMessage(t, "m2", "f2")}

	moved, err := svc.MoveMany(context.Background(), []string{"m1", "m2"}, "fd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moved) != 2 {
		t.Errorf("moved = %v, want two ids", moved)
	}
	if len(remote.moveManyBatches) != 2 {
		t.Errorf("expected two batches, one per source folder, got %v", remote.moveManyBatches)
	}
}

func TestMoveManyDestFolderError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getFolderErr = errBoom
	if _, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMoveManyDestAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	store.folders["a1"] = []domain.Folder{testFolder(t, "fd", "a1", "Archive")}
	accounts.getErr = errBoom
	if _, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMoveManyGetMessageError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "fd", "a1", "Archive")}
	store.getMessageErr = errBoom
	moved, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd")
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(moved) != 0 {
		t.Errorf("moved = %v, want none", moved)
	}
}

func TestMoveManySourceFolderMissing(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "fd", "a1", "Archive")}
	// m1's source folder f1 is deliberately absent from the folder cache.
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	moved, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd")
	if err == nil {
		t.Error("expected an error when the source folder is missing")
	}
	if len(moved) != 0 {
		t.Errorf("moved = %v, want none", moved)
	}
}

func TestMoveManyCrossAccountRejected(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "fd", "a1", "Archive")}
	store.folders["a2"] = []domain.Folder{testFolder(t, "f1", "a2", "INBOX")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	moved, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd")
	if err == nil {
		t.Error("expected an error moving across accounts")
	}
	if len(moved) != 0 {
		t.Errorf("moved = %v, want none", moved)
	}
}

func TestMoveManyServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), testFolder(t, "fd", "a1", "Archive")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.moveManyErr = errBoom
	moved, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd")
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(moved) != 0 {
		t.Errorf("moved = %v, want none when the server move fails", moved)
	}
	if len(store.deletedMessages) != 0 {
		t.Errorf("cache untouched when the server move fails, got %v", store.deletedMessages)
	}
}

func TestMoveManyCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), testFolder(t, "fd", "a1", "Archive")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	store.deleteMessageErr = errBoom
	moved, err := svc.MoveMany(context.Background(), []string{"m1"}, "fd")
	if !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(moved) != 1 || moved[0] != "m1" {
		t.Errorf("moved = %v, want [m1] despite the cache error", moved)
	}
}

func TestMoveManyEmpty(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "fd", "a1", "Archive")}
	moved, err := svc.MoveMany(context.Background(), nil, "fd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moved) != 0 {
		t.Errorf("moved = %v, want none", moved)
	}
	if len(remote.moveManyBatches) != 0 {
		t.Errorf("no server move expected, got %v", remote.moveManyBatches)
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

func junkFolder(t *testing.T, id, accountID string) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder(id, accountID, "Junk", domain.FolderJunk, 0, 0)
	if err != nil {
		t.Fatalf("build junk folder: %v", err)
	}
	return folder
}

func TestMarkJunkMovesToJunk(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], junkFolder(t, "fj", "a1"))

	if err := svc.MarkJunk(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.moveDestPaths) != 1 || remote.moveDestPaths[0] != "Junk" {
		t.Errorf("expected move to Junk, got %v", remote.moveDestPaths)
	}
	if len(store.deletedMessages) != 1 || store.deletedMessages[0] != "m1" {
		t.Errorf("expected local removal of m1, got %v", store.deletedMessages)
	}
}

func TestMarkJunkNoJunkFolder(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts) // only an inbox folder, no Junk
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, ErrNoJunkFolder) {
		t.Errorf("error = %v, want ErrNoJunkFolder", err)
	}
}

func TestMarkJunkAlreadyInJunk(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{junkFolder(t, "fj", "a1")}
	store.messages["fj"] = []domain.MessageSummary{testMessage(t, "m1", "fj")}
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, ErrAlreadyJunk) {
		t.Errorf("error = %v, want ErrAlreadyJunk", err)
	}
}

func TestMarkJunkGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkJunkGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkJunkGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], junkFolder(t, "fj", "a1"))
	accounts.getErr = errBoom
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkJunkResolveError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.listFoldersErr = errBoom
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkJunkServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], junkFolder(t, "fj", "a1"))
	remote.moveErr = errBoom
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(store.deletedMessages) != 0 {
		t.Error("cache changed despite a server failure")
	}
}

func TestMarkJunkCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], junkFolder(t, "fj", "a1"))
	store.deleteMessageErr = errBoom
	if err := svc.MarkJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkJunkRecordsVerdictKeywords(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	store.folders["a1"] = append(store.folders["a1"], junkFolder(t, "fj", "a1"))

	if err := svc.MarkJunk(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []keywordCall{
		{keyword: "$Junk", set: true}, {keyword: "Junk", set: true},
		{keyword: "$NotJunk", set: false}, {keyword: "NonJunk", set: false},
	}
	if !reflect.DeepEqual(remote.keywordCalls, want) {
		t.Errorf("keyword calls = %v, want %v", remote.keywordCalls, want)
	}
}

// seedJunkedMessage places m1 in the account's Junk folder with an Inbox available to rescue it to.
func seedJunkedMessage(t *testing.T, store *fakeMailStore, accounts *fakeAccountStore) {
	t.Helper()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), junkFolder(t, "fj", "a1")}
	store.messages["fj"] = []domain.MessageSummary{testMessage(t, "m1", "fj")}
}

func TestMarkNotJunkMovesToInboxWithVerdict(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedJunkedMessage(t, store, accounts)

	if err := svc.MarkNotJunk(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.moveDestPaths) != 1 || remote.moveDestPaths[0] != "INBOX" {
		t.Errorf("expected move to INBOX, got %v", remote.moveDestPaths)
	}
	if len(store.deletedMessages) != 1 || store.deletedMessages[0] != "m1" {
		t.Errorf("expected local removal of m1, got %v", store.deletedMessages)
	}
	want := []keywordCall{
		{keyword: "$NotJunk", set: true}, {keyword: "NonJunk", set: true},
		{keyword: "$Junk", set: false}, {keyword: "Junk", set: false},
	}
	if !reflect.DeepEqual(remote.keywordCalls, want) {
		t.Errorf("keyword calls = %v, want %v", remote.keywordCalls, want)
	}
}

func TestMarkNotJunkSucceedsWhenKeywordsRejected(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedJunkedMessage(t, store, accounts)
	remote.keywordErr = errBoom // a server that rejects custom keywords must not fail the rescue

	if err := svc.MarkNotJunk(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.moveDestPaths) != 1 || remote.moveDestPaths[0] != "INBOX" {
		t.Errorf("expected move to INBOX despite keyword errors, got %v", remote.moveDestPaths)
	}
}

func TestMarkNotJunkNotInJunk(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts) // m1 lives in the inbox
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, ErrNotInJunk) {
		t.Errorf("error = %v, want ErrNotInJunk", err)
	}
}

func TestMarkNotJunkNoInboxFolder(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{junkFolder(t, "fj", "a1")}
	store.messages["fj"] = []domain.MessageSummary{testMessage(t, "m1", "fj")}
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, ErrNoInboxFolder) {
		t.Errorf("error = %v, want ErrNoInboxFolder", err)
	}
}

func TestMarkNotJunkGetMessageError(t *testing.T) {
	svc, store, _, _ := newActionService()
	store.getMessageErr = errBoom
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkNotJunkGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedJunkedMessage(t, store, accounts)
	store.getFolderErr = errBoom
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkNotJunkGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedJunkedMessage(t, store, accounts)
	accounts.getErr = errBoom
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkNotJunkResolveError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedJunkedMessage(t, store, accounts)
	store.listFoldersErr = errBoom
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestMarkNotJunkServerError(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedJunkedMessage(t, store, accounts)
	remote.moveErr = errBoom
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
	if len(store.deletedMessages) != 0 {
		t.Error("cache changed despite a server failure")
	}
}

func TestMarkNotJunkCacheError(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedJunkedMessage(t, store, accounts)
	store.deleteMessageErr = errBoom
	if err := svc.MarkNotJunk(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}
