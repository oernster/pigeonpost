package storage

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// saveUnread caches the given ids as unread messages in folder f1.
func saveUnread(t *testing.T, store *Store, ids ...string) {
	t.Helper()
	msgs := make([]domain.MessageSummary, 0, len(ids))
	for _, id := range ids {
		msgs = append(msgs, buildMessageIn(t, id, "f1", false))
	}
	if err := store.SaveMessages(context.Background(), "f1", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}
}

func TestSetFlagManyMarksEveryMessageWithItsIntent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveUnread(t, store, "m1", "m2", "m3")

	if err := store.SetFlagMany(ctx, []string{"m1", "m3"}, domain.FlagSeen, true, true); err != nil {
		t.Fatalf("set seen: %v", err)
	}
	msgs, err := store.ListMessages(ctx, "f1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	read := map[string]bool{}
	for _, m := range msgs {
		read[m.ID()] = m.IsRead()
	}
	if !read["m1"] || read["m2"] || !read["m3"] {
		t.Errorf("read = %v, want m1 and m3 read, m2 unread", read)
	}
	for _, id := range []string{"m1", "m3"} {
		ops, err := store.PendingFlagOps(ctx, id)
		if err != nil {
			t.Fatalf("pending %s: %v", id, err)
		}
		if ops[domain.FlagSeen] != true {
			t.Errorf("pending for %s = %v, want Seen true", id, ops)
		}
	}
}

func TestSetFlagManyIsAllOrNothing(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveUnread(t, store, "m1")

	// One missing id fails the transaction, so m1 is not left half-applied.
	if err := store.SetFlagMany(ctx, []string{"m1", "missing"}, domain.FlagSeen, true, true); err == nil {
		t.Fatal("expected an error for a missing message")
	}
	msgs, _ := store.ListMessages(ctx, "f1")
	if msgs[0].IsRead() {
		t.Error("m1 was marked read although the batch failed")
	}
	ops, _ := store.PendingFlagOps(ctx, "m1")
	if len(ops) != 0 {
		t.Errorf("pending intent left behind by a failed batch: %v", ops)
	}
}
