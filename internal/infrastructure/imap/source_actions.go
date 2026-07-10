package imap

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/message"
)

// SetSeen sets or clears the \Seen flag for one message by UID on the server. It satisfies
// application.MailActions. The mailbox is selected read-write so the STORE is permitted.
func (s *Source) SetSeen(ctx context.Context, account domain.Account, folder domain.Folder, uid string, seen bool) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	u, err := parseUID(uid)
	if err != nil {
		return err
	}
	uidSet := imap.UIDSet{}
	uidSet.AddNum(u)
	op := imap.StoreFlagsDel
	if seen {
		op = imap.StoreFlagsAdd
	}
	store := &imap.StoreFlags{Op: op, Silent: true, Flags: []imap.Flag{imap.FlagSeen}}
	if err := client.Store(uidSet, store, nil).Close(); err != nil {
		return fmt.Errorf("imap: store \\Seen uid %q: %w", uid, err)
	}
	return nil
}

// SetFlagged sets or clears the \Flagged flag for one message by UID on the server. It satisfies
// application.MailActions.
func (s *Source) SetFlagged(ctx context.Context, account domain.Account, folder domain.Folder, uid string, flagged bool) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	u, err := parseUID(uid)
	if err != nil {
		return err
	}
	uidSet := imap.UIDSet{}
	uidSet.AddNum(u)
	op := imap.StoreFlagsDel
	if flagged {
		op = imap.StoreFlagsAdd
	}
	store := &imap.StoreFlags{Op: op, Silent: true, Flags: []imap.Flag{imap.FlagFlagged}}
	if err := client.Store(uidSet, store, nil).Close(); err != nil {
		return fmt.Errorf("imap: store \\Flagged uid %q: %w", uid, err)
	}
	return nil
}

// Delete removes a message by UID: it moves it to trashPath when that is set, otherwise marks it
// \Deleted and expunges it permanently. It satisfies application.MailActions.
func (s *Source) Delete(ctx context.Context, account domain.Account, folder domain.Folder, uid string, trashPath string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	u, err := parseUID(uid)
	if err != nil {
		return err
	}
	uidSet := imap.UIDSet{}
	uidSet.AddNum(u)

	if trashPath != "" {
		if _, err := client.Move(uidSet, trashPath).Wait(); err != nil {
			return fmt.Errorf("imap: move uid %q to %q: %w", uid, trashPath, err)
		}
		return nil
	}

	store := &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}
	if err := client.Store(uidSet, store, nil).Close(); err != nil {
		return fmt.Errorf("imap: mark \\Deleted uid %q: %w", uid, err)
	}
	if err := client.Expunge().Close(); err != nil {
		return fmt.Errorf("imap: expunge uid %q: %w", uid, err)
	}
	return nil
}

// Move relocates a message by UID from its folder to destPath on the server. It satisfies
// application.MailActions.
func (s *Source) Move(ctx context.Context, account domain.Account, folder domain.Folder, uid string, destPath string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	u, err := parseUID(uid)
	if err != nil {
		return err
	}
	uidSet := imap.UIDSet{}
	uidSet.AddNum(u)
	if _, err := client.Move(uidSet, destPath).Wait(); err != nil {
		return fmt.Errorf("imap: move uid %q to %q: %w", uid, destPath, err)
	}
	return nil
}

// Copy duplicates a message by UID into destPath on the server, leaving the original untouched. It
// satisfies application.MailActions.
func (s *Source) Copy(ctx context.Context, account domain.Account, folder domain.Folder, uid string, destPath string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	u, err := parseUID(uid)
	if err != nil {
		return err
	}
	uidSet := imap.UIDSet{}
	uidSet.AddNum(u)
	if _, err := client.Copy(uidSet, destPath).Wait(); err != nil {
		return fmt.Errorf("imap: copy uid %q to %q: %w", uid, destPath, err)
	}
	return nil
}

