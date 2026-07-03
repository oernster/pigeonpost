package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// MessageActionService is the use-case boundary for actions that change a message on the server as
// well as in the local cache, such as marking it read. The server is updated first so the change is
// durable: a later sync, which mirrors server state into the cache, preserves it.
type MessageActionService struct {
	store    MailStore
	accounts AccountStore
	remote   MailActions
}

// NewMessageActionService constructs the service with its injected store, account store and remote.
func NewMessageActionService(store MailStore, accounts AccountStore, remote MailActions) *MessageActionService {
	return &MessageActionService{store: store, accounts: accounts, remote: remote}
}

// MarkRead sets or clears a message's read (Seen) state on the server and then in the local cache.
func (s *MessageActionService) MarkRead(ctx context.Context, messageID string, read bool) error {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("locate message %q: %w", messageID, err)
	}
	folder, err := s.store.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return fmt.Errorf("locate account %q: %w", folder.AccountID(), err)
	}
	if err := s.remote.SetSeen(ctx, account, folder, msg.UID(), read); err != nil {
		return fmt.Errorf("set server seen for %q: %w", messageID, err)
	}
	if err := s.store.SetSeen(ctx, messageID, read); err != nil {
		return fmt.Errorf("set cached seen for %q: %w", messageID, err)
	}
	return nil
}

// MarkFlagged sets or clears a message's flagged (starred) state on the server and then in the cache.
func (s *MessageActionService) MarkFlagged(ctx context.Context, messageID string, flagged bool) error {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("locate message %q: %w", messageID, err)
	}
	folder, err := s.store.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return fmt.Errorf("locate account %q: %w", folder.AccountID(), err)
	}
	if err := s.remote.SetFlagged(ctx, account, folder, msg.UID(), flagged); err != nil {
		return fmt.Errorf("set server flagged for %q: %w", messageID, err)
	}
	if err := s.store.SetFlagged(ctx, messageID, flagged); err != nil {
		return fmt.Errorf("set cached flagged for %q: %w", messageID, err)
	}
	return nil
}

// Delete removes a message from the server and the local cache. It moves the message to the account's
// Trash folder when one exists; if the message already lives in Trash, or the account has no Trash
// folder, it is deleted permanently.
func (s *MessageActionService) Delete(ctx context.Context, messageID string) error {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("locate message %q: %w", messageID, err)
	}
	folder, err := s.store.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return fmt.Errorf("locate account %q: %w", folder.AccountID(), err)
	}
	trashPath, err := s.trashPath(ctx, folder)
	if err != nil {
		return fmt.Errorf("resolve trash for %q: %w", messageID, err)
	}
	if err := s.remote.Delete(ctx, account, folder, msg.UID(), trashPath); err != nil {
		return fmt.Errorf("delete message %q on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return fmt.Errorf("delete cached message %q: %w", messageID, err)
	}
	return nil
}

// Move relocates a message to another folder within the same account: it is moved on the server and
// then removed from the local cache (the destination folder re-lists it, with its new UID, on the
// next sync).
func (s *MessageActionService) Move(ctx context.Context, messageID, destFolderID string) error {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("locate message %q: %w", messageID, err)
	}
	source, err := s.store.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return fmt.Errorf("locate source folder %q: %w", msg.FolderID(), err)
	}
	account, err := s.accounts.GetAccount(ctx, source.AccountID())
	if err != nil {
		return fmt.Errorf("locate account %q: %w", source.AccountID(), err)
	}
	dest, err := s.store.GetFolder(ctx, destFolderID)
	if err != nil {
		return fmt.Errorf("locate destination folder %q: %w", destFolderID, err)
	}
	if dest.AccountID() != account.ID() {
		return fmt.Errorf("cannot move message %q to a folder in another account", messageID)
	}
	if err := s.remote.Move(ctx, account, source, msg.UID(), dest.Path()); err != nil {
		return fmt.Errorf("move message %q on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return fmt.Errorf("remove moved message %q from cache: %w", messageID, err)
	}
	return nil
}

// trashPath returns the destination mailbox for a delete: the account's Trash folder, or an empty
// string (meaning permanent deletion) when the message is already in Trash or no Trash folder exists.
func (s *MessageActionService) trashPath(ctx context.Context, current domain.Folder) (string, error) {
	if current.Kind() == domain.FolderTrash {
		return "", nil
	}
	folders, err := s.store.ListFolders(ctx, current.AccountID())
	if err != nil {
		return "", err
	}
	for _, folder := range folders {
		if folder.Kind() == domain.FolderTrash {
			return folder.Path(), nil
		}
	}
	return "", nil
}
