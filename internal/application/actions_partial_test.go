package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// partialMailActions is a hand-written MailActions whose batched move and delete land part of a batch and
// then fail: the shape the IMAP adapter returns when one UID chunk is accepted and a later one refused,
// the accepted UIDs mapped to where they landed together with the error. A delete with no trash path (the
// expunge inside Trash that a purge issues second) succeeds and is recorded, unless refuseExpunge makes
// it the partly refused call instead (an ordinary server's permanent delete).
type partialMailActions struct {
	*fakeMailActions
	landed        map[string]string
	err           error
	refuseExpunge bool
	purged        [][]string
	movedTo       []string
}

func (p *partialMailActions) MoveMany(_ context.Context, _ domain.Account, _ domain.Folder, _ []string, destPath string) (map[string]string, error) {
	p.movedTo = append(p.movedTo, destPath)
	return p.landed, p.err
}

func (p *partialMailActions) DeleteMany(_ context.Context, _ domain.Account, _ domain.Folder, uids []string, trashPath string) (map[string]string, error) {
	if trashPath == "" && !p.refuseExpunge {
		p.purged = append(p.purged, uids)
		return nil, nil
	}
	return p.landed, p.err
}

// newPartialService wires a service over two messages (UIDs 10 and 20) in INBOX, an Archive and a Trash,
// with a remote that lands only UID 10 before refusing the rest.
func newPartialService(t *testing.T, account domain.Account) (*MessageActionService, *fakeMailStore, *partialMailActions) {
	t.Helper()
	store := newFakeMailStore()
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = account
	store.folders["a1"] = []domain.Folder{
		testFolder(t, "f1", "a1", "INBOX"), testFolder(t, "fd", "a1", "Archive"), trashFolder(t, "ft", "a1"),
	}
	store.messages["f1"] = []domain.MessageSummary{
		testMessageUID(t, "m1", "f1", "10"),
		testMessageUID(t, "m2", "f1", "20"),
	}
	remote := &partialMailActions{
		fakeMailActions: &fakeMailActions{},
		landed:          map[string]string{"10": "100"},
		err:             errBoom,
	}
	return NewMessageActionService(store, accounts, remote), store, remote
}

// assertOnlyFirstLanded checks the outcome every partial batch must share: the error surfaces; only the
// message the server accepted is reported as acted on; only its cache row is dropped.
func assertOnlyFirstLanded(t *testing.T, store *fakeMailStore, acted []string, err error) {
	t.Helper()
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want the refusal to surface", err)
	}
	if !reflect.DeepEqual(acted, []string{"m1"}) {
		t.Errorf("acted = %v, want [m1]: the server accepted UID 10 before refusing", acted)
	}
	if !reflect.DeepEqual(store.deletedMessages, []string{"m1"}) {
		t.Errorf("cache rows dropped = %v, want [m1] so it does not return at a UID that is gone", store.deletedMessages)
	}
}

func TestMoveManyKeepsWhatLandedBeforeARefusal(t *testing.T) {
	svc, store, _ := newPartialService(t, testAccount(t, "a1"))

	moved, newIDs, err := svc.MoveMany(context.Background(), []string{"m1", "m2"}, "fd")

	assertOnlyFirstLanded(t, store, moved, err)
	if want := map[string]string{"m1": domain.MessageIDFor("fd", "100")}; !reflect.DeepEqual(newIDs, want) {
		t.Errorf("newIDs = %v, want %v so undo covers the moved part", newIDs, want)
	}
}

func TestDeleteManyKeepsWhatLandedInTrashBeforeARefusal(t *testing.T) {
	svc, store, _ := newPartialService(t, testAccount(t, "a1"))

	deleted, newIDs, err := svc.DeleteMany(context.Background(), []string{"m1", "m2"}, false)

	assertOnlyFirstLanded(t, store, deleted, err)
	if want := map[string]string{"m1": domain.MessageIDFor("ft", "100")}; !reflect.DeepEqual(newIDs, want) {
		t.Errorf("newIDs = %v, want %v so undo covers the deleted part", newIDs, want)
	}
}

func TestDeleteManyPermanentOnGmailPurgesWhatLandedBeforeARefusal(t *testing.T) {
	svc, store, remote := newPartialService(t, gmailAccount(t, "a1"))

	deleted, newIDs, err := svc.DeleteMany(context.Background(), []string{"m1", "m2"}, true)

	assertOnlyFirstLanded(t, store, deleted, err)
	if len(newIDs) != 0 {
		t.Errorf("newIDs = %v, want none: a permanent delete leaves nothing to undo", newIDs)
	}
	if !reflect.DeepEqual(remote.purged, [][]string{{"100"}}) {
		t.Errorf("purged = %v, want [[100]]: the landed message must still be expunged from Trash", remote.purged)
	}
}

func TestPurgeViaTrashReportsWhatLeftTheFolderBeforeARefusal(t *testing.T) {
	store, _, account, inbox := newPurgeFixture(t)
	remote := &partialMailActions{fakeMailActions: &fakeMailActions{}, landed: map[string]string{"1": "91"}, err: errBoom}

	handled, left, err := purgeViaTrashReporting(context.Background(), remote, store, account, inbox, []string{"1", "2"})

	if !handled || !errors.Is(err, errBoom) {
		t.Fatalf("handled=%v err=%v, want the route taken and the refusal surfaced", handled, err)
	}
	if !reflect.DeepEqual(left, []string{"1"}) {
		t.Errorf("left = %v, want [1]: only UID 1 reached Trash", left)
	}
}

func TestPurgeViaTrashCountsAnUnlocatedMessageAsGoneFromTheFolder(t *testing.T) {
	store, remote, account, inbox := newPurgeFixture(t)
	remote.moveManyNewUIDs = map[string]string{} // the move succeeded but no COPYUID came back

	_, left, err := purgeViaTrashReporting(context.Background(), remote, store, account, inbox, []string{"1"})

	if err == nil {
		t.Fatal("a message stranded in Trash must still be reported")
	}
	if !reflect.DeepEqual(left, []string{"1"}) {
		t.Errorf("left = %v, want [1]: the message left the folder even though it could not be purged", left)
	}
}

func TestDeletePermanentOnGmailDropsTheRowWhenOnlyThePurgeFails(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	accounts.accounts["a1"] = gmailAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), trashFolder(t, "ft", "a1")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
	remote.moveManyNewUIDs = map[string]string{testMessage(t, "m1", "f1").UID(): "91"}
	remote.deleteManyErrOnCall = 2 // the move to Trash lands; the expunge there fails

	if err := svc.DeletePermanent(context.Background(), "m1"); err == nil {
		t.Fatal("expected the failed purge to surface")
	}
	if !reflect.DeepEqual(store.deletedMessages, []string{"m1"}) {
		t.Errorf("cache rows dropped = %v, want [m1]: the message is no longer in INBOX", store.deletedMessages)
	}
}
