package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// seedTwoFolders wires m1 and m2 into INBOX (f1) and m3 into Archive (f2), all in account a1.
func seedTwoFolders(t *testing.T, store *fakeMailStore, accounts *fakeAccountStore) {
	t.Helper()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), testFolder(t, "f2", "a1", "Archive")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1"), testMessage(t, "m2", "f1")}
	store.messages["f2"] = []domain.MessageSummary{testMessage(t, "m3", "f2")}
}

func TestMarkReadManyWritesCacheWithIntentAndLeavesTheServerAlone(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedTwoFolders(t, store, accounts)

	written, err := svc.MarkReadMany(context.Background(), []string{"m1", "m2", "m3"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(written, []string{"m1", "m2", "m3"}) {
		t.Errorf("written = %v, want every id", written)
	}
	for _, id := range []string{"m1", "m2", "m3"} {
		if store.pendingFlags[id][domain.FlagSeen] != true {
			t.Errorf("pending intent for %s = %+v, want Seen true", id, store.pendingFlags[id])
		}
	}
	if !store.messages["f1"][0].IsRead() || !store.messages["f2"][0].IsRead() {
		t.Error("cache was not marked read")
	}
	// The cache write is the whole of it: the badges refresh from the cache before any server round trip.
	if len(remote.seenCalls) != 0 || len(remote.seenManyBatches) != 0 {
		t.Errorf("server called by MarkReadMany: single %v, batches %v", remote.seenCalls, remote.seenManyBatches)
	}
}

func TestMarkReadManyPOP3RecordsNoIntent(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.accounts["a1"] = pop3Account(t, "a1")

	written, err := svc.MarkReadMany(context.Background(), []string{"m1"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(written) != 1 || !store.messages["f1"][0].IsRead() {
		t.Errorf("POP3 message not marked read locally: written %v", written)
	}
	if len(store.pendingFlags) != 0 {
		t.Errorf("pending intents recorded for POP3: %+v", store.pendingFlags)
	}
}

func TestMarkReadManySkipsAnUnresolvableMessageAndWritesTheRest(t *testing.T) {
	svc, store, accounts, _ := newActionService()
	seedMessageLocation(t, store, accounts)

	written, err := svc.MarkReadMany(context.Background(), []string{"missing", "m1"}, true)
	if err == nil {
		t.Error("expected an error naming the missing message")
	}
	if !reflect.DeepEqual(written, []string{"m1"}) {
		t.Errorf("written = %v, want [m1]", written)
	}
}

func TestMarkReadManyReportsLookupErrors(t *testing.T) {
	cases := map[string]func(*fakeMailStore, *fakeAccountStore){
		"message": func(s *fakeMailStore, _ *fakeAccountStore) { s.getMessageErr = errBoom },
		"folder":  func(s *fakeMailStore, _ *fakeAccountStore) { s.getFolderErr = errBoom },
		"account": func(_ *fakeMailStore, a *fakeAccountStore) { a.getErr = errBoom },
		"cache":   func(s *fakeMailStore, _ *fakeAccountStore) { s.setFlagErr = errBoom },
	}
	for name, plant := range cases {
		t.Run(name, func(t *testing.T) {
			svc, store, accounts, _ := newActionService()
			seedMessageLocation(t, store, accounts)
			plant(store, accounts)
			written, err := svc.MarkReadMany(context.Background(), []string{"m1"}, true)
			if !errors.Is(err, errBoom) {
				t.Errorf("error = %v, want wrapped boom", err)
			}
			if len(written) != 0 {
				t.Errorf("written = %v, want none", written)
			}
		})
	}
}

func TestPushReadManySendsOneBatchPerFolder(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedTwoFolders(t, store, accounts)

	if err := svc.PushReadMany(context.Background(), []string{"m1", "m2", "m3"}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// One server call per folder carrying that folder's UIDs, not one per message.
	if len(remote.seenManyBatches) != 2 || len(remote.seenManyBatches[0]) != 2 || len(remote.seenManyBatches[1]) != 1 {
		t.Errorf("batches = %v, want [[2 uids] [1 uid]]", remote.seenManyBatches)
	}
	if !reflect.DeepEqual(remote.seenManyValues, []bool{true, true}) {
		t.Errorf("values = %v, want [true true]", remote.seenManyValues)
	}
	if len(remote.seenCalls) != 0 {
		t.Errorf("per-message SetSeen used: %v", remote.seenCalls)
	}
}

func TestPushReadManySkipsPOP3(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	accounts.accounts["a1"] = pop3Account(t, "a1")

	if err := svc.PushReadMany(context.Background(), []string{"m1"}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.seenManyBatches) != 0 {
		t.Errorf("server called for POP3: %v", remote.seenManyBatches)
	}
}

func TestPushReadManyReportsAServerFailure(t *testing.T) {
	svc, store, accounts, remote := newActionService()
	seedMessageLocation(t, store, accounts)
	remote.seenManyErr = errBoom

	if err := svc.PushReadMany(context.Background(), []string{"m1"}, true); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}
