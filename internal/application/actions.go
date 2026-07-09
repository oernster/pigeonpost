package application

import (
	"context"
	"errors"
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
	return s.delete(ctx, messageID, false)
}

// DeletePermanent removes a message from the server and the local cache without moving it to Trash,
// regardless of which folder it lives in. It is the irreversible counterpart to Delete.
func (s *MessageActionService) DeletePermanent(ctx context.Context, messageID string) error {
	return s.delete(ctx, messageID, true)
}

// delete is the shared core of Delete and DeletePermanent. When permanent is false the destination is
// resolved from trashPath (move to Trash, or permanent when no Trash applies); when permanent is true
// the trash path is always empty, forcing an immediate permanent deletion.
func (s *MessageActionService) delete(ctx context.Context, messageID string, permanent bool) error {
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
	trashPath := ""
	if !permanent {
		trashPath, err = s.trashPath(ctx, folder)
		if err != nil {
			return fmt.Errorf("resolve trash for %q: %w", messageID, err)
		}
	}
	if err := s.remote.Delete(ctx, account, folder, msg.UID(), trashPath); err != nil {
		return fmt.Errorf("delete message %q on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return fmt.Errorf("delete cached message %q: %w", messageID, err)
	}
	return nil
}

// DeleteMany removes several messages in as few server round trips as possible: it groups them by
// folder and issues one batched server delete per folder (moving each folder's messages to Trash or
// deleting them permanently when permanent is true or the folder has no Trash), rather than a fresh
// connection per message. It returns the ids that were removed from the server so the caller can drop
// exactly those from the UI; a folder whose batch fails leaves its messages in place and contributes
// the returned error, so a partial failure is never silent.
func (s *MessageActionService) DeleteMany(ctx context.Context, messageIDs []string, permanent bool) ([]string, error) {
	type batch struct {
		account   domain.Account
		folder    domain.Folder
		trashPath string
		uids      []string
		ids       []string
	}
	batches := map[string]*batch{}
	order := make([]string, 0)
	var errs []error
	for _, id := range messageIDs {
		msg, err := s.store.GetMessage(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("locate message %q: %w", id, err))
			continue
		}
		b, ok := batches[msg.FolderID()]
		if !ok {
			folder, err := s.store.GetFolder(ctx, msg.FolderID())
			if err != nil {
				errs = append(errs, fmt.Errorf("locate folder %q: %w", msg.FolderID(), err))
				continue
			}
			account, err := s.accounts.GetAccount(ctx, folder.AccountID())
			if err != nil {
				errs = append(errs, fmt.Errorf("locate account %q: %w", folder.AccountID(), err))
				continue
			}
			trashPath := ""
			if !permanent {
				trashPath, err = s.trashPath(ctx, folder)
				if err != nil {
					errs = append(errs, fmt.Errorf("resolve trash for %q: %w", msg.FolderID(), err))
					continue
				}
			}
			b = &batch{account: account, folder: folder, trashPath: trashPath}
			batches[msg.FolderID()] = b
			order = append(order, msg.FolderID())
		}
		b.uids = append(b.uids, msg.UID())
		b.ids = append(b.ids, id)
	}
	deleted := make([]string, 0, len(messageIDs))
	for _, folderID := range order {
		b := batches[folderID]
		if err := s.remote.DeleteMany(ctx, b.account, b.folder, b.uids, b.trashPath); err != nil {
			errs = append(errs, fmt.Errorf("delete %d messages in %q on server: %w", len(b.uids), folderID, err))
			continue
		}
		// The server delete succeeded, so each message is gone remotely: drop it from the UI even if the
		// cache row cannot be removed (the next sync reconciles the cache) and report the cache error.
		for _, id := range b.ids {
			if err := s.store.DeleteMessage(ctx, id); err != nil {
				errs = append(errs, fmt.Errorf("delete cached message %q: %w", id, err))
			}
			deleted = append(deleted, id)
		}
	}
	return deleted, errors.Join(errs...)
}

