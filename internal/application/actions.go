package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// MessageActionService is the use-case boundary for actions that change a message on the server as
// well as in the local cache. Flag changes (read, starred, answered, forwarded) are applied to the
// cache first, together with a pending intent recorded in the same transaction, then pushed to the
// server best-effort: a push that fails (offline; or a server that drops the STORE) leaves the intent
// for the sync to replay, while the sync's reconcile guards the local value against a stale fetch until
// the server confirms it (see FlagSyncService). Destructive actions (delete, move) still talk to the
// server first, because they cannot be overlaid onto a later fetch.
type MessageActionService struct {
	store    MailStore
	accounts AccountStore
	remote   MailActions
}

// NewMessageActionService constructs the service with its injected store, account store and remote.
func NewMessageActionService(store MailStore, accounts AccountStore, remote MailActions) *MessageActionService {
	return &MessageActionService{store: store, accounts: accounts, remote: remote}
}

// MarkRead sets or clears a message's read (Seen) state: cache first (with the pending intent), then the
// server best-effort.
func (s *MessageActionService) MarkRead(ctx context.Context, messageID string, read bool) error {
	return s.markFlag(ctx, messageID, domain.FlagSeen, read)
}

// MarkFlagged sets or clears a message's flagged (starred) state: cache first, then the server best-effort.
func (s *MessageActionService) MarkFlagged(ctx context.Context, messageID string, flagged bool) error {
	return s.markFlag(ctx, messageID, domain.FlagFlagged, flagged)
}

// MarkAnswered sets or clears a message's answered (\Answered) state, called after a reply is sent so the
// original message shows the replied indicator: cache first, then the server best-effort.
func (s *MessageActionService) MarkAnswered(ctx context.Context, messageID string, answered bool) error {
	return s.markFlag(ctx, messageID, domain.FlagAnswered, answered)
}

// MarkForwarded sets or clears a message's forwarded ($Forwarded) state, called after a message is
// forwarded so the original shows the forwarded indicator: cache first, then the server best-effort.
func (s *MessageActionService) MarkForwarded(ctx context.Context, messageID string, forwarded bool) error {
	return s.markFlag(ctx, messageID, domain.FlagForwarded, forwarded)
}

// markFlag is the shared body of the four mark actions. The cache write and the pending intent land in
// one transaction (the intent is recorded for server-flagged accounts only: POP3 has no server flags, so
// its read state is purely local and preserveFlags carries it across syncs). The server push that
// follows is best-effort: a failure (offline; or a server that accepts the STORE and drops it) leaves
// the intent for the sync to replay and the reconcile to guard, so the local change survives either way.
func (s *MessageActionService) markFlag(ctx context.Context, messageID string, flag domain.Flag, value bool) error {
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return err
	}
	recordPending := hasServerFlags(account)
	if err := s.store.SetFlag(ctx, messageID, flag, value, recordPending); err != nil {
		return fmt.Errorf("set cached flag for %q: %w", messageID, err)
	}
	if !recordPending {
		return nil
	}
	switch flag {
	case domain.FlagSeen:
		_ = s.remote.SetSeen(ctx, account, folder, msg.UID(), value)
	case domain.FlagFlagged:
		_ = s.remote.SetFlagged(ctx, account, folder, msg.UID(), value)
	case domain.FlagAnswered:
		_ = s.remote.SetAnswered(ctx, account, folder, msg.UID(), value)
	case domain.FlagForwarded:
		_ = s.remote.SetForwarded(ctx, account, folder, msg.UID(), value)
	}
	return nil
}

