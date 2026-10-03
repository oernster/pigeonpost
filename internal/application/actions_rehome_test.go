package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// spamFrom builds a message from a sender the destroy rule matches.
func spamFrom(t *testing.T, id, folderID, uid string) domain.MessageSummary {
	t.Helper()
	from, err := domain.NewEmailAddress("", "spam@bad.example")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: uid, From: from, Subject: "rescued", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

// rehomeFixture is an established inbox f1 (baselined, with a destroy rule for bad.example), a Junk fj
// and an Archive fd, the spam message j1 (UID 5) waiting in Junk, plus an action service over the same
// store whose server reports COPYUID 9 for every move.
func rehomeFixture(t *testing.T) (*MessageActionService, *fakeMailStore, *fakeMailSource, *SyncService, *fakeMailActions) {
	t.Helper()
	remote := &fakeMailActions{moveNewUID: "9", moveManyNewUIDs: map[string]string{"5": "9"}}
	mail, source, rules, sync := syncRuleFixture(t, remote)
	mail.folders["a1"] = append(mail.folders["a1"], junkFolder(t, "fj", "a1"), testFolder(t, "fd", "a1", "Archive"))
	mail.messages["fj"] = []domain.MessageSummary{spamFrom(t, "j1", "fj", "5")}
	rules.rules = []domain.Rule{destroyRule(t, "nuke", "bad.example")}
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	return NewMessageActionService(mail, accounts, remote), mail, source, sync, remote
}

// syncAfterMoveIntoInbox has the server list the moved message in the inbox under its new UID beside the
// one already known, as it will after the move, then syncs the inbox.
func syncAfterMoveIntoInbox(t *testing.T, source *fakeMailSource, sync *SyncService) {
	t.Helper()
	source.messagesByFolder = map[string][]domain.MessageSummary{
		"f1": {testMessage(t, "known", "f1"), spamFrom(t, domain.MessageIDFor("f1", "9"), "f1", "9")},
	}
	if err := sync.SyncFolder(context.Background(), "f1"); err != nil {
		t.Fatalf("sync: %v", err)
	}
}

// assertNotDestroyedAsArrival is the reproduction: a message the user moved into the inbox is not new
// mail, so a destroy rule must not take it.
func assertNotDestroyedAsArrival(t *testing.T, remote *fakeMailActions) {
	t.Helper()
	if len(remote.deleteManyBatches) != 0 {
		t.Errorf("the rule destroyed %v: the message moved into the inbox was read as an arrival", remote.deleteManyBatches)
	}
}

func TestMarkNotJunkRescueIsNotDestroyedAsAnArrival(t *testing.T) {
	svc, _, source, sync, remote := rehomeFixture(t)
	if _, err := svc.MarkNotJunk(context.Background(), "j1"); err != nil {
		t.Fatalf("MarkNotJunk: %v", err)
	}
	syncAfterMoveIntoInbox(t, source, sync)
	assertNotDestroyedAsArrival(t, remote)
}

func TestMoveIntoTheInboxIsNotDestroyedAsAnArrival(t *testing.T) {
	svc, _, source, sync, remote := rehomeFixture(t)
	if _, err := svc.Move(context.Background(), "j1", "f1"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	syncAfterMoveIntoInbox(t, source, sync)
	assertNotDestroyedAsArrival(t, remote)
}

func TestMoveManyIntoTheInboxIsNotDestroyedAsAnArrival(t *testing.T) {
	svc, _, source, sync, remote := rehomeFixture(t)
	if _, _, err := svc.MoveMany(context.Background(), []string{"j1"}, "f1"); err != nil {
		t.Fatalf("MoveMany: %v", err)
	}
	syncAfterMoveIntoInbox(t, source, sync)
	assertNotDestroyedAsArrival(t, remote)
}

func TestARehomedMessageKeepsItsDataUnderTheNewID(t *testing.T) {
	svc, store, _, _, _ := rehomeFixture(t)
	if _, err := svc.Move(context.Background(), "j1", "fd"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if len(store.rehomed) != 1 {
		t.Fatalf("rehomed = %v, want the one moved message", store.rehomed)
	}
	got := store.rehomed[0]
	if got.ID() != domain.MessageIDFor("fd", "9") || got.FolderID() != "fd" || got.UID() != "9" {
		t.Errorf("rehomed as %s in %s uid %s, want %s in fd uid 9", got.ID(), got.FolderID(), got.UID(), domain.MessageIDFor("fd", "9"))
	}
	if got.Subject() != "rescued" || got.From().Address() != "spam@bad.example" {
		t.Errorf("rehomed message lost its data: subject %q from %q", got.Subject(), got.From().Address())
	}
}

func TestMarkJunkRehomesIntoJunk(t *testing.T) {
	svc, store, _, _, _ := rehomeFixture(t)
	store.messages["f1"] = append(store.messages["f1"], spamFrom(t, "i1", "f1", "5"))
	if _, err := svc.MarkJunk(context.Background(), "i1"); err != nil {
		t.Fatalf("MarkJunk: %v", err)
	}
	if len(store.rehomed) != 1 || store.rehomed[0].FolderID() != "fj" {
		t.Errorf("rehomed = %v, want the message filed in Junk", store.rehomed)
	}
}

// rehomeMoves are the two move entry points, each moving j1 into the Archive fd.
func rehomeMoves() map[string]func(*MessageActionService) error {
	return map[string]func(*MessageActionService) error{
		"Move": func(s *MessageActionService) error { _, err := s.Move(context.Background(), "j1", "fd"); return err },
		"MoveMany": func(s *MessageActionService) error {
			_, _, err := s.MoveMany(context.Background(), []string{"j1"}, "fd")
			return err
		},
	}
}

// Without COPYUID the destination id is unknown, so nothing is filed: the destination learns of the
// message at its next sync, as it did before.
func TestAMoveWithNoReportedUIDRehomesNothing(t *testing.T) {
	for name, run := range rehomeMoves() {
		t.Run(name, func(t *testing.T) {
			svc, store, _, _, remote := rehomeFixture(t)
			remote.moveNewUID = ""
			remote.moveManyNewUIDs = map[string]string{}
			if err := run(svc); err != nil {
				t.Fatalf("move: %v", err)
			}
			if len(store.rehomed) != 0 {
				t.Errorf("rehomed = %v, want nothing filed without a destination uid", store.rehomed)
			}
		})
	}
}

func TestARehomeFailureIsReported(t *testing.T) {
	for name, run := range map[string]func(*MessageActionService) error{
		"Move": func(s *MessageActionService) error { _, err := s.Move(context.Background(), "j1", "fd"); return err },
		"MoveMany": func(s *MessageActionService) error {
			_, _, err := s.MoveMany(context.Background(), []string{"j1"}, "fd")
			return err
		},
		"MarkNotJunk": func(s *MessageActionService) error { _, err := s.MarkNotJunk(context.Background(), "j1"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			svc, store, _, _, _ := rehomeFixture(t)
			store.rehomeErr = errBoom
			if err := run(svc); !errors.Is(err, errBoom) {
				t.Errorf("err = %v, want the failed rehome reported", err)
			}
		})
	}
}

// A delete to Trash is a move: where the server reported the Trash UID the message is filed there, so
// Trash lists it at once and an undo can find it.
func TestDeleteToTrashFilesTheMessageInTrash(t *testing.T) {
	for name, run := range map[string]func(*MessageActionService) error{
		"Delete": func(s *MessageActionService) error { _, err := s.Delete(context.Background(), "j1"); return err },
		"DeleteMany": func(s *MessageActionService) error {
			_, _, err := s.DeleteMany(context.Background(), []string{"j1"}, false)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc, store, _, _, _ := rehomeFixture(t)
			store.folders["a1"] = append(store.folders["a1"], trashFolder(t, "ft", "a1"))
			if err := run(svc); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if len(store.rehomed) != 1 || store.rehomed[0].ID() != domain.MessageIDFor("ft", "9") {
				t.Errorf("rehomed = %v, want j1 filed in Trash as %s", store.rehomed, domain.MessageIDFor("ft", "9"))
			}
		})
	}
}

func TestABlankReportedUIDIsReportedNotFiled(t *testing.T) {
	for name, run := range rehomeMoves() {
		t.Run(name, func(t *testing.T) {
			svc, store, _, _, remote := rehomeFixture(t)
			remote.moveNewUID = " "
			remote.moveManyNewUIDs = map[string]string{"5": " "}
			if err := run(svc); err == nil {
				t.Error("a destination uid that cannot address a message was accepted")
			}
			if len(store.rehomed) != 0 {
				t.Errorf("rehomed = %v, want nothing filed under a blank uid", store.rehomed)
			}
		})
	}
}
