package storage

import (
	"context"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func draftRecoveryFixture(t *testing.T, subject string) domain.DraftRecovery {
	t.Helper()
	rec, err := domain.NewDraftRecovery(domain.DraftRecoveryInput{
		AccountID: "acc-1",
		To:        "friend@example.com, half@examp",
		Cc:        "cc@example.com",
		Bcc:       "bcc@example.com",
		Subject:   subject,
		BodyHTML:  "<p>still writing</p>",
	}, time.UnixMilli(1000).UTC())
	if err != nil {
		t.Fatalf("build recovery: %v", err)
	}
	return rec
}

func TestDraftRecoveryRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if _, ok, err := store.GetDraftRecovery(ctx); err != nil || ok {
		t.Fatalf("empty store: ok=%v err=%v, want ok=false err=nil", ok, err)
	}

	if err := store.SaveDraftRecovery(ctx, draftRecoveryFixture(t, "First")); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := store.GetDraftRecovery(ctx)
	if err != nil || !ok {
		t.Fatalf("get after save: ok=%v err=%v", ok, err)
	}
	if got.AccountID() != "acc-1" || got.To() != "friend@example.com, half@examp" ||
		got.Cc() != "cc@example.com" || got.Bcc() != "bcc@example.com" ||
		got.Subject() != "First" || got.BodyHTML() != "<p>still writing</p>" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if !got.SavedAt().Equal(time.UnixMilli(1000).UTC()) {
		t.Errorf("savedAt = %v", got.SavedAt())
	}
}

func TestDraftRecoveryReplacesPrevious(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveDraftRecovery(ctx, draftRecoveryFixture(t, "First")); err != nil {
		t.Fatalf("save first: %v", err)
	}
	if err := store.SaveDraftRecovery(ctx, draftRecoveryFixture(t, "Second")); err != nil {
		t.Fatalf("save second: %v", err)
	}
	got, ok, err := store.GetDraftRecovery(ctx)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Subject() != "Second" {
		t.Errorf("subject = %q, want the replacing snapshot 'Second'", got.Subject())
	}
}

func TestDraftRecoveryClear(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveDraftRecovery(ctx, draftRecoveryFixture(t, "First")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.ClearDraftRecovery(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok, err := store.GetDraftRecovery(ctx); err != nil || ok {
		t.Errorf("after clear: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
	// Clearing an already-empty slot is a no-op, not an error.
	if err := store.ClearDraftRecovery(ctx); err != nil {
		t.Errorf("clear on empty: %v", err)
	}
}