// Delete removes a message from the server and the local cache. It moves the message to the account's
// Trash folder when one exists. Two cases are deleted permanently instead: a message already living in
// Trash; a message whose account has no Trash folder to go to. When the message moved to Trash and the server reported where it
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
// resolved from the account's Trash folder (move to Trash; permanent when no Trash applies); when
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
	if permanent {
		// A message that reached Trash has left this folder even when its purge there failed, so its row
		// goes either way and the error is reported alongside.
		handled, left, err := purgeViaTrashReporting(ctx, s.remote, s.store, account, folder, []string{msg.UID()})
		var errs []error
		if err != nil {
			errs = append(errs, fmt.Errorf("delete message %q on server: %w", messageID, err))
		}
		if len(left) > 0 {
			if err := s.store.DeleteMessage(ctx, messageID); err != nil {
				errs = append(errs, fmt.Errorf("delete cached message %q: %w", messageID, err))
			}
		}
		if handled || err != nil {
			return "", errors.Join(errs...)
		}
	}
	newUID, err := s.remote.Delete(ctx, account, folder, msg.UID(), trashPath)
	if err != nil {
		return "", fmt.Errorf("delete message %q on server: %w", messageID, err)
	}
	if trashFolderID == "" {
		if err := s.store.DeleteMessage(ctx, messageID); err != nil {
			return "", fmt.Errorf("delete cached message %q: %w", messageID, err)
		}
		return "", nil
	}
	// A delete to Trash is a move: the message is filed in Trash's cache like any other (see settleMove).
	return s.settleMove(ctx, msg, trashFolderID, newUID)
}

// DeleteMany removes several messages in as few server round trips as possible: it groups them by
// folder and issues one batched server delete per folder (moving each folder's messages to Trash or
// deleting them permanently when permanent is true or the folder has no Trash), rather than a fresh
// connection per message. It returns the ids that were removed from the server so the caller can drop
// exactly those from the UI, plus each removed id's new id in its Trash folder where the server
// reported one (COPYUID; empty for permanent deletions), so the caller can undo the delete by moving
// the messages back. A folder whose batch the server refuses contributes the returned error, so a partial
// failure is never silent; what the server accepted before refusing is still reported as removed.
func (s *MessageActionService) DeleteMany(ctx context.Context, messageIDs []string, permanent bool) ([]string, map[string]string, error) {
	// trashOf holds each batched folder's Trash, keyed by source folder id; absent means delete permanently.
	trashOf := map[string]domain.Folder{}
	rules := batchRules{prepare: func(b *folderBatch) error {
		if permanent {
			return nil
		}
		trash, ok, err := s.trashFolder(ctx, b.folder)
		if err != nil {
			return fmt.Errorf("resolve trash for %q: %w", b.folder.ID(), err)
		}
		if ok {
			trashOf[b.folder.ID()] = trash
		}
		return nil
	}}
	batches, errs := s.batchByFolder(ctx, messageIDs, rules)
	deleted := make([]string, 0, len(messageIDs))
	// trashed records where each message moved to Trash landed, so it is filed there (see landings).
	trashed := newLandings()
	// dropCached removes a message the server no longer holds in its folder: it leaves the UI even if the
	// cache row cannot be removed (the next sync reconciles the cache) and the cache error is reported.
	dropCached := func(id string) {
		if err := s.store.DeleteMessage(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("delete cached message %q: %w", id, err))
		}
		deleted = append(deleted, id)
	}
	for _, b := range batches {
		folderID := b.folder.ID()
		trash, hasTrash := trashOf[folderID]
		trashPath := ""
		if hasTrash {
			trashPath = trash.Path()
		}
		if permanent {
			handled, left, err := purgeViaTrashReporting(ctx, s.remote, s.store, b.account, b.folder, b.uids)
			if err != nil {
				errs = append(errs, fmt.Errorf("delete messages in %q on server: %w", folderID, err))
			}
			for _, id := range b.idsOf(left) {
				dropCached(id)
			}
			if handled || err != nil {
				continue
			}
		}
		// A batch refused part way still deleted what the server accepted. Only those leave the UI; only
		// they can be moved back out of Trash.
		movedUIDs, err := s.remote.DeleteMany(ctx, b.account, b.folder, b.uids, trashPath)
		landed := landedIndexes(b.uids, movedUIDs, err)
		if err != nil {
			errs = append(errs, refusedBatchError("delete", folderID, len(landed), len(b.uids), err))
		}
		for _, i := range landed {
			id := b.ids[i]
			dropCached(id)
			if newUID, ok := movedUIDs[b.uids[i]]; ok && hasTrash {
				trashed.add(id, b.msgs[i], trash.ID(), newUID)
			}
		}
	}
	errs = append(errs, s.fileLandings(ctx, trashed))
	return deleted, trashed.newIDs, errors.Join(errs...)
}

