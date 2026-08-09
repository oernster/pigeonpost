package application

import (
	"context"
	"errors"
	"testing"
)

// fakeFolderUIStateStore is a hand-written in-memory FolderUIStateStore with error-injection fields.
type fakeFolderUIStateStore struct {
	order     map[string][]string
	collapsed map[string][]string
	loadErr   error
	saveErr   error
}

func newFakeFolderUIStateStore() *fakeFolderUIStateStore {
	return &fakeFolderUIStateStore{order: map[string][]string{}, collapsed: map[string][]string{}}
}

func (f *fakeFolderUIStateStore) FolderUIState(_ context.Context, accountID string) ([]string, []string, error) {
	if f.loadErr != nil {
		return nil, nil, f.loadErr
	}
	return f.order[accountID], f.collapsed[accountID], nil
}

func (f *fakeFolderUIStateStore) SaveFolderUIState(_ context.Context, accountID string, order, collapsed []string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.order[accountID] = append([]string(nil), order...)
	f.collapsed[accountID] = append([]string(nil), collapsed...)
	return nil
}

func TestFolderUIStateServiceRoundTrip(t *testing.T) {
	store := newFakeFolderUIStateStore()
	svc := NewFolderUIStateService(store)
	ctx := context.Background()

	if err := svc.Save(ctx, "a1", []string{"Work", "Home"}, []string{"Archive"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	order, collapsed, err := svc.Load(ctx, "a1")
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

func TestFolderUIStateServiceWrapsErrors(t *testing.T) {
	store := newFakeFolderUIStateStore()
	svc := NewFolderUIStateService(store)
	ctx := context.Background()

	store.loadErr = errBoom
	if _, _, err := svc.Load(ctx, "a1"); !errors.Is(err, errBoom) {
		t.Errorf("Load error = %v, want wrapped boom", err)
	}
	store.saveErr = errBoom
	if err := svc.Save(ctx, "a1", nil, nil); !errors.Is(err, errBoom) {
		t.Errorf("Save error = %v, want wrapped boom", err)
	}
}
