package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newFlagSyncService() (*FlagSyncService, *fakeMailStore, *fakeAccountStore, *fakeMailActions) {
	store := newFakeMailStore()
	accounts := newFakeAccountStore()
	remote := &fakeMailActions{}
	return NewFlagSyncService(store, accounts, remote), store, accounts, remote
}

func TestFlagSyncFlushPushesEachFlagKind(t *testing.T) {
	svc, store, accounts, remote := newFlagSyncService()
	seedMessageLocation(t, store, accounts)
	store.pendingFlags = map[string]map[domain.Flag]bool{
		"m1": {
			domain.FlagSeen:      true,
			domain.FlagFlagged:   true,
			domain.FlagAnswered:  false,
			domain.FlagForwarded: true,
		},
	}

	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	// Each flag kind is its own batch, carrying the value the intent asked for.
	got := map[domain.Flag]bool{}
	for _, call := range remote.pushFlagBatches {
		got[call.flag] = call.set
	}
	want := map[domain.Flag]bool{
		domain.FlagSeen: true, domain.FlagFlagged: true, domain.FlagAnswered: false, domain.FlagForwarded: true,
	}
	if len(remote.pushFlagBatches) != len(want) || !reflect.DeepEqual(got, want) {
		t.Errorf("pushed %v, want one batch per flag kind %v", remote.pushFlagBatches, want)
	}
	if len(store.pendingFlags) != 0 {
		t.Errorf("settled intents were kept: %v", store.pendingFlags)
	}
}

// messageWithUID builds a cached message in folderID with its own UID.
func messageWithUID(t *testing.T, id, folderID, uid string) domain.MessageSummary {
	t.Helper()
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{ID: id, FolderID: folderID, UID: uid, Size: 1, Flags: domain.NewFlags(0)})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

// The storm StartMail blocked the address for: thousands of intents in one folder were pushed on one
// connection each, on every pass, never to be cleared. They must go as one batch and be cleared once settled,
// so the next pass pushes nothing at all.
func TestFlagSyncFlushBatchesAFolderAndClearsWhatSettles(t *testing.T) {
	svc, store, accounts, remote := newFlagSyncService()
	seedMessageLocation(t, store, accounts)
	const intents = 50
	store.messages["f1"] = nil
	store.pendingFlags = map[string]map[domain.Flag]bool{}
	for i := range intents {
		id, uid := fmt.Sprintf("m%d", i), strconv.Itoa(i+1)
		store.messages["f1"] = append(store.messages["f1"], messageWithUID(t, id, "f1", uid))
		store.pendingFlags[id] = map[domain.Flag]bool{domain.FlagSeen: true}
	}

	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(remote.pushFlagBatches) != 1 || len(remote.pushFlagBatches[0].uids) != intents {
		t.Fatalf("pushes = %d, want one carrying all %d intents", len(remote.pushFlagBatches), intents)
	}
	if len(store.pendingFlags) != 0 {
		t.Fatalf("%d intents left after the server settled them all", len(store.pendingFlags))
	}
	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("second flush: %v", err)
	}
	if len(remote.pushFlagBatches) != 1 {
		t.Errorf("the second pass pushed again: %d pushes, want still 1", len(remote.pushFlagBatches))
	}
}

// An intent the server keeps reporting otherwise stays, so a reconcile can still guard the local value.
func TestFlagSyncFlushKeepsWhatTheServerDidNotSettle(t *testing.T) {
	svc, store, accounts, remote := newFlagSyncService()
	seedMessageLocation(t, store, accounts)
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	remote.pushFlagUnsettled = map[string]bool{"1": true}

	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.pendingFlags["m1"][domain.FlagSeen] != true {
		t.Error("an intent the server did not settle was dropped")
	}
}

// A settled intent the store cannot clear is reported rather than passing quietly.
func TestFlagSyncFlushReportsAClearThatFailed(t *testing.T) {
	svc, store, accounts, _ := newFlagSyncService()
	seedMessageLocation(t, store, accounts)
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	store.clearPendingFlagErr = errBoom

	if err := svc.FlushPending(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("flush error = %v, want the failed clear reported", err)
	}
}

func TestFlagSyncFlushSkipsUnresolvableAndKeepsIntent(t *testing.T) {
	svc, store, _, remote := newFlagSyncService()
	// The message cannot be resolved (nothing seeded), so the push is skipped without error and the
	// intent stays for the store's orphan sweep or a later resolvable pass.
	store.pendingFlags = map[string]map[domain.Flag]bool{"gone": {domain.FlagSeen: true}}

	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(remote.pushFlagBatches) != 0 {
		t.Errorf("expected no pushes for an unresolvable message, got %v", remote.pushFlagBatches)
	}
	if store.pendingFlags["gone"][domain.FlagSeen] != true {
		t.Error("intent for the unresolvable message was dropped")
	}
}

