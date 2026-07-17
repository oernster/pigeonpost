package application

import (
	"context"
	"errors"
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
	if len(remote.seenCalls) != 1 || remote.seenCalls[0] != true {
		t.Errorf("SetSeen calls = %v, want [true]", remote.seenCalls)
	}
	if len(remote.flaggedCalls) != 1 || remote.flaggedCalls[0] != true {
		t.Errorf("SetFlagged calls = %v, want [true]", remote.flaggedCalls)
	}
	if len(remote.answeredCalls) != 1 || remote.answeredCalls[0] != false {
		t.Errorf("SetAnswered calls = %v, want [false]", remote.answeredCalls)
	}
	if len(remote.forwardedCalls) != 1 || remote.forwardedCalls[0] != true {
		t.Errorf("SetForwarded calls = %v, want [true]", remote.forwardedCalls)
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
	if len(remote.seenCalls) != 0 {
		t.Errorf("expected no pushes for an unresolvable message, got %v", remote.seenCalls)
	}
	if store.pendingFlags["gone"][domain.FlagSeen] != true {
		t.Error("intent for the unresolvable message was dropped")
	}
}

func TestFlagSyncFlushServerErrorKeepsIntent(t *testing.T) {
	svc, store, accounts, remote := newFlagSyncService()
	seedMessageLocation(t, store, accounts)
	store.pendingFlags = map[string]map[domain.Flag]bool{"m1": {domain.FlagSeen: true}}
	remote.setSeenErr = errBoom

	// A failed push is best-effort: no error and the intent stays to be retried.
	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
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
