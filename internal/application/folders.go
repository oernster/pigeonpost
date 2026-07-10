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

// ReconcileSent enforces the single-Sent invariant on the server, so after every account sync there is
// exactly one sent folder and it sits at the top level. Any stray sent-like folder (an old client's
// "Sent Messages", a "Sent" nested under another folder) has its messages moved into the canonical Sent
// and is then deleted; a stray is only deleted after its messages have moved out, so mail is never lost.
// A canonical Sent that itself sits nested is renamed to the top level, unless the server declared its
// position via a special-use attribute (Gmail keeps Sent under [Gmail]) or it sits directly under INBOX
// (an INBOX-namespace server), both of which are the server's own layout and are respected. POP3
// accounts have no server folders, so they are skipped. The pass is idempotent: once one top-level Sent
// remains it does nothing.
func (s *FolderService) ReconcileSent(ctx context.Context, accountID string) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("folders: load account %q: %w", accountID, err)
	}
	if account.Protocol() == domain.ProtocolPOP3 {
		return nil
	}
	folders, err := s.source.FetchFolders(ctx, account)
	if err != nil {
		return fmt.Errorf("folders: reconcile sent: list mailboxes: %w", err)
	}
	var canonical *domain.Folder
	for i := range folders {
		if folders[i].Kind() == domain.FolderSent {
			canonical = &folders[i]
			break
		}
	}
	if canonical == nil {
		return nil
	}
	changed := false
	for _, folder := range folders {
		// Only a Custom folder wearing a sent name is a stray: a folder the server flagged as another
		// role (a real Drafts named oddly) keeps that role and is never merged.
		if folder.ID() == canonical.ID() || folder.Kind() != domain.FolderCustom || !folder.IsSentLike() {
			continue
		}
		if err := s.remote.MoveAllMessages(ctx, account, folder.Path(), canonical.Path()); err != nil {
			return fmt.Errorf("folders: merge %q into %q: %w", folder.Path(), canonical.Path(), err)
		}
		if err := s.store.SaveMessages(ctx, folder.ID(), nil); err != nil {
			return fmt.Errorf("folders: clear cached messages for %q: %w", folder.ID(), err)
		}
		if err := s.remote.DeleteFolder(ctx, account, folder.Path()); err != nil {
			return fmt.Errorf("folders: remove merged %q: %w", folder.Path(), err)
		}
		changed = true
	}
	if target := sentRelocationTarget(*canonical); target != "" {
		if err := s.remote.RenameFolder(ctx, account, canonical.Path(), target); err != nil {
			return fmt.Errorf("folders: relocate %q to top level: %w", canonical.Path(), err)
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return s.refresh(ctx, account)
}

// sentRelocationTarget returns the top-level path the canonical Sent should be renamed to; it returns
// an empty string when the folder must stay where it is: already top level, declared by the server's
// special-use flag or nested directly under INBOX.
func sentRelocationTarget(f domain.Folder) string {
	parent := folderParentPath(f)
	if parent == "" || f.SpecialUse() || strings.EqualFold(parent, "INBOX") {
		return ""
	}
	return f.Name()
}

// folderParentPath returns the path of a folder's parent (an empty string for a top-level folder).
func folderParentPath(f domain.Folder) string {
	suffix := f.Separator() + f.Name()
	if !strings.HasSuffix(f.Path(), suffix) {
		return ""
	}
	return strings.TrimSuffix(f.Path(), suffix)
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
