package storage

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// rehomedSummary is a message as it stands after a move into fd under UID 9, with a subject word no other
// fixture row carries so search can find exactly it.
func rehomedSummary(t *testing.T) domain.MessageSummary {
	t.Helper()
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: domain.MessageIDFor("fd", "9"), FolderID: "fd", UID: "9", MessageID: "<moved@x>",
		Subject: "Quarterlyzebra report", Date: time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC),
		Size: 2048, Flags: domain.NewFlags(domain.FlagSeen), Snippet: "moved snippet",
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

func TestRehomeMessagesFilesTheMovedRowBesideTheDestinationsOwn(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "fd", "a1", "Archive")
	if err := store.SaveMessages(ctx, "fd", []domain.MessageSummary{buildMessageIn(t, "k1", "fd", true)}); err != nil {
		t.Fatalf("seed destination: %v", err)
	}
	moved := rehomedSummary(t)

	// Filing it twice must leave one row: a sync may already have written it; a retry may repeat it.
	for range 2 {
		if err := store.RehomeMessages(ctx, []domain.MessageSummary{moved}); err != nil {
			t.Fatalf("RehomeMessages: %v", err)
		}
	}

	ids, err := store.MessageIDs(ctx, "fd")
	if err != nil {
		t.Fatalf("MessageIDs: %v", err)
	}
	sort.Strings(ids)
	if want := []string{moved.ID(), "k1"}; len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("fd holds %q, want %q: the destination's own row kept and the moved one added once", ids, want)
	}
	got, err := store.GetMessage(ctx, moved.ID())
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if got.Subject() != moved.Subject() || got.UID() != "9" || !got.IsRead() || got.Size() != moved.Size() {
		t.Errorf("filed row = subject %q uid %q read %v size %d, want the moved message's data", got.Subject(), got.UID(), got.IsRead(), got.Size())
	}
	if hits := searchFor(t, store, "quarterlyzebra"); len(hits) != 1 {
		t.Errorf("search hits = %d, want the filed message indexed exactly once", len(hits))
	}
}

func TestRehomeMessagesWithNothingToFileDoesNothing(t *testing.T) {
	if err := openTestStore(t).RehomeMessages(context.Background(), nil); err != nil {
		t.Fatalf("RehomeMessages(nil): %v", err)
	}
}