// MoveMany relocates several messages into destFolderID in as few server round trips as possible: it
// groups them by source folder and issues one batched server move per folder, rather than a fresh
// connection per message. Every message must belong to the same account as the destination. It returns
// the ids that moved so the caller can drop exactly those from the source list, plus each moved id's
// new id in the destination where the server reported one (COPYUID), so the caller can undo the move.
// A folder whose batch the server refuses contributes the returned error; what it accepted before
// refusing is still reported as moved. A message already in the destination is skipped.
func (s *MessageActionService) MoveMany(ctx context.Context, messageIDs []string, destFolderID string) ([]string, map[string]string, error) {
	dest, err := s.store.GetFolder(ctx, destFolderID)
	if err != nil {
		return nil, nil, fmt.Errorf("locate destination folder %q: %w", destFolderID, err)
	}
	account, err := s.accounts.GetAccount(ctx, dest.AccountID())
	if err != nil {
		return nil, nil, fmt.Errorf("locate account %q: %w", dest.AccountID(), err)
	}
	rules := batchRules{
		skip: func(msg domain.MessageSummary) bool { return msg.FolderID() == destFolderID },
		admit: func(messageID string, folder domain.Folder) error {
			if folder.AccountID() != account.ID() {
				return fmt.Errorf("cannot move message %q to a folder in another account", messageID)
			}
			return nil
		},
	}
	batches, errs := s.batchByFolder(ctx, messageIDs, rules)
	moved := make([]string, 0, len(messageIDs))
	// arrivals records where each moved message landed, filed in the destination in one step at the end
	// so the destination's next sync knows it rather than reading it as new mail.
	arrivals := newLandings()
	for _, b := range batches {
		folderID := b.folder.ID()
		movedUIDs, err := s.remote.MoveMany(ctx, account, b.folder, b.uids, dest.Path())
		landed := landedIndexes(b.uids, movedUIDs, err)
		if err != nil {
			errs = append(errs, refusedBatchError("move", folderID, len(landed), len(b.uids), err))
		}
		// Each message the server accepted left its source folder, even from a batch refused part way:
		// drop it from the cache even if the row cannot be removed (the next sync reconciles) and report
		// the cache error.
		for _, i := range landed {
			id := b.ids[i]
			if err := s.store.DeleteMessage(ctx, id); err != nil {
				errs = append(errs, fmt.Errorf("remove moved message %q from cache: %w", id, err))
			}
			moved = append(moved, id)
			if newUID, ok := movedUIDs[b.uids[i]]; ok {
				arrivals.add(id, b.msgs[i], destFolderID, newUID)
			}
		}
	}
	errs = append(errs, s.fileLandings(ctx, arrivals))
	return moved, arrivals.newIDs, errors.Join(errs...)
}

// Move relocates a message to another folder within the same account: it is moved on the server, then
// removed from the source folder's cache and filed in the destination's under its new UID where the
// server reported one (see settleMove). That new id (COPYUID) is also returned, so the caller can undo
// the move by addressing the message there; a server that reports nothing returns an empty id.
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
	return s.settleMove(ctx, msg, destFolderID, newUID)
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

// trashFolder returns the destination folder for a delete: the account's Trash folder; else false,
// meaning permanent deletion, when the message is already in Trash or no Trash folder exists.
func (s *MessageActionService) trashFolder(ctx context.Context, current domain.Folder) (domain.Folder, bool, error) {
	if current.Kind() == domain.FolderTrash {
		return domain.Folder{}, false, nil
	}
	return folderByKind(ctx, s.store, current.AccountID(), domain.FolderTrash)
}
