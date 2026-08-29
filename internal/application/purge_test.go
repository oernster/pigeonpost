package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// gmailAccount builds an account on the one provider that archives rather than deletes on an expunge.
func gmailAccount(t *testing.T, id string) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@gmail.com")
	if err != nil {
		t.Fatalf("build address: %v", err)
	}
	in, err := domain.NewServerConfig("imap.gmail.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("build incoming config: %v", err)
	}
	out, err := domain.NewServerConfig("smtp.gmail.com", 465, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("build outgoing config: %v", err)
	}
	account, err := domain.NewAccount(id, "Gmail", addr, domain.ProtocolIMAP, in, out, domain.AuthPassword)
	if err != nil {
		t.Fatalf("build account: %v", err)
	}
	return account
}

// newPurgeFixture seeds a Gmail account with an inbox and a Trash, the shape the hop needs.
func newPurgeFixture(t *testing.T) (*fakeMailStore, *fakeMailActions, domain.Account, domain.Folder) {
	t.Helper()
	store := newFakeMailStore()
	inbox := testFolder(t, "f1", "a1", "INBOX")
	store.folders["a1"] = []domain.Folder{inbox, trashFolder(t, "ft", "a1")}
	return store, &fakeMailActions{}, gmailAccount(t, "a1"), inbox
}

func TestPurgeViaTrashIsNotTakenOnAnOrdinaryServer(t *testing.T) {
	store, remote, _, inbox := newPurgeFixture(t)
	handled, err := purgeViaTrash(context.Background(), remote, store, testAccount(t, "a1"), inbox, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Error("an ordinary server deletes on an expunge, so the hop must not be taken")
	}
	if len(remote.deleteManyBatches) != 0 {
		t.Errorf("the server should not have been called: %v", remote.deleteManyBatches)
	}
}

func TestPurgeViaTrashIsNotTakenInsideTrash(t *testing.T) {
	store, remote, account, _ := newPurgeFixture(t)
	handled, err := purgeViaTrash(context.Background(), remote, store, account, trashFolder(t, "ft", "a1"), []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Error("an expunge inside Trash already deletes, so the hop must not be taken")
	}
}

func TestPurgeViaTrashIsNotTakenWithoutATrashFolder(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	store.folders["a1"] = []domain.Folder{inbox} // no Trash to hop through
	handled, err := purgeViaTrash(context.Background(), remote, store, account, inbox, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Error("with nowhere to hop to the caller must take its own route")
	}
}

func TestPurgeViaTrashSurfacesAFolderLookupFailure(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	store.listFoldersErr = errors.New("boom")
	handled, err := purgeViaTrash(context.Background(), remote, store, account, inbox, []string{"1"})
	if err == nil {
		t.Fatal("expected the lookup failure to surface")
	}
	if handled {
		t.Error("nothing was deleted, so the deletion was not handled")
	}
}

