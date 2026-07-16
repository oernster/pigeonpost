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
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return err
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
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return err
	}
	if err := s.remote.SetFlagged(ctx, account, folder, msg.UID(), flagged); err != nil {
		return fmt.Errorf("set server flagged for %q: %w", messageID, err)
	}
	if err := s.store.SetFlagged(ctx, messageID, flagged); err != nil {
		return fmt.Errorf("set cached flagged for %q: %w", messageID, err)
	}
	return nil
}

// MarkAnswered sets or clears a message's answered (\Answered) state on the server and then in the cache. It
// is called after a reply is sent, so the original message shows the replied indicator.
func (s *MessageActionService) MarkAnswered(ctx context.Context, messageID string, answered bool) error {
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return err
	}
	if err := s.remote.SetAnswered(ctx, account, folder, msg.UID(), answered); err != nil {
		return fmt.Errorf("set server answered for %q: %w", messageID, err)
	}
	if err := s.store.SetAnswered(ctx, messageID, answered); err != nil {
		return fmt.Errorf("set cached answered for %q: %w", messageID, err)
	}
	return nil
}

// MarkForwarded sets or clears a message's forwarded ($Forwarded) state on the server and then in the cache. It
// is called after a message is forwarded, so the original shows the forwarded indicator.
func (s *MessageActionService) MarkForwarded(ctx context.Context, messageID string, forwarded bool) error {
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return err
	}
	if err := s.remote.SetForwarded(ctx, account, folder, msg.UID(), forwarded); err != nil {
		return fmt.Errorf("set server forwarded for %q: %w", messageID, err)
	}
	if err := s.store.SetForwarded(ctx, messageID, forwarded); err != nil {
		return fmt.Errorf("set cached forwarded for %q: %w", messageID, err)
	}
	return nil
}

// Delete removes a message from the server and the local cache. It moves the message to the account's
// Trash folder when one exists; if the message already lives in Trash, or the account has no Trash
// folder, it is deleted permanently. When the message moved to Trash and the server reported where it
// landed (COPYUID), the returned id is the one it will carry there, so the caller can undo the delete
// by moving it back; a permanent deletion (or a server that reports nothing) returns an empty id.
func (s *MessageActionService) Delete(ctx context.Context, messageID string) (string, error) {
	return s.delete(ctx, messageID, false)
}

// DeletePermanent removes a message from the server and the local cache without moving it to Trash,
// regardless of which folder it lives in. It is the irreversible counterpart to Delete.
func (s *MessageActionService) DeletePermanent(ctx context.Context, messageID string) error {
	_, err := s.delete(ctx, messageID, true)
	return err
}

// delete is the shared core of Delete and DeletePermanent. When permanent is false the destination is
// resolved from the account's Trash folder (move to Trash, or permanent when no Trash applies); when
// permanent is true the trash path is always empty, forcing an immediate permanent deletion.
func (s *MessageActionService) delete(ctx context.Context, messageID string, permanent bool) (string, error) {
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return "", err
	}
	trashPath, trashFolderID := "", ""
	if !permanent {
		trash, ok, err := s.trashFolder(ctx, folder)
		if err != nil {
			return "", fmt.Errorf("resolve trash for %q: %w", messageID, err)
		}
		if ok {
			trashPath, trashFolderID = trash.Path(), trash.ID()
		}
	}
	newUID, err := s.remote.Delete(ctx, account, folder, msg.UID(), trashPath)
	if err != nil {
		return "", fmt.Errorf("delete message %q on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return "", fmt.Errorf("delete cached message %q: %w", messageID, err)
	}
	if trashFolderID == "" || newUID == "" {
		return "", nil
	}
	return domain.MessageIDFor(trashFolderID, newUID), nil
}

// DeleteMany removes several messages in as few server round trips as possible: it groups them by
// folder and issues one batched server delete per folder (moving each folder's messages to Trash or
// deleting them permanently when permanent is true or the folder has no Trash), rather than a fresh
// connection per message. It returns the ids that were removed from the server so the caller can drop
// exactly those from the UI, plus each removed id's new id in its Trash folder where the server
// reported one (COPYUID; empty for permanent deletions), so the caller can undo the delete by moving
// the messages back. A folder whose batch fails leaves its messages in place and contributes the
// returned error, so a partial failure is never silent.
func (s *MessageActionService) DeleteMany(ctx context.Context, messageIDs []string, permanent bool) ([]string, map[string]string, error) {
	type batch struct {
		account       domain.Account
		folder        domain.Folder
		trashPath     string
		trashFolderID string
		uids          []string
		ids           []string
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
			trashPath, trashFolderID := "", ""
			if !permanent {
				trash, ok, err := s.trashFolder(ctx, folder)
				if err != nil {
					errs = append(errs, fmt.Errorf("resolve trash for %q: %w", msg.FolderID(), err))
					continue
				}
				if ok {
					trashPath, trashFolderID = trash.Path(), trash.ID()
				}
			}
			b = &batch{account: account, folder: folder, trashPath: trashPath, trashFolderID: trashFolderID}
			batches[msg.FolderID()] = b
			order = append(order, msg.FolderID())
		}
		b.uids = append(b.uids, msg.UID())
		b.ids = append(b.ids, id)
	}
	deleted := make([]string, 0, len(messageIDs))
	newIDs := map[string]string{}
	for _, folderID := range order {
		b := batches[folderID]
		movedUIDs, err := s.remote.DeleteMany(ctx, b.account, b.folder, b.uids, b.trashPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("delete %d messages in %q on server: %w", len(b.uids), folderID, err))
			continue
		}
		// The server delete succeeded, so each message is gone remotely: drop it from the UI even if the
		// cache row cannot be removed (the next sync reconciles the cache) and report the cache error.
		for i, id := range b.ids {
			if err := s.store.DeleteMessage(ctx, id); err != nil {
				errs = append(errs, fmt.Errorf("delete cached message %q: %w", id, err))
			}
			deleted = append(deleted, id)
			if newUID, ok := movedUIDs[b.uids[i]]; ok && b.trashFolderID != "" {
				newIDs[id] = domain.MessageIDFor(b.trashFolderID, newUID)
			}
		}
	}
	return deleted, newIDs, errors.Join(errs...)
}

