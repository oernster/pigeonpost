package storage

import (
	"context"
	"testing"
)

func TestFolderUIStateEmptyWhenUnsaved(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)

	order, collapsed, err := store.FolderUIState(context.Background(), "a1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(order) != 0 || len(collapsed) != 0 {
		t.Fatalf("unsaved state = %v / %v, want empty lists", order, collapsed)
	}
}

func TestFolderUIStateRoundTrip(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveFolderUIState(ctx, "a1", []string{"Work", "Home"}, []string{"Archive"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	order, collapsed, err := store.FolderUIState(ctx, "a1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(order) != 2 || order[0] != "Work" || order[1] != "Home" {
		t.Fatalf("order = %v, want [Work Home]", order)
	}
	if len(collapsed) != 1 || collapsed[0] != "Archive" {
		t.Fatalf("collapsed = %v, want [Archive]", collapsed)
	}
}

func TestFolderUIStateSaveReplacesAndIsPerAccount(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveFolderUIState(ctx, "a1", []string{"Work"}, []string{"Archive"}); err != nil {
		t.Fatalf("save a1: %v", err)
	}
	if err := store.SaveFolderUIState(ctx, "a2", []string{"Other"}, nil); err != nil {
		t.Fatalf("save a2: %v", err)
	}
	// A re-save replaces the whole state, including clearing lists back to empty.
	if err := store.SaveFolderUIState(ctx, "a1", []string{"Home", "Work"}, nil); err != nil {
		t.Fatalf("resave a1: %v", err)
	}

	order, collapsed, err := store.FolderUIState(ctx, "a1")
	if err != nil {
		t.Fatalf("load a1: %v", err)
	}
	if len(order) != 2 || order[0] != "Home" || len(collapsed) != 0 {
		t.Fatalf("a1 state = %v / %v, want [Home Work] / []", order, collapsed)
	}

	otherOrder, _, err := store.FolderUIState(ctx, "a2")
	if err != nil {
		t.Fatalf("load a2: %v", err)
	}
	if len(otherOrder) != 1 || otherOrder[0] != "Other" {
		t.Fatalf("a2 order = %v, want [Other]", otherOrder)
	}
}