func TestPurgeViaTrashMovesThenExpungesInTrash(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	remote.moveManyNewUIDs = map[string]string{"1": "91", "2": "92"}

	handled, err := purgeViaTrash(context.Background(), remote, store, account, inbox, []string{"1", "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("the purge should have been handled")
	}
	if len(remote.deleteManyTrash) != 2 {
		t.Fatalf("expected a move then an expunge, got %v", remote.deleteManyTrash)
	}
	if remote.deleteManyTrash[0] != "Trash" {
		t.Errorf("first call should move to Trash, got %q", remote.deleteManyTrash[0])
	}
	if remote.deleteManyTrash[1] != "" {
		t.Errorf("second call should expunge, got %q", remote.deleteManyTrash[1])
	}
	if got := remote.deleteManyBatches[1]; len(got) != 2 || got[0] != "91" || got[1] != "92" {
		t.Errorf("the expunge should address the uids the messages carry in Trash, got %v", got)
	}
}

func TestPurgeViaTrashReportsAFailedMove(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	remote.deleteManyErr = errors.New("boom")
	handled, err := purgeViaTrash(context.Background(), remote, store, account, inbox, []string{"1"})
	if err == nil {
		t.Fatal("expected the move failure to surface")
	}
	if !handled {
		t.Error("the route was taken, so the caller must not also delete")
	}
}

func TestPurgeViaTrashReportsAFailedExpunge(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	remote.moveManyNewUIDs = map[string]string{"1": "91"}
	remote.deleteManyErrOnCall = 2 // the move lands; the expunge in Trash fails

	handled, err := purgeViaTrash(context.Background(), remote, store, account, inbox, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "purge") {
		t.Fatalf("expected the expunge failure to surface, got %v", err)
	}
	if !handled {
		t.Error("the messages did leave the folder, so the route was taken")
	}
}

func TestPurgeViaTrashNamesMessagesTheServerDidNotLocate(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	remote.moveManyNewUIDs = map[string]string{} // no COPYUID reply

	handled, err := purgeViaTrash(context.Background(), remote, store, account, inbox, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "could not be purged") {
		t.Fatalf("a message left in Trash must be reported, got %v", err)
	}
	if !handled {
		t.Error("the move happened, so the route was taken")
	}
	if len(remote.deleteManyTrash) != 1 {
		t.Errorf("with nothing addressable there is nothing to expunge, got %v", remote.deleteManyTrash)
	}
}

func TestPurgeViaTrashExpungesWhatItCanAndReportsTheRest(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	remote.moveManyNewUIDs = map[string]string{"1": "91"} // "2" comes back unlocated

	handled, err := purgeViaTrash(context.Background(), remote, store, account, inbox, []string{"1", "2"})
	if err == nil || !strings.Contains(err.Error(), "could not be purged") {
		t.Fatalf("expected the unlocated message to be reported, got %v", err)
	}
	if !handled {
		t.Error("the route was taken")
	}
	if got := remote.deleteManyBatches[1]; len(got) != 1 || got[0] != "91" {
		t.Errorf("the located message should still be purged, got %v", got)
	}
}

func TestDeletePermanentGoesThroughTrashOnGmail(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = gmailAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.moveManyNewUIDs = map[string]string{testMessage(t, "m1", "f1").UID(): "91"}

	if err := svc.DeletePermanent(context.Background(), "m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.deleteManyTrash) != 2 || remote.deleteManyTrash[0] != "Trash" || remote.deleteManyTrash[1] != "" {
		t.Errorf("expected a move to Trash then an expunge there, got %v", remote.deleteManyTrash)
	}
	if len(remote.deleteTrashPaths) != 0 {
		t.Errorf("the single-step route should not also have run: %v", remote.deleteTrashPaths)
	}
	if len(store.deletedMessages) != 1 || store.deletedMessages[0] != "m1" {
		t.Errorf("expected the local row to go, got %v", store.deletedMessages)
	}
}

func TestDeletePermanentOnGmailSurfacesAServerFailure(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = gmailAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.deleteManyErr = errors.New("boom")

	if err := svc.DeletePermanent(context.Background(), "m1"); err == nil {
		t.Fatal("expected the failure to surface")
	}
	if len(store.deletedMessages) != 0 {
		t.Errorf("nothing left the server, so the cache must stand: %v", store.deletedMessages)
	}
}

func TestDeletePermanentOnGmailReportsACacheFailure(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = gmailAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.moveManyNewUIDs = map[string]string{testMessage(t, "m1", "f1").UID(): "91"}
	store.deleteMessageErr = errors.New("cache boom")

	if err := svc.DeletePermanent(context.Background(), "m1"); err == nil {
		t.Fatal("expected the cache failure to surface")
	}
}

func TestDeleteManyPermanentGoesThroughTrashOnGmail(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = gmailAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.moveManyNewUIDs = map[string]string{testMessage(t, "m1", "f1").UID(): "91"}

	deleted, newIDs, err := svc.DeleteMany(context.Background(), []string{"m1"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "m1" {
		t.Errorf("expected m1 removed, got %v", deleted)
	}
	if len(newIDs) != 0 {
		t.Errorf("a permanent delete leaves nothing to undo, got %v", newIDs)
	}
	if len(remote.deleteManyTrash) != 2 || remote.deleteManyTrash[0] != "Trash" || remote.deleteManyTrash[1] != "" {
		t.Errorf("expected a move to Trash then an expunge there, got %v", remote.deleteManyTrash)
	}
}

func TestDeleteManyPermanentOnGmailReportsAFailure(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = gmailAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.deleteManyErr = errors.New("boom")

	deleted, _, err := svc.DeleteMany(context.Background(), []string{"m1"}, true)
	if err == nil {
		t.Fatal("expected the failure to surface")
	}
	if len(deleted) != 0 {
		t.Errorf("nothing left the server, so nothing was deleted: %v", deleted)
	}
}

func TestDeleteManyPermanentOnGmailReportsACacheFailure(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = gmailAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.moveManyNewUIDs = map[string]string{testMessage(t, "m1", "f1").UID(): "91"}
	store.deleteMessageErr = errors.New("cache boom")

	deleted, _, err := svc.DeleteMany(context.Background(), []string{"m1"}, true)
	if err == nil {
		t.Fatal("expected the cache failure to surface")
	}
	// The server delete succeeded, so the id still leaves the UI; the next sync reconciles the cache.
	if len(deleted) != 1 || deleted[0] != "m1" {
		t.Errorf("expected m1 reported as removed, got %v", deleted)
	}
}

func TestRuleDestroyGoesThroughTrashOnGmail(t *testing.T) {
	exec, mail, remote := execFixture(t)
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	remote.moveManyNewUIDs = map[string]string{"22": "92"}
	junk := execMessage(t, "m2", "22", "spam@bad.com")
	fetched := []domain.MessageSummary{execMessage(t, "m1", "11", "friend@good.com"), junk}
	rules := []domain.Rule{execRule(t, "nuke", "bad.com", execAction(t, domain.RuleDestroy, ""))}

	saved, err := exec.Apply(context.Background(), gmailAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		fetched, primed("m1"), true, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 1 || saved[0].ID() != "m1" {
		t.Fatalf("the destroyed message was still saved: %v", saved)
	}
	if len(remote.deleteManyTrash) != 2 || remote.deleteManyTrash[0] != "Trash" || remote.deleteManyTrash[1] != "" {
		t.Errorf("a destroying rule on Gmail must move to Trash then expunge there, got %v", remote.deleteManyTrash)
	}
	if got := remote.deleteManyBatches[1]; len(got) != 1 || got[0] != "92" {
		t.Errorf("the expunge should address the uid the message carries in Trash, got %v", got)
	}
}

func TestRuleDestroyOnGmailReportsAFailedPurge(t *testing.T) {
	exec, mail, remote := execFixture(t)
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	remote.deleteManyErr = errors.New("boom")
	junk := execMessage(t, "m2", "22", "spam@bad.com")
	rules := []domain.Rule{execRule(t, "nuke", "bad.com", execAction(t, domain.RuleDestroy, ""))}

	saved, err := exec.Apply(context.Background(), gmailAccount(t, "a1"), testFolder(t, "f1", "a1", "INBOX"),
		[]domain.MessageSummary{junk}, primed(), true, rules)
	if err == nil {
		t.Fatal("expected the failed purge to surface")
	}
	// The batch the server refused stays where it is, so the message is still saved locally.
	if len(saved) != 1 || saved[0].ID() != "m2" {
		t.Errorf("a refused destroy must leave the message in place, got %v", saved)
	}
}
