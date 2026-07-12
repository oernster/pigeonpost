package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newTagSync() (*TagSyncService, *fakeTagStore, *fakeMailStore, *fakeAccountStore, *fakeMailActions) {
	tags := newFakeTagStore()
	store := newFakeMailStore()
	accounts := newFakeAccountStore()
	remote := &fakeMailActions{}
	return NewTagSyncService(tags, store, accounts, remote), tags, store, accounts, remote
}

func makeTag(t *testing.T, id, name string) domain.Tag {
	t.Helper()
	colour, err := domain.NewColour("#3366ff")
	if err != nil {
		t.Fatalf("colour: %v", err)
	}
	tg, err := domain.NewTag(id, name, colour, domain.KeywordForName(name))
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	return tg
}

func keywordMessage(t *testing.T, id string, keywords ...string) domain.MessageSummary {
	t.Helper()
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: "f1", UID: "1", Size: 1, Keywords: keywords,
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

func hasTag(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestTagSyncAssign(t *testing.T) {
	svc, tags, store, accounts, _ := newTagSync()
	seedMessageLocation(t, store, accounts) // m1 in f1, account a1 (IMAP)

	if err := svc.Assign(context.Background(), "m1", "t1"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if !hasTag(tags.byMessage["m1"], "t1") {
		t.Errorf("tag not assigned locally: %v", tags.byMessage["m1"])
	}
	if got, ok := tags.pending["m1"]["t1"]; !ok || got != true {
		t.Errorf("expected a pending assign recorded, got %v (present=%v)", got, ok)
	}
}

func TestTagSyncAssignAddError(t *testing.T) {
	svc, tags, store, accounts, _ := newTagSync()
	seedMessageLocation(t, store, accounts)
	tags.addErr = errBoom
	if err := svc.Assign(context.Background(), "m1", "t1"); !errors.Is(err, errBoom) {
		t.Errorf("assign error = %v, want wrapped boom", err)
	}
}

func TestTagSyncAssignResolveError(t *testing.T) {
	svc, _, store, _, _ := newTagSync()
	store.getMessageErr = errBoom // the local add succeeds, then recordIntent's resolve fails
	if err := svc.Assign(context.Background(), "m1", "t1"); !errors.Is(err, errBoom) {
		t.Errorf("assign resolve error = %v, want wrapped boom", err)
	}
}

func TestTagSyncAssignPendingError(t *testing.T) {
	svc, tags, store, accounts, _ := newTagSync()
	seedMessageLocation(t, store, accounts)
	tags.setPendingErr = errBoom
	if err := svc.Assign(context.Background(), "m1", "t1"); !errors.Is(err, errBoom) {
		t.Errorf("assign pending error = %v, want wrapped boom", err)
	}
}

func TestTagSyncAssignPop3IsLocalOnly(t *testing.T) {
	svc, tags, store, accounts, _ := newTagSync()
	seedMessageLocation(t, store, accounts)
	accounts.accounts["a1"] = pop3Account(t, "a1") // a POP3 account has no server keywords

	if err := svc.Assign(context.Background(), "m1", "t1"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if !hasTag(tags.byMessage["m1"], "t1") {
		t.Error("tag should still be assigned locally on POP3")
	}
	if len(tags.pending["m1"]) != 0 {
		t.Errorf("a POP3 assign must record no pending intent, got %v", tags.pending["m1"])
	}
}

func TestTagSyncUnassign(t *testing.T) {
	svc, tags, store, accounts, _ := newTagSync()
	seedMessageLocation(t, store, accounts)
	tags.byMessage["m1"] = []string{"t1"}

	if err := svc.Unassign(context.Background(), "m1", "t1"); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	if hasTag(tags.byMessage["m1"], "t1") {
		t.Errorf("tag not unassigned locally: %v", tags.byMessage["m1"])
	}
	if got, ok := tags.pending["m1"]["t1"]; !ok || got != false {
		t.Errorf("expected a pending unassign recorded, got %v (present=%v)", got, ok)
	}
}

func TestTagSyncUnassignRemoveError(t *testing.T) {
	svc, tags, store, accounts, _ := newTagSync()
	seedMessageLocation(t, store, accounts)
	tags.removeErr = errBoom
	if err := svc.Unassign(context.Background(), "m1", "t1"); !errors.Is(err, errBoom) {
		t.Errorf("unassign error = %v, want wrapped boom", err)
	}
}

func TestTagSyncUnassignResolveError(t *testing.T) {
	svc, _, store, _, _ := newTagSync()
	store.getMessageErr = errBoom
	if err := svc.Unassign(context.Background(), "m1", "t1"); !errors.Is(err, errBoom) {
		t.Errorf("unassign resolve error = %v, want wrapped boom", err)
	}
}

func TestReconcileFetchedAlignsLocalWithServer(t *testing.T) {
	svc, tags, _, _, _ := newTagSync()
	work := makeTag(t, "t1", "Work")
	personal := makeTag(t, "t2", "Personal")
	urgent := makeTag(t, "t3", "Urgent")
	tags.tags["t1"] = work
	tags.tags["t2"] = personal
	tags.tags["t3"] = urgent

	// m1: the server has Work, the local cache does not, no pending -> add it locally.
	// m2: the local cache has Personal, the server does not, no pending -> remove it locally.
	// m3: the server and the local cache agree on Urgent, no pending -> leave it untouched.
	tags.byMessage["m2"] = []string{"t2"}
	tags.byMessage["m3"] = []string{"t3"}

	msgs := []domain.MessageSummary{
		keywordMessage(t, "m1", work.Keyword()),
		keywordMessage(t, "m2"),
		keywordMessage(t, "m3", urgent.Keyword()),
	}
	if err := svc.ReconcileFetched(context.Background(), msgs); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !hasTag(tags.byMessage["m1"], "t1") {
		t.Errorf("expected Work added to m1, got %v", tags.byMessage["m1"])
	}
	if hasTag(tags.byMessage["m2"], "t2") {
		t.Errorf("expected Personal removed from m2, got %v", tags.byMessage["m2"])
	}
	if !hasTag(tags.byMessage["m3"], "t3") {
		t.Errorf("expected Urgent kept on m3, got %v", tags.byMessage["m3"])
	}
}

func TestReconcileFetchedPendingGuardsAndConfirms(t *testing.T) {
	svc, tags, _, _, _ := newTagSync()
	work := makeTag(t, "t1", "Work")
	personal := makeTag(t, "t2", "Personal")
	tags.tags["t1"] = work
	tags.tags["t2"] = personal

	// m1: pending assign of Work, the server now has it -> confirmed, pending cleared, local kept.
	tags.byMessage["m1"] = []string{"t1"}
	tags.pending["m1"] = map[string]bool{"t1": true}
	// m2: pending assign of Work, the server does not have it yet -> guarded, pending kept.
	tags.byMessage["m2"] = []string{"t1"}
	tags.pending["m2"] = map[string]bool{"t1": true}
	// m3: pending unassign of Personal, the server does not have it -> confirmed removal, pending cleared.
	tags.pending["m3"] = map[string]bool{"t2": false}
	// m4: pending unassign of Personal, the server still has it -> guarded, pending kept.
	tags.pending["m4"] = map[string]bool{"t2": false}

	msgs := []domain.MessageSummary{
		keywordMessage(t, "m1", work.Keyword()),
		keywordMessage(t, "m2"),
		keywordMessage(t, "m3"),
		keywordMessage(t, "m4", personal.Keyword()),
	}
	if err := svc.ReconcileFetched(context.Background(), msgs); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(tags.pending["m1"]) != 0 {
		t.Errorf("m1 pending should be cleared once the server agrees, got %v", tags.pending["m1"])
	}
	if !tags.pending["m2"]["t1"] {
		t.Error("m2 pending assign should be kept while the server still disagrees")
	}
	if _, ok := tags.pending["m3"]["t2"]; ok {
		t.Errorf("m3 pending unassign should be cleared once the server agrees, got %v", tags.pending["m3"])
	}
	if _, ok := tags.pending["m4"]["t2"]; !ok {
		t.Error("m4 pending unassign should be kept while the server still disagrees")
	}
}

func TestReconcileFetchedEmptyIsNoOp(t *testing.T) {
	svc, tags, _, _, _ := newTagSync()
	tags.listErr = errBoom // must not even be consulted for an empty batch
	if err := svc.ReconcileFetched(context.Background(), nil); err != nil {
		t.Errorf("empty reconcile = %v, want nil", err)
	}
}

func TestReconcileFetchedErrors(t *testing.T) {
	work := makeTag(t, "t1", "Work")

	t.Run("list tags", func(t *testing.T) {
		svc, tags, _, _, _ := newTagSync()
		tags.listErr = errBoom
		if err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{keywordMessage(t, "m1")}); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("tags for message", func(t *testing.T) {
		svc, tags, _, _, _ := newTagSync()
		tags.forMsgErr = errBoom
		if err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{keywordMessage(t, "m1")}); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("pending ops", func(t *testing.T) {
		svc, tags, _, _, _ := newTagSync()
		tags.pendingErr = errBoom
		if err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{keywordMessage(t, "m1")}); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("add message tag", func(t *testing.T) {
		svc, tags, _, _, _ := newTagSync()
		tags.tags["t1"] = work
		tags.addErr = errBoom
		if err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{keywordMessage(t, "m1", work.Keyword())}); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("remove message tag", func(t *testing.T) {
		svc, tags, _, _, _ := newTagSync()
		tags.tags["t1"] = work
		tags.byMessage["m1"] = []string{"t1"}
		tags.removeErr = errBoom
		if err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{keywordMessage(t, "m1")}); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("clear pending", func(t *testing.T) {
		svc, tags, _, _, _ := newTagSync()
		tags.tags["t1"] = work
		tags.byMessage["m1"] = []string{"t1"}
		tags.pending["m1"] = map[string]bool{"t1": true}
		tags.clearPendingErr = errBoom
		if err := svc.ReconcileFetched(context.Background(), []domain.MessageSummary{keywordMessage(t, "m1", work.Keyword())}); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestFlushPendingPushesToServer(t *testing.T) {
	svc, tags, store, accounts, remote := newTagSync()
	seedMessageLocation(t, store, accounts) // m1 in f1, account a1 (IMAP)
	work := makeTag(t, "t1", "Work")
	tags.tags["t1"] = work
	tags.pending["m1"] = map[string]bool{"t1": true}

	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(remote.keywordCalls) != 1 || remote.keywordCalls[0].keyword != work.Keyword() || !remote.keywordCalls[0].set {
		t.Errorf("expected one SetKeyword add of %q, got %+v", work.Keyword(), remote.keywordCalls)
	}
}

func TestFlushPendingNoPendingPushesNothing(t *testing.T) {
	svc, _, _, _, remote := newTagSync()
	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(remote.keywordCalls) != 0 {
		t.Errorf("no pending should push nothing, got %+v", remote.keywordCalls)
	}
}

func TestFlushPendingListError(t *testing.T) {
	svc, tags, _, _, _ := newTagSync()
	tags.listPendingErr = errBoom
	if err := svc.FlushPending(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("flush list error = %v, want wrapped boom", err)
	}
}

func TestFlushPendingListTagsError(t *testing.T) {
	svc, tags, _, _, _ := newTagSync()
	tags.pending["m1"] = map[string]bool{"t1": true}
	tags.listErr = errBoom
	if err := svc.FlushPending(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("flush list tags error = %v, want wrapped boom", err)
	}
}

func TestFlushPendingSkipsUnknownTagAndMissingMessage(t *testing.T) {
	svc, tags, store, accounts, remote := newTagSync()
	seedMessageLocation(t, store, accounts)
	tags.tags["t1"] = makeTag(t, "t1", "Work")
	// A pending op for a tag that no longer exists has no keyword mapping and is skipped.
	tags.pending["m1"] = map[string]bool{"ghost": true}
	// A pending op for a message no longer cached fails to resolve and is skipped.
	tags.pending["mX"] = map[string]bool{"t1": true}

	if err := svc.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(remote.keywordCalls) != 0 {
		t.Errorf("an unknown tag and a missing message must be skipped, got %+v", remote.keywordCalls)
	}
}

func TestFlushPendingSwallowsPushError(t *testing.T) {
	svc, tags, store, accounts, remote := newTagSync()
	seedMessageLocation(t, store, accounts)
	tags.tags["t1"] = makeTag(t, "t1", "Work")
	tags.pending["m1"] = map[string]bool{"t1": true}
	remote.keywordErr = errBoom
	// A push failure is swallowed so an offline or rejecting server never fails the sync; the pending op
	// is left in place to be retried.
	if err := svc.FlushPending(context.Background()); err != nil {
		t.Errorf("flush should swallow a push error, got %v", err)
	}
}
