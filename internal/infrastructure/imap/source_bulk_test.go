package imap

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// uidRange answers the uid strings first to last inclusive.
func uidRange(first, last int) []string {
	uids := make([]string, 0, last-first+1)
	for u := first; u <= last; u++ {
		uids = append(uids, strconv.Itoa(u))
	}
	return uids
}

func TestUIDChunksSplitAtTheBatchSize(t *testing.T) {
	chunks, err := uidChunks(uidRange(1, bulkBatchSize+1))
	if err != nil {
		t.Fatalf("uidChunks: %v", err)
	}
	if len(chunks) != 2 || chunks[0].count != bulkBatchSize || chunks[1].count != 1 {
		t.Fatalf("chunks = %d (counts %v), want %d then 1", len(chunks), chunkCounts(chunks), bulkBatchSize)
	}
}

func TestUIDChunksRefuseAMalformedUID(t *testing.T) {
	if _, err := uidChunks([]string{"1", "not-a-uid"}); err == nil {
		t.Fatal("expected an error for a malformed uid")
	}
}

func chunkCounts(chunks []uidChunk) []int {
	counts := make([]int, 0, len(chunks))
	for _, c := range chunks {
		counts = append(counts, c.count)
	}
	return counts
}

// SetSeenMany must cost one login and one SELECT for the whole folder, whatever the selection's size,
// with the UIDs carried in chunked silent STOREs of \Seen. The per-message path it replaces logged in
// once per message.
func TestSetSeenManyUsesOneConnectionForTheFolder(t *testing.T) {
	log := &commandLog{}
	host, port := listenFake(t, script{commands: log})
	source := fakeSource()
	uids := uidRange(1, bulkBatchSize+1)

	if err := source.SetSeenMany(context.Background(), fakeAccount(t, host, port), fakeFolder(t), uids, true); err != nil {
		t.Fatalf("SetSeenMany: %v", err)
	}
	if logins := log.matching("LOGIN"); len(logins) != 1 {
		t.Errorf("logins = %d, want 1", len(logins))
	}
	if selects := log.matching("SELECT"); len(selects) != 1 {
		t.Errorf("selects = %d, want 1", len(selects))
	}
	stores := log.matching("UID STORE")
	if len(stores) != 2 {
		t.Fatalf("stores = %v, want 2 chunks", stores)
	}
	for _, store := range stores {
		if !strings.Contains(store, `+FLAGS.SILENT (\Seen)`) {
			t.Errorf("store %q does not add \\Seen silently", store)
		}
	}
}

func TestSetSeenManyClearsSeen(t *testing.T) {
	log := &commandLog{}
	host, port := listenFake(t, script{commands: log})

	if err := fakeSource().SetSeenMany(context.Background(), fakeAccount(t, host, port), fakeFolder(t), []string{"7"}, false); err != nil {
		t.Fatalf("SetSeenMany: %v", err)
	}
	stores := log.matching("UID STORE")
	if len(stores) != 1 || !strings.Contains(stores[0], `-FLAGS.SILENT (\Seen)`) {
		t.Errorf("stores = %v, want one silent removal of \\Seen", stores)
	}
}

// MoveMany and DeleteMany share the connect, select and chunking helpers with SetSeenMany; these pin
// that each still costs one login for the folder and sends the command its chunks call for.
func TestMoveManyUsesOneConnectionAndChunks(t *testing.T) {
	log := &commandLog{}
	host, port := listenFake(t, script{commands: log, extraCaps: "MOVE"})

	if _, err := fakeSource().MoveMany(context.Background(), fakeAccount(t, host, port), fakeFolder(t), uidRange(1, bulkBatchSize+1), "Archive"); err != nil {
		t.Fatalf("MoveMany: %v", err)
	}
	if logins := log.matching("LOGIN"); len(logins) != 1 {
		t.Errorf("logins = %d, want 1", len(logins))
	}
	moves := log.matching("UID MOVE")
	if len(moves) != 2 || !strings.Contains(moves[0], "Archive") {
		t.Errorf("moves = %v, want 2 chunks to Archive", moves)
	}
}

func TestDeleteManyToTrashMoves(t *testing.T) {
	log := &commandLog{}
	host, port := listenFake(t, script{commands: log, extraCaps: "MOVE"})

	if _, err := fakeSource().DeleteMany(context.Background(), fakeAccount(t, host, port), fakeFolder(t), []string{"7", "8"}, "Trash"); err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}
	if moves := log.matching("UID MOVE"); len(moves) != 1 || !strings.Contains(moves[0], "Trash") {
		t.Errorf("moves = %v, want one move to Trash", moves)
	}
	if stores := log.matching("UID STORE"); len(stores) != 0 {
		t.Errorf("a delete to Trash flagged messages \\Deleted: %v", stores)
	}
}

func TestDeleteManyPermanentFlagsAndExpungesEachChunk(t *testing.T) {
	log := &commandLog{}
	host, port := listenFake(t, script{commands: log})

	if _, err := fakeSource().DeleteMany(context.Background(), fakeAccount(t, host, port), fakeFolder(t), uidRange(1, bulkBatchSize+1), ""); err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}
	stores := log.matching("UID STORE")
	if len(stores) != 2 || !strings.Contains(stores[0], `+FLAGS.SILENT (\Deleted)`) {
		t.Errorf("stores = %v, want 2 chunks flagged \\Deleted", stores)
	}
	if expunges := log.matching("EXPUNGE"); len(expunges) != 2 {
		t.Errorf("expunges = %d, want one per chunk", len(expunges))
	}
	if logins := log.matching("LOGIN"); len(logins) != 1 {
		t.Errorf("logins = %d, want 1", len(logins))
	}
}

func TestBulkCommandsRefuseAMalformedUIDBeforeConnecting(t *testing.T) {
	log := &commandLog{}
	host, port := listenFake(t, script{commands: log})
	account, folder := fakeAccount(t, host, port), fakeFolder(t)

	if _, err := fakeSource().MoveMany(context.Background(), account, folder, []string{"x"}, "Archive"); err == nil {
		t.Error("MoveMany accepted a malformed uid")
	}
	if _, err := fakeSource().DeleteMany(context.Background(), account, folder, []string{"x"}, ""); err == nil {
		t.Error("DeleteMany accepted a malformed uid")
	}
	if err := fakeSource().SetSeenMany(context.Background(), account, folder, []string{"x"}, true); err == nil {
		t.Error("SetSeenMany accepted a malformed uid")
	}
	if logins := log.matching("LOGIN"); len(logins) != 0 {
		t.Errorf("logged in %d times for malformed uids", len(logins))
	}
}

func TestSetSeenManyWithNoUIDsDoesNotConnect(t *testing.T) {
	log := &commandLog{}
	host, port := listenFake(t, script{commands: log})

	if err := fakeSource().SetSeenMany(context.Background(), fakeAccount(t, host, port), fakeFolder(t), nil, true); err != nil {
		t.Fatalf("SetSeenMany: %v", err)
	}
	if logins := log.matching("LOGIN"); len(logins) != 0 {
		t.Errorf("logged in %d times for an empty selection", len(logins))
	}
}
