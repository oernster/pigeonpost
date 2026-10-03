package imap

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The source and destination mailboxes the move and expunge tests use. Neither path holds a space, so
// the scripted server can read them as single arguments.
const (
	inboxPath   = "INBOX"
	archivePath = "Archive"
	trashPath   = "Trash"
	// chosenUID is the message the user acts on; bystanderUID is one another client flagged \Deleted
	// without expunging, which no action of ours may remove.
	chosenUID    = 7
	bystanderUID = 9
	// overQuota is the refusal a full destination answers a COPY with.
	overQuota = "[OVERQUOTA] mailbox is over quota"
)

func inboxFolder(t *testing.T) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder("folder-inbox", "acct", inboxPath, domain.FolderInbox, 0, 1)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	return folder
}

// action is one way the adapter can relocate or remove the chosen message, run against a server.
type action struct {
	name string
	run  func(context.Context, domain.Account, domain.Folder) error
}

func relocations() []action {
	uid := "7"
	return []action{
		{"Move", func(ctx context.Context, a domain.Account, f domain.Folder) error {
			_, err := fakeSource().Move(ctx, a, f, uid, archivePath)
			return err
		}},
		{"MoveMany", func(ctx context.Context, a domain.Account, f domain.Folder) error {
			_, err := fakeSource().MoveMany(ctx, a, f, []string{uid}, archivePath)
			return err
		}},
		{"Delete to Trash", func(ctx context.Context, a domain.Account, f domain.Folder) error {
			_, err := fakeSource().Delete(ctx, a, f, uid, trashPath)
			return err
		}},
		{"DeleteMany to Trash", func(ctx context.Context, a domain.Account, f domain.Folder) error {
			_, err := fakeSource().DeleteMany(ctx, a, f, []string{uid}, trashPath)
			return err
		}},
	}
}

func permanentDeletes() []action {
	uid := "7"
	return []action{
		{"Delete", func(ctx context.Context, a domain.Account, f domain.Folder) error {
			_, err := fakeSource().Delete(ctx, a, f, uid, "")
			return err
		}},
		{"DeleteMany", func(ctx context.Context, a domain.Account, f domain.Folder) error {
			_, err := fakeSource().DeleteMany(ctx, a, f, []string{uid}, "")
			return err
		}},
	}
}

// assertNoRemoval checks that nothing which could remove mail reached the wire and that the source still
// holds both the chosen message and the bystander.
func assertNoRemoval(t *testing.T, srv *mailboxServer) {
	t.Helper()
	for _, command := range []string{"UID STORE", "EXPUNGE", "UID EXPUNGE"} {
		if sent := srv.commands.matching(command); len(sent) != 0 {
			t.Errorf("%s reached the wire: %v", command, sent)
		}
	}
	if got, want := srv.uids(inboxPath), []uint32{chosenUID, bystanderUID}; !reflect.DeepEqual(got, want) {
		t.Errorf("source holds %v, want %v untouched", got, want)
	}
}

// P-2 regression: on a server without MOVE a refused COPY must stop the move there. go-imap's fallback
// pipelines STORE \Deleted and EXPUNGE behind the COPY without reading its answer, which destroyed the
// message while the app reported it could not be moved.
func TestARefusedCopySendsNoStoreOrExpungeAndKeepsTheMessage(t *testing.T) {
	for _, caps := range []string{"UIDPLUS", ""} {
		for _, act := range relocations() {
			t.Run(act.name+" caps="+caps, func(t *testing.T) {
				srv := &mailboxServer{caps: caps, copyRefusal: overQuota}
				host, port := newMailboxServer(t, srv, inboxPath, chosenUID, bystanderUID)
				if err := act.run(context.Background(), fakeAccount(t, host, port), inboxFolder(t)); err == nil {
					t.Fatal("a refused copy was reported as a success")
				}
				assertNoRemoval(t, srv)
			})
		}
	}
}

