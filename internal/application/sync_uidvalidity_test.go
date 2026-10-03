package application

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The UIDVALIDITY a test inbox was last synced under and the one the server reports after renumbering it.
const (
	storedValidity      uint32 = 1
	renumberedValidity  uint32 = 2
	validityFolderID           = "f1"
	validityJunkAddress        = "spam@bad.example"
)

// junkAt is a message from the sender the destroy rule matches, under the given UID in the test inbox.
func junkAt(t *testing.T, uid string) domain.MessageSummary {
	t.Helper()
	from, err := domain.NewEmailAddress("", validityJunkAddress)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: domain.MessageIDFor(validityFolderID, uid), FolderID: validityFolderID, UID: uid,
		From: from, Subject: "s", Size: 1, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

// validityFixture is the audit's measured case: a baselined inbox whose cache holds uids 5 and 6, last
// synced under storedValidity, with a destroy rule matching both. The server now serves the same mail
// as uids 105 and 106 under serverValidity.
func validityFixture(t *testing.T, serverValidity uint32) (*fakeMailStore, *fakeMailActions, *SyncService) {
	t.Helper()
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	inbox := testFolder(t, validityFolderID, "a1", "INBOX")
	mail := newFakeMailStore()
	mail.validity = map[string]uint32{validityFolderID: storedValidity}
	mail.folders["a1"] = []domain.Folder{inbox}
	mail.messages[validityFolderID] = []domain.MessageSummary{junkAt(t, "5"), junkAt(t, "6")}
	mail.baselined = map[string]bool{validityFolderID: true}
	source := &fakeMailSource{folders: []domain.Folder{inbox},
		messagesByFolder: map[string][]domain.MessageSummary{
			validityFolderID: {junkAt(t, "105"), junkAt(t, "106")}},
		validity: map[string]uint32{validityFolderID: serverValidity},
	}
	rules := &fakeRuleStore{rules: []domain.Rule{destroyRule(t, "nuke", "bad.example")}}
	remote := &fakeMailActions{}
	svc := NewSyncService(accounts, mail, source, rules, &fakeTagSyncer{}, &fakeFlagSyncer{},
		NewRuleExecutor(mail, remote))
	return mail, remote, svc
}

// validitySyncPaths are every sync entry point that runs the rules over the inbox, the background inbox
// pass included, each reduced to its error.
func validitySyncPaths() map[string]func(*SyncService) error {
	paths := syncRulePaths()
	paths["SyncInboxes"] = func(svc *SyncService) error {
		_, err := svc.SyncInboxes(context.Background())
		return err
	}
	return paths
}

// cachedUIDs lists the UIDs the store holds for the test inbox, sorted for comparison.
func cachedUIDs(mail *fakeMailStore) []string {
	uids := make([]string, 0, len(mail.messages[validityFolderID]))
	for _, m := range mail.messages[validityFolderID] {
		uids = append(uids, m.UID())
	}
	sort.Strings(uids)
	return uids
}

// TestSyncRebaselinesARenumberedFolder is the audit's P-4 regression. The server renumbered the inbox,
// so every message came back under a new UID; under the old rule each counted as an arrival and the
// destroy rule deleted the backlog it was promised never to touch. A changed UIDVALIDITY must instead
// rebaseline the folder: no destroy, the cache refreshed to the new UIDs and the new value recorded.
func TestSyncRebaselinesARenumberedFolder(t *testing.T) {
	for name, run := range validitySyncPaths() {
		t.Run(name, func(t *testing.T) {
			mail, remote, svc := validityFixture(t, renumberedValidity)

			if err := run(svc); err != nil {
				t.Fatalf("sync: %v", err)
			}
			if len(remote.deleteManyBatches) != 0 || len(remote.moveManyBatches) != 0 {
				t.Errorf("rules acted on renumbered mail: destroyBatches=%v moveBatches=%v",
					remote.deleteManyBatches, remote.moveManyBatches)
			}
			if got, want := cachedUIDs(mail), []string{"105", "106"}; !reflect.DeepEqual(got, want) {
				t.Errorf("cached uids = %v, want %v", got, want)
			}
			if got := mail.validity[validityFolderID]; got != renumberedValidity {
				t.Errorf("stored UIDVALIDITY = %d, want %d", got, renumberedValidity)
			}
		})
	}
}

// TestSyncInboxesAnnouncesNoRenumberedMail pins that a renumbered inbox raises no desktop notification:
// the messages are the ones the user already had, not mail that arrived.
func TestSyncInboxesAnnouncesNoRenumberedMail(t *testing.T) {
	_, _, svc := validityFixture(t, renumberedValidity)

	arrived, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(arrived) != 0 {
		t.Errorf("announced %d renumbered messages as new", len(arrived))
	}
}

// TestSyncStillActsOnArrivalsUnderAnUnchangedValidity is the control that proves the fixture bites:
// the same new UIDs under the stored UIDVALIDITY are genuine arrivals; the destroy rule removes them.
func TestSyncStillActsOnArrivalsUnderAnUnchangedValidity(t *testing.T) {
	for name, run := range validitySyncPaths() {
		t.Run(name, func(t *testing.T) {
			_, remote, svc := validityFixture(t, storedValidity)

			if err := run(svc); err != nil {
				t.Fatalf("sync: %v", err)
			}
			if want := [][]string{{"105", "106"}}; !reflect.DeepEqual(remote.deleteManyBatches, want) {
				t.Errorf("destroyBatches = %v, want %v", remote.deleteManyBatches, want)
			}
		})
	}
}

// TestSyncRecordsAFirstValidity pins the first sight: with nothing stored the value is simply recorded
// and the rules behave exactly as before, so genuine arrivals on a baselined folder are still acted on.
func TestSyncRecordsAFirstValidity(t *testing.T) {
	for name, run := range validitySyncPaths() {
		t.Run(name, func(t *testing.T) {
			mail, remote, svc := validityFixture(t, renumberedValidity)
			delete(mail.validity, validityFolderID)

			if err := run(svc); err != nil {
				t.Fatalf("sync: %v", err)
			}
			if got := mail.validity[validityFolderID]; got != renumberedValidity {
				t.Errorf("stored UIDVALIDITY = %d, want %d", got, renumberedValidity)
			}
			if len(remote.deleteManyBatches) != 1 {
				t.Errorf("destroyBatches = %v, want the arrivals acted on as before", remote.deleteManyBatches)
			}
		})
	}
}

// TestSyncReportsAValidityReadThatFailed pins that an unreadable stored UIDVALIDITY stops the folder
// rather than defaulting to "unchanged", which would hand a renumbered backlog to the rules.
func TestSyncReportsAValidityReadThatFailed(t *testing.T) {
	for name, run := range syncRulePaths() {
		t.Run(name, func(t *testing.T) {
			mail, remote, svc := validityFixture(t, renumberedValidity)
			mail.validityErr = errBoom

			if err := run(svc); !errors.Is(err, errBoom) {
				t.Errorf("error = %v, want the failed read wrapped", err)
			}
			if len(remote.deleteManyBatches) != 0 {
				t.Errorf("rules acted despite the failed read: %v", remote.deleteManyBatches)
			}
		})
	}
}

// TestSyncInboxesSkipsAFolderWhoseValidityCannotBeRead pins the background pass's half: the folder is
// skipped like any other per-folder failure, so nothing is destroyed or announced.
func TestSyncInboxesSkipsAFolderWhoseValidityCannotBeRead(t *testing.T) {
	mail, remote, svc := validityFixture(t, renumberedValidity)
	mail.validityErr = errBoom

	arrived, err := svc.SyncInboxes(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(arrived) != 0 || len(remote.deleteManyBatches) != 0 {
		t.Errorf("arrived=%d destroyBatches=%v, want the folder skipped", len(arrived), remote.deleteManyBatches)
	}
}

// TestSyncReportsAValidityRecordThatFailed pins that a UIDVALIDITY that could not be stored fails the
// sync rather than passing quietly; the cache is saved first, so no mail is lost to it.
func TestSyncReportsAValidityRecordThatFailed(t *testing.T) {
	for name, run := range syncRulePaths() {
		t.Run(name, func(t *testing.T) {
			mail, _, svc := validityFixture(t, renumberedValidity)
			mail.setValidityErr = errBoom

			if err := run(svc); !errors.Is(err, errBoom) {
				t.Errorf("error = %v, want the failed record wrapped", err)
			}
			if got, want := cachedUIDs(mail), []string{"105", "106"}; !reflect.DeepEqual(got, want) {
				t.Errorf("cached uids = %v, want %v", got, want)
			}
		})
	}
}

// TestSyncReportsAValidityFetchThatFailed pins that a fetch failure on the UIDVALIDITY-reporting path is
// surfaced like any other fetch failure.
func TestSyncReportsAValidityFetchThatFailed(t *testing.T) {
	for name, run := range syncRulePaths() {
		t.Run(name, func(t *testing.T) {
			_, _, svc := validityFixture(t, renumberedValidity)
			svc.source.(*fakeMailSource).fetchMessagesErr = errBoom

			if err := run(svc); !errors.Is(err, errBoom) {
				t.Errorf("error = %v, want the failed fetch wrapped", err)
			}
		})
	}
}

// TestSyncWithoutAReportedValidityKeepsTheOldBehaviour pins that a source reporting no UIDVALIDITY (POP3)
// neither fails nor rebaselines: the rules see arrivals exactly as before and nothing is recorded over
// the folder's stored value.
func TestSyncWithoutAReportedValidityKeepsTheOldBehaviour(t *testing.T) {
	mail, remote, svc := validityFixture(t, unknownUIDValidity)

	if err := svc.SyncFolder(context.Background(), validityFolderID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(remote.deleteManyBatches) != 1 {
		t.Errorf("destroyBatches = %v, want the old behaviour", remote.deleteManyBatches)
	}
	if got := mail.validity[validityFolderID]; got != storedValidity {
		t.Errorf("recorded UIDVALIDITY = %d, want %d left in place", got, storedValidity)
	}
}