// CreateFolder creates a mailbox at path on the server. It satisfies application.FolderActions.
func (s *Source) CreateFolder(ctx context.Context, account domain.Account, path string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()
	if err := client.Create(path, nil).Wait(); err != nil {
		return fmt.Errorf("imap: create mailbox %q: %w", path, err)
	}
	return nil
}

// RenameFolder renames the mailbox at oldPath to newPath on the server. It satisfies
// application.FolderActions.
func (s *Source) RenameFolder(ctx context.Context, account domain.Account, oldPath, newPath string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()
	if err := client.Rename(oldPath, newPath, nil).Wait(); err != nil {
		return fmt.Errorf("imap: rename mailbox %q to %q: %w", oldPath, newPath, err)
	}
	return nil
}

// DeleteFolder deletes the mailbox at path on the server. It satisfies application.FolderActions.
func (s *Source) DeleteFolder(ctx context.Context, account domain.Account, path string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()
	if err := client.Delete(path).Wait(); err != nil {
		return fmt.Errorf("imap: delete mailbox %q: %w", path, err)
	}
	return nil
}

// MoveAllMessages moves every message in the mailbox at fromPath into the mailbox at toPath, used when
// a stray sent folder is merged into the canonical one. An empty source mailbox is a no-op. It
// satisfies application.FolderActions; the client library falls back to COPY plus expunge when the
// server lacks the MOVE extension.
func (s *Source) MoveAllMessages(ctx context.Context, account domain.Account, fromPath, toPath string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	selected, err := client.Select(fromPath, nil).Wait()
	if err != nil {
		return fmt.Errorf("imap: select %q: %w", fromPath, err)
	}
	if selected.NumMessages == 0 {
		return nil
	}
	var all imap.SeqSet
	all.AddRange(1, 0)
	if _, err := client.Move(all, toPath).Wait(); err != nil {
		return fmt.Errorf("imap: move all messages from %q to %q: %w", fromPath, toPath, err)
	}
	return nil
}

// SaveDraft appends a message to the account's Drafts mailbox, flagged \Draft and \Seen, so it is
// available from any device. It satisfies application.DraftSaver.
func (s *Source) SaveDraft(ctx context.Context, account domain.Account, draftsPath string, msg domain.OutgoingMessage) error {
	return s.appendMessage(ctx, account, draftsPath, msg, []imap.Flag{imap.FlagDraft, imap.FlagSeen})
}

// SaveSent appends a copy of a sent message to the account's Sent mailbox, flagged \Seen, so the user
// keeps a record of what they sent on providers that do not save sent mail server-side. It satisfies
// application.SentSaver.
func (s *Source) SaveSent(ctx context.Context, account domain.Account, sentPath string, msg domain.OutgoingMessage) error {
	return s.appendMessage(ctx, account, sentPath, msg, []imap.Flag{imap.FlagSeen})
}

// appendMessage renders msg to RFC 5322 bytes (with a generated Date and Message-ID so it is a
// well-formed message on the server) and appends them to the given mailbox with the given flags. It is
// the shared body of SaveDraft and SaveSent.
func (s *Source) appendMessage(ctx context.Context, account domain.Account, path string, msg domain.OutgoingMessage, flags []imap.Flag) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	now := s.clock.Now()
	raw := message.BuildMIME(msg, now, s.newID())
	options := &imap.AppendOptions{Flags: flags, Time: now}
	cmd := client.Append(path, int64(len(raw)), options)
	if _, err := cmd.Write(raw); err != nil {
		_ = cmd.Close()
		return fmt.Errorf("imap: write message to %q: %w", path, err)
	}
	if err := cmd.Close(); err != nil {
		return fmt.Errorf("imap: close append to %q: %w", path, err)
	}
	if _, err := cmd.Wait(); err != nil {
		return fmt.Errorf("imap: append message to %q: %w", path, err)
	}
	return nil
}
