package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// junkKeywords and notJunkKeywords are the two spam-verdict keyword spellings in the wild: Apple
// Mail and friends use the $-prefixed pair while Thunderbird and Dovecot use the bare pair. Both
// are written so a client reading either convention agrees with the verdict.
var junkKeywords = [...]string{"$Junk", "Junk"}

var notJunkKeywords = [...]string{"$NotJunk", "NonJunk"}

// setJunkVerdict records a message's spam verdict on the server as keywords: the chosen pair is
// set and the opposite pair cleared. It is best-effort by design: the folder move is the
// authoritative action and keyword support varies by server, so a keyword failure never fails
// the operation.
func (s *MessageActionService) setJunkVerdict(ctx context.Context, account domain.Account, folder domain.Folder, uid string, junk bool) {
	set, clear := junkKeywords, notJunkKeywords
	if !junk {
		set, clear = notJunkKeywords, junkKeywords
	}
	for _, keyword := range set {
		_ = s.remote.SetKeyword(ctx, account, folder, uid, keyword, true)
	}
	for _, keyword := range clear {
		_ = s.remote.SetKeyword(ctx, account, folder, uid, keyword, false)
	}
}

// MarkJunk files a message as spam: it records the junk verdict as keywords (best-effort), moves
// the message to the account's Junk folder on the server and removes it from the local cache, the
// same mechanism as Move. It fails with ErrAlreadyJunk when the message already lives in Junk and
// ErrNoJunkFolder when the account has none. When the server reported where the message landed
// (COPYUID), the returned id is the one it will carry in Junk, so the caller can undo the junking.
func (s *MessageActionService) MarkJunk(ctx context.Context, messageID string) (string, error) {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return "", fmt.Errorf("locate message %q: %w", messageID, err)
	}
	source, err := s.store.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return "", fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	if source.Kind() == domain.FolderJunk {
		return "", ErrAlreadyJunk
	}
	account, err := s.accounts.GetAccount(ctx, source.AccountID())
	if err != nil {
		return "", fmt.Errorf("locate account %q: %w", source.AccountID(), err)
	}
	junk, ok, err := folderByKind(ctx, s.store, source.AccountID(), domain.FolderJunk)
	if err != nil {
		return "", fmt.Errorf("resolve junk for %q: %w", messageID, err)
	}
	if !ok {
		return "", ErrNoJunkFolder
	}
	s.setJunkVerdict(ctx, account, source, msg.UID(), true)
	newUID, err := s.remote.Move(ctx, account, source, msg.UID(), junk.Path())
	if err != nil {
		return "", fmt.Errorf("move message %q to junk on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return "", fmt.Errorf("remove junked message %q from cache: %w", messageID, err)
	}
	if newUID == "" {
		return "", nil
	}
	return domain.MessageIDFor(junk.ID(), newUID), nil
}

// MarkNotJunk rescues a message wrongly filed as spam: it records the not-junk verdict as keywords
// (best-effort, both the $NotJunk and NonJunk conventions), moves the message back to the
// account's Inbox on the server and removes it from the local cache, the same mechanism as Move
// (the inbox re-lists it on the next sync). It fails with ErrNotInJunk when the message is not in
// the Junk folder and ErrNoInboxFolder when the account has no Inbox to return it to. When the
// server reported where the message landed (COPYUID), the returned id is the one it will carry in
// the Inbox, so the caller can undo the rescue.
func (s *MessageActionService) MarkNotJunk(ctx context.Context, messageID string) (string, error) {
	msg, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return "", fmt.Errorf("locate message %q: %w", messageID, err)
	}
	source, err := s.store.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return "", fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	if source.Kind() != domain.FolderJunk {
		return "", ErrNotInJunk
	}
	account, err := s.accounts.GetAccount(ctx, source.AccountID())
	if err != nil {
		return "", fmt.Errorf("locate account %q: %w", source.AccountID(), err)
	}
	inbox, ok, err := folderByKind(ctx, s.store, source.AccountID(), domain.FolderInbox)
	if err != nil {
		return "", fmt.Errorf("resolve inbox for %q: %w", messageID, err)
	}
	if !ok {
		return "", ErrNoInboxFolder
	}
	s.setJunkVerdict(ctx, account, source, msg.UID(), false)
	newUID, err := s.remote.Move(ctx, account, source, msg.UID(), inbox.Path())
	if err != nil {
		return "", fmt.Errorf("move message %q to inbox on server: %w", messageID, err)
	}
	if err := s.store.DeleteMessage(ctx, messageID); err != nil {
		return "", fmt.Errorf("remove rescued message %q from cache: %w", messageID, err)
	}
	if newUID == "" {
		return "", nil
	}
	return domain.MessageIDFor(inbox.ID(), newUID), nil
}