// Without MOVE but with UIDPLUS a move is UID COPY, then a STORE and a UID EXPUNGE of exactly the
// copied UIDs, so a message another client flagged \Deleted stays where it is.
func TestTheCopyFallbackExpungesOnlyTheMovedUIDs(t *testing.T) {
	for _, act := range relocations() {
		t.Run(act.name, func(t *testing.T) {
			srv := &mailboxServer{caps: "UIDPLUS"}
			host, port := newMailboxServer(t, srv, inboxPath, chosenUID, bystanderUID)
			srv.flagDeleted(inboxPath, bystanderUID)
			if err := act.run(context.Background(), fakeAccount(t, host, port), inboxFolder(t)); err != nil {
				t.Fatalf("%s: %v", act.name, err)
			}
			if got := srv.uids(inboxPath); !reflect.DeepEqual(got, []uint32{bystanderUID}) {
				t.Errorf("source holds %v, want only the bystander %d", got, bystanderUID)
			}
			if plain := srv.commands.matching("EXPUNGE"); len(plain) != 0 {
				t.Errorf("a plain EXPUNGE reached the wire: %v", plain)
			}
			if uidExpunges := srv.commands.matching("UID EXPUNGE"); !reflect.DeepEqual(uidExpunges, []string{"UID EXPUNGE 7"}) {
				t.Errorf("uid expunges = %v, want exactly the moved uid", uidExpunges)
			}
		})
	}
}

func TestTheCopyFallbackReportsTheDestinationUID(t *testing.T) {
	srv := &mailboxServer{caps: "UIDPLUS"}
	host, port := newMailboxServer(t, srv, inboxPath, chosenUID)
	dest, err := fakeSource().Move(context.Background(), fakeAccount(t, host, port), inboxFolder(t), "7", archivePath)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if want := srv.uids(archivePath); len(want) != 1 || dest != "1" {
		t.Errorf("destination uid = %q, archive holds %v", dest, want)
	}
}

// P-8: a permanent delete with UIDPLUS expunges the chosen UIDs only, never with a plain EXPUNGE that
// would also destroy every message another client flagged \Deleted.
func TestPermanentDeleteExpungesOnlyTheChosenUIDs(t *testing.T) {
	for _, act := range permanentDeletes() {
		t.Run(act.name, func(t *testing.T) {
			srv := &mailboxServer{caps: "UIDPLUS"}
			host, port := newMailboxServer(t, srv, inboxPath, chosenUID, bystanderUID)
			srv.flagDeleted(inboxPath, bystanderUID)
			if err := act.run(context.Background(), fakeAccount(t, host, port), inboxFolder(t)); err != nil {
				t.Fatalf("%s: %v", act.name, err)
			}
			if plain := srv.commands.matching("EXPUNGE"); len(plain) != 0 {
				t.Errorf("a plain EXPUNGE reached the wire: %v", plain)
			}
			if uidExpunges := srv.commands.matching("UID EXPUNGE"); !reflect.DeepEqual(uidExpunges, []string{"UID EXPUNGE 7"}) {
				t.Errorf("uid expunges = %v, want exactly the chosen uid", uidExpunges)
			}
			if got := srv.uids(inboxPath); !reflect.DeepEqual(got, []uint32{bystanderUID}) {
				t.Errorf("source holds %v, want the bystander %d to survive", got, bystanderUID)
			}
		})
	}
}

// A server with neither MOVE nor UIDPLUS can only remove mail with a plain EXPUNGE, which takes every
// \Deleted message in the folder. The adapter refuses before sending anything that changes the mailbox.
func TestWithoutUIDPlusNothingIsCopiedFlaggedOrExpunged(t *testing.T) {
	for _, act := range append(relocations(), permanentDeletes()...) {
		t.Run(act.name, func(t *testing.T) {
			srv := &mailboxServer{}
			host, port := newMailboxServer(t, srv, inboxPath, chosenUID, bystanderUID)
			err := act.run(context.Background(), fakeAccount(t, host, port), inboxFolder(t))
			if !errors.Is(err, ErrNoSelectiveExpunge) {
				t.Fatalf("err = %v, want ErrNoSelectiveExpunge", err)
			}
			if copies := srv.commands.matching("UID COPY"); len(copies) != 0 {
				t.Errorf("a COPY reached the wire: %v", copies)
			}
			assertNoRemoval(t, srv)
		})
	}
}

