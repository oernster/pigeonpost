package imap

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// A permanent delete refused on its second chunk has already destroyed the first: the adapter returns
// those UIDs together with the error, so the application drops their rows instead of showing them at
// UIDs that no longer exist. The refused chunk is untouched on the server.
func TestDeleteManyPermanentKeepsTheChunksDestroyedBeforeARefusal(t *testing.T) {
	srv := &mailboxServer{caps: "UIDPLUS", storeRefusal: "[CANNOT] read-only", refuseStoreFrom: 2}
	seeded := make([]uint32, 0, bulkBatchSize+1)
	for u := uint32(1); u <= bulkBatchSize+1; u++ {
		seeded = append(seeded, u)
	}
	host, port := newMailboxServer(t, srv, inboxPath, seeded...)
	uids := uidRange(1, bulkBatchSize+1)

	destroyed, err := fakeSource().DeleteMany(context.Background(), fakeAccount(t, host, port), inboxFolder(t), uids, "")

	if err == nil || !strings.Contains(err.Error(), "CANNOT") {
		t.Fatalf("err = %v, want the second chunk's refusal", err)
	}
	if len(destroyed) != bulkBatchSize {
		t.Fatalf("destroyed = %d entries, want the first chunk's %d", len(destroyed), bulkBatchSize)
	}
	for _, uid := range uids[:bulkBatchSize] {
		if _, ok := destroyed[uid]; !ok {
			t.Fatalf("destroyed lacks uid %s from the expunged first chunk", uid)
		}
	}
	if got := srv.uids(inboxPath); !reflect.DeepEqual(got, []uint32{bulkBatchSize + 1}) {
		t.Errorf("server holds %v, want only the refused chunk", got)
	}
}
