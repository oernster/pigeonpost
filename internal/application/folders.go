package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// FolderService is the use-case boundary for managing an account's mailbox structure: creating,
// renaming and deleting folders. Each operation is applied to the server first, then the cached folder
// list is refreshed so the change shows without waiting for a full sync.
type FolderService struct {
	accounts AccountStore
	store    MailStore
	source   MailSource
	remote   FolderActions
}

// NewFolderService constructs the service with its injected dependencies.
func NewFolderService(accounts AccountStore, store MailStore, source MailSource, remote FolderActions) *FolderService {
	return &FolderService{accounts: accounts, store: store, source: source, remote: remote}
}

// Create creates a new mailbox (name is its full path) on the server, then refreshes the cached list.
func (s *FolderService) Create(ctx context.Context, accountID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyFolderName
	}
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("folders: load account %q: %w", accountID, err)
	}
	if err := s.remote.CreateFolder(ctx, account, name); err != nil {
		return fmt.Errorf("folders: create %q: %w", name, err)
	}
	return s.refresh(ctx, account)
}

// Rename changes a folder's leaf name on the server, keeping its parent hierarchy, then refreshes the
// cached list. The owning account is taken from the folder itself.
func (s *FolderService) Rename(ctx context.Context, folderID, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return ErrEmptyFolderName
	}
	folder, err := s.store.GetFolder(ctx, folderID)
	if err != nil {
		return fmt.Errorf("folders: locate folder %q: %w", folderID, err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return fmt.Errorf("folders: load account %q: %w", folder.AccountID(), err)
	}
	if err := s.remote.RenameFolder(ctx, account, folder.Path(), folder.RenamedTo(newName)); err != nil {
		return fmt.Errorf("folders: rename %q: %w", folder.Path(), err)
	}
	return s.refresh(ctx, account)
}

// Delete removes a folder on the server, clears its cached messages, then refreshes the cached list.
func (s *FolderService) Delete(ctx context.Context, folderID string) error {
	folder, err := s.store.GetFolder(ctx, folderID)
	if err != nil {
		return fmt.Errorf("folders: locate folder %q: %w", folderID, err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return fmt.Errorf("folders: load account %q: %w", folder.AccountID(), err)
	}
	if err := s.remote.DeleteFolder(ctx, account, folder.Path()); err != nil {
		return fmt.Errorf("folders: delete %q: %w", folder.Path(), err)
	}
	if err := s.store.SaveMessages(ctx, folderID, nil); err != nil {
		return fmt.Errorf("folders: clear cached messages for %q: %w", folderID, err)
	}
	return s.refresh(ctx, account)
}

// refresh re-fetches the account's folder list from the server and replaces the cached copy.
func (s *FolderService) refresh(ctx context.Context, account domain.Account) error {
	folders, err := s.source.FetchFolders(ctx, account)
	if err != nil {
		return fmt.Errorf("folders: refresh list: %w", err)
	}
	if err := s.store.SaveFolders(ctx, account.ID(), folders); err != nil {
		return fmt.Errorf("folders: save list: %w", err)
	}
	return nil
}
