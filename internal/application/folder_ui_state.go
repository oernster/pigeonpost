package application

import (
	"context"
	"fmt"
)

// FolderUIStateService loads and saves an account's local folder display state (custom-folder order
// and collapsed paths). The state is presentational and per-user, so it lives in the local store
// rather than on the mail server; keeping it there (not in the WebView's own storage) is what lets
// it survive an application update.
type FolderUIStateService struct {
	store FolderUIStateStore
}

// NewFolderUIStateService constructs the service with its injected store.
func NewFolderUIStateService(store FolderUIStateStore) *FolderUIStateService {
	return &FolderUIStateService{store: store}
}

// Load returns the account's saved order and collapsed path lists, empty when nothing is saved yet.
func (s *FolderUIStateService) Load(ctx context.Context, accountID string) ([]string, []string, error) {
	order, collapsed, err := s.store.FolderUIState(ctx, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("load folder ui state: %w", err)
	}
	return order, collapsed, nil
}

// Save replaces the account's saved order and collapsed path lists with the given full state.
func (s *FolderUIStateService) Save(ctx context.Context, accountID string, order, collapsed []string) error {
	if err := s.store.SaveFolderUIState(ctx, accountID, order, collapsed); err != nil {
		return fmt.Errorf("save folder ui state: %w", err)
	}
	return nil
}