// MoveMany relocates several messages into destFolderID in as few server round trips as possible: it
// groups them by source folder and issues one batched server move per folder, rather than a fresh
// connection per message. Every message must belong to the same account as the destination. It returns
// the ids that moved so the caller can drop exactly those from the source list, plus each moved id's
// new id in the destination where the server reported one (COPYUID), so the caller can undo the move.
// A folder whose batch fails leaves its messages in place and contributes the returned error. A
// message already in the destination is skipped.
func (s *MessageActionService) MoveMany(ctx context.Context, messageIDs []string, destFolderID string) ([]string, map[string]string, error) {
	dest, err := s.store.GetFolder(ctx, destFolderID)
	if err != nil {
		return nil, nil, fmt.Errorf("locate destination folder %q: %w", destFolderID, err)
	}
	account, err := s.accounts.GetAccount(ctx, dest.AccountID())
	if err != nil {
		return nil, nil, fmt.Errorf("locate account %q: %w", dest.AccountID(), err)
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
	newIDs := map[string]string{}
	for _, folderID := range order {
		b := batches[folderID]
		movedUIDs, err := s.remote.MoveMany(ctx, account, b.folder, b.uids, dest.Path())
		if err != nil {
			errs = append(errs, fmt.Errorf("move %d messages from %q on server: %w", len(b.uids), folderID, err))
			continue
		}
		// The server move succeeded, so each message left its source folder: drop it from the cache even
		// if the row cannot be removed (the next sync reconciles) and report the cache error.
		for i, id := range b.ids {
			if err := s.store.DeleteMessage(ctx, id); err != nil {
				errs = append(errs, fmt.Errorf("remove moved message %q from cache: %w", id, err))
			}
			moved = append(moved, id)
			if newUID, ok := movedUIDs[b.uids[i]]; ok {
				newIDs[id] = domain.MessageIDFor(destFolderID, newUID)
			}
		}
	}
	return moved, newIDs, errors.Join(errs...)
}

// Move relocates a message to another folder within the same account: it is moved on the server and
// then removed from the local cache (the destination folder re-lists it, with its new UID, on the
// next sync). When the server reported where the message landed (COPYUID), the returned id is the
// one it will carry in the destination, so the caller can undo the move by addressing it there; a
// server that reports nothing returns an empty id.
func (s *MessageActionService) Move(ctx context.Context, messageID, destFolderID string) (string, error) {
	msg, source, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return "", err
	}
	dest, err := s.store.GetFolder(ctx, destFolderID)
	if err != nil {
		return "", fmt.Errorf("locate destination folder %q: %w", destFolderID, err)
	}
	if dest.AccountID() != account.ID() {
		return "", fmt.Errorf("cannot move message %q to a folder in another account", messageID)
	}
	newUID, err := s.remote.Move(ctx, account, source, msg.UID(), dest.Path())
	if err != nil {
		return "", fmt.Errorf("move message %q on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return "", fmt.Errorf("remove moved message %q from cache: %w", messageID, err)
	}
	if newUID == "" {
		return "", nil
	}
	return domain.MessageIDFor(destFolderID, newUID), nil
}

// Copy duplicates a message into another folder within the same account: it is copied on the server and
// left in place locally (the destination folder lists the new copy, with its own server UID, on the
// next sync). Unlike Move, the original message is untouched. When the server reported the duplicate's
// place (COPYUID), the returned id is the one it carries in the destination, so the caller can show it
// there ahead of the sync; a server that reports nothing returns an empty id.
func (s *MessageActionService) Copy(ctx context.Context, messageID, destFolderID string) (string, error) {
	msg, source, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return "", err
	}
	dest, err := s.store.GetFolder(ctx, destFolderID)
	if err != nil {
		return "", fmt.Errorf("locate destination folder %q: %w", destFolderID, err)
	}
	if dest.AccountID() != account.ID() {
		return "", fmt.Errorf("cannot copy message %q to a folder in another account", messageID)
	}
	newUID, err := s.remote.Copy(ctx, account, source, msg.UID(), dest.Path())
	if err != nil {
		return "", fmt.Errorf("copy message %q on server: %w", messageID, err)
	}
	if newUID == "" {
		return "", nil
	}
	return domain.MessageIDFor(destFolderID, newUID), nil
}

// trashFolder returns the destination folder for a delete: the account's Trash folder, or false
// (meaning permanent deletion) when the message is already in Trash or no Trash folder exists.
func (s *MessageActionService) trashFolder(ctx context.Context, current domain.Folder) (domain.Folder, bool, error) {
	if current.Kind() == domain.FolderTrash {
		return domain.Folder{}, false, nil
	}
	return folderByKind(ctx, s.store, current.AccountID(), domain.FolderTrash)
}