func TestFlagSyncFlushServerErrorKeepsIntent(t *testing.T) {
	svc, store, accounts, remote := newFlagSyncService()
	seedMessageLocation(t, store, accounts)
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	remote.pushFlagErr = errBoom

	// A failed push leaves the intent to be retried and is reported for the caller to record.
	if err := svc.FlushPending(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("flush error = %v, want the failed push reported", err)
	}
	if store.pendingFlags["m1"][domain.FlagSeen] != true {
		t.Error("intent was dropped on a failed push")
	}
}

func TestFlagSyncFlushListError(t *testing.T) {
	svc, store, _, _ := newFlagSyncService()
	store.listPendingFlagErr = errBoom
	if err := svc.FlushPending(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("flush error = %v, want wrapped boom", err)
	}
}

func TestFlagSyncReconcileConfirmsAgreement(t *testing.T) {
	svc, store, _, _ := newFlagSyncService()
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	fetched := testMessage(t, "m1", "f1").WithFlags(domain.NewFlags(domain.FlagSeen))

	out, err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{fetched})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !out[0].IsRead() {
		t.Error("an agreeing fetch must pass through unchanged")
	}
	if len(store.pendingFlags) != 0 {
		t.Errorf("expected the confirmed intent cleared, got %+v", store.pendingFlags)
	}
}

func TestFlagSyncReconcileOverlaysDisagreement(t *testing.T) {
	svc, store, _, _ := newFlagSyncService()
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	// The server still reports the message unseen (a lazy or dropped STORE); the overlay must guard
	// the local read state and keep the intent for the next flush.
	fetched := testMessage(t, "m1", "f1")

	out, err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{fetched})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !out[0].IsRead() {
		t.Error("a disagreeing fetch must be overlaid with the pending intent")
	}
	if store.pendingFlags["m1"][domain.FlagSeen] != true {
		t.Error("the unconfirmed intent must stay recorded")
	}
}

func TestFlagSyncReconcileLeavesOtherMessagesAlone(t *testing.T) {
	svc, store, _, _ := newFlagSyncService()
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	fetched := []domain.MessageSummary{testMessage(t, "m1", "f1"), testMessage(t, "m2", "f1")}

	out, err := svc.ReconcileFetched(context.Background(), fetched)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !out[0].IsRead() || out[1].IsRead() {
		t.Errorf("overlay leaked: m1 read = %t, m2 read = %t", out[0].IsRead(), out[1].IsRead())
	}
}

func TestFlagSyncReconcileOverlaysPendingClear(t *testing.T) {
	svc, store, _, _ := newFlagSyncService()
	// A pending mark-unread must also survive a stale fetch that still reports the message seen.
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: false}}
	fetched := testMessage(t, "m1", "f1").WithFlags(domain.NewFlags(domain.FlagSeen))

	out, err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{fetched})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if out[0].IsRead() {
		t.Error("a pending unread intent must overlay a stale seen fetch")
	}
	if store.pendingFlags["m1"][domain.FlagSeen] != false {
		t.Error("the unconfirmed intent must stay recorded")
	}
}

func TestFlagSyncReconcileEmptyFetchIsPassThrough(t *testing.T) {
	svc, _, _, _ := newFlagSyncService()
	out, err := svc.ReconcileFetched(context.Background(), nil)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected an empty result for an empty fetch, got %+v", out)
	}
}

func TestFlagSyncReconcileNoPendingIsPassThrough(t *testing.T) {
	svc, _, _, _ := newFlagSyncService()
	fetched := []domain.MessageSummary{testMessage(t, "m1", "f1")}
	out, err := svc.ReconcileFetched(context.Background(), fetched)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(out) != 1 || out[0].ID() != "m1" || out[0].IsRead() {
		t.Errorf("pass-through changed the messages: %+v", out)
	}
}

func TestFlagSyncReconcileListError(t *testing.T) {
	svc, store, _, _ := newFlagSyncService()
	store.listPendingFlagErr = errBoom
	if _, err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{testMessage(t, "m1", "f1")}); !errors.Is(err, errBoom) {
		t.Errorf("reconcile error = %v, want wrapped boom", err)
	}
}

func TestFlagSyncReconcileClearError(t *testing.T) {
	svc, store, _, _ := newFlagSyncService()
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	store.clearPendingFlagErr = errBoom
	fetched := testMessage(t, "m1", "f1").WithFlags(domain.NewFlags(domain.FlagSeen))
	if _, err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{fetched}); !errors.Is(err, errBoom) {
		t.Errorf("reconcile error = %v, want wrapped boom", err)
	}
}