// MoveMany relocates several messages into destFolderID in as few server round trips as possible: it
// groups them by source folder and issues one batched server move per folder, rather than a fresh
// connection per message. Every message must belong to the same account as the destination. It returns
// the ids that moved so the caller can drop exactly those from the source list; a folder whose batch
// fails leaves its messages in place and contributes the returned error. A message already in the
// destination is skipped.
func (s *MessageActionService) MoveMany(ctx context.Context, messageIDs []string, destFolderID string) ([]string, error) {
	dest, err := s.store.GetFolder(ctx, destFolderID)
	if err != nil {
		return nil, fmt.Errorf("locate destination folder %q: %w", destFolderID, err)
	}
	account, err := s.accounts.GetAccount(ctx, dest.AccountID())
	if err != nil {
		return nil, fmt.Errorf("locate account %q: %w", dest.AccountID(), err)
	}
	type batch struct {
		folder domain.Folder
		uids   []string
		ids    []string
	}
	batches := map[string]*batch{}
	order := make([]string, 0)
	var errs []error
	for _, id := range messageIDs {
		msg, err := s.store.GetMessage(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("locate message %q: %w", id, err))
			continue
		}
		if msg.FolderID() == destFolderID {
			continue
		}
		b, ok := batches[msg.FolderID()]
		if !ok {
			folder, err := s.store.GetFolder(ctx, msg.FolderID())
			if err != nil {
				errs = append(errs, fmt.Errorf("locate folder %q: %w", msg.FolderID(), err))
				continue
			}
			if folder.AccountID() != account.ID() {
				errs = append(errs, fmt.Errorf("cannot move message %q to a folder in another account", id))
				continue
			}
			b = &batch{folder: folder}
			batches[msg.FolderID()] = b
			order = append(order, msg.FolderID())
		}
		b.uids = append(b.uids, msg.UID())
		b.ids = append(b.ids, id)
	}
	moved := make([]string, 0, len(messageIDs))
	for _, folderID := range order {
		b := batches[folderID]
		if err := s.remote.MoveMany(ctx, account, b.folder, b.uids, dest.Path()); err != nil {
			errs = append(errs, fmt.Errorf("move %d messages from %q on server: %w", len(b.uids), folderID, err))
			continue
		}
		// The server move succeeded, so each message left its source folder: drop it from the cache even
		// if the row cannot be removed (the next sync reconciles) and report the cache error.
		for _, id := range b.ids {
			if err := s.store.DeleteMessage(ctx, id); err != nil {
				errs = append(errs, fmt.Errorf("remove moved message %q from cache: %w", id, err))
			}
			moved = append(moved, id)
		}
	}
	return moved, errors.Join(errs...)
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

// Copy duplicates a message into another folder within the same account: it is copied on the server and
// left in place locally (the destination folder lists the new copy, with its own server UID, on the
// next sync). Unlike Move, the original message is untouched.
func (s *MessageActionService) Copy(ctx context.Context, messageID, destFolderID string) error {
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
		return fmt.Errorf("cannot copy message %q to a folder in another account", messageID)
	}
	if err := s.remote.Copy(ctx, account, source, msg.UID(), dest.Path()); err != nil {
		return fmt.Errorf("copy message %q on server: %w", messageID, err)
	}
	return nil
}

// MarkJunk moves a message to the account's Junk (spam) folder, filing unwanted mail out of the inbox.
// It is moved on the server and removed from the local cache, the same mechanism as Move. It fails with
// ErrAlreadyJunk when the message already lives in Junk, and ErrNoJunkFolder when the account has none.
func (s *MessageActionService) MarkJunk(ctx context.Context, messageID string) error {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("locate message %q: %w", messageID, err)
	}
	source, err := s.store.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	if source.Kind() == domain.FolderJunk {
		return ErrAlreadyJunk
	}
	account, err := s.accounts.GetAccount(ctx, source.AccountID())
	if err != nil {
		return fmt.Errorf("locate account %q: %w", source.AccountID(), err)
	}
	junkPath, err := s.junkPath(ctx, source)
	if err != nil {
		return fmt.Errorf("resolve junk for %q: %w", messageID, err)
	}
	if junkPath == "" {
		return ErrNoJunkFolder
	}
	if err := s.remote.Move(ctx, account, source, msg.UID(), junkPath); err != nil {
		return fmt.Errorf("move message %q to junk on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return fmt.Errorf("remove junked message %q from cache: %w", messageID, err)
	}
	return nil
}

// junkPath returns the path of the account's Junk folder, or an empty string when the account has none.
func (s *MessageActionService) junkPath(ctx context.Context, current domain.Folder) (string, error) {
	folders, err := s.store.ListFolders(ctx, current.AccountID())
	if err != nil {
		return "", err
	}
	for _, folder := range folders {
		if folder.Kind() == domain.FolderJunk {
			return folder.Path(), nil
		}
	}
	return "", nil
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