// The bulk fallback keeps per-chunk results: a COPY refused on the second chunk leaves the first chunk
// moved and reported, with the error naming the refused chunk.
func TestMoveManyFallbackKeepsTheChunksMovedBeforeARefusal(t *testing.T) {
	srv := &mailboxServer{caps: "UIDPLUS", copyRefusal: overQuota, refuseCopyFrom: 2}
	seeded := make([]uint32, 0, bulkBatchSize+1)
	for u := uint32(1); u <= bulkBatchSize+1; u++ {
		seeded = append(seeded, u)
	}
	host, port := newMailboxServer(t, srv, inboxPath, seeded...)
	moved, err := fakeSource().MoveMany(context.Background(), fakeAccount(t, host, port), inboxFolder(t), uidRange(1, bulkBatchSize+1), archivePath)
	if err == nil || !strings.Contains(err.Error(), "OVERQUOTA") {
		t.Fatalf("err = %v, want the second chunk's refusal", err)
	}
	if len(moved) != bulkBatchSize {
		t.Errorf("moved = %d entries, want the first chunk's %d", len(moved), bulkBatchSize)
	}
	if got := srv.uids(inboxPath); !reflect.DeepEqual(got, []uint32{bulkBatchSize + 1}) {
		t.Errorf("source holds %v, want only the refused chunk", got)
	}
}

// A copy that landed but could not be flagged \Deleted is still in the source: the error says so and no
// expunge follows, so the message is duplicated rather than lost.
func TestARefusedStoreAfterTheCopyLeavesTheSourceAlone(t *testing.T) {
	srv := &mailboxServer{caps: "UIDPLUS", storeRefusal: "[CANNOT] read-only"}
	host, port := newMailboxServer(t, srv, inboxPath, chosenUID)
	_, err := fakeSource().Move(context.Background(), fakeAccount(t, host, port), inboxFolder(t), "7", archivePath)
	if err == nil || !strings.Contains(err.Error(), "copied") {
		t.Fatalf("err = %v, want one saying the copy landed", err)
	}
	if expunges := srv.commands.matching("UID EXPUNGE"); len(expunges) != 0 {
		t.Errorf("an expunge followed a refused store: %v", expunges)
	}
	if got := srv.uids(inboxPath); !reflect.DeepEqual(got, []uint32{chosenUID}) {
		t.Errorf("source holds %v, want the message kept", got)
	}
}

// MoveAllMessages (merging a stray sent folder) takes the same fallback over every UID in the folder.
func TestMoveAllMessagesFallbackMovesEveryUIDWithoutAPlainExpunge(t *testing.T) {
	srv := &mailboxServer{caps: "UIDPLUS"}
	host, port := newMailboxServer(t, srv, inboxPath, chosenUID, bystanderUID)
	if err := fakeSource().MoveAllMessages(context.Background(), fakeAccount(t, host, port), inboxPath, archivePath); err != nil {
		t.Fatalf("MoveAllMessages: %v", err)
	}
	if got := srv.uids(inboxPath); len(got) != 0 {
		t.Errorf("source still holds %v", got)
	}
	if got := sortedUIDs(srv.uids(archivePath)); len(got) != 2 {
		t.Errorf("archive holds %v, want both messages", got)
	}
	if plain := srv.commands.matching("EXPUNGE"); len(plain) != 0 {
		t.Errorf("a plain EXPUNGE reached the wire: %v", plain)
	}
}

func TestMoveAllMessagesWithoutUIDPlusChangesNothing(t *testing.T) {
	srv := &mailboxServer{}
	host, port := newMailboxServer(t, srv, inboxPath, chosenUID, bystanderUID)
	err := fakeSource().MoveAllMessages(context.Background(), fakeAccount(t, host, port), inboxPath, archivePath)
	if !errors.Is(err, ErrNoSelectiveExpunge) {
		t.Fatalf("err = %v, want ErrNoSelectiveExpunge", err)
	}
	assertNoRemoval(t, srv)
}
