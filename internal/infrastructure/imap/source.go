package imap

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/message"
)

// parseUID converts an opaque message handle back into the numeric IMAP UID the server expects. On
// IMAP the handle is the UID held as a decimal string, so a value that is not a uint32 is a
// programming error rather than a server condition.
func parseUID(uid string) (imap.UID, error) {
	n, err := strconv.ParseUint(uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("imap: invalid uid %q: %w", uid, err)
	}
	return imap.UID(n), nil
}

// PasswordProvider yields the secret used to authenticate an account. It is backed by the OS
// keychain so passwords never touch the local database.
type PasswordProvider interface {
	Password(ctx context.Context, account domain.Account) (string, error)
}

// IDGenerator produces the local part of a Message-ID for a drafted message.
type IDGenerator func() string

// Source is a MailSource backed by a live IMAP server.
type Source struct {
	passwords PasswordProvider
	clock     domain.Clock
	newID     IDGenerator
}

// NewSource constructs the source with its injected password provider, clock and id generator. The
// clock and id generator are used only when appending a draft, so its Date and Message-ID headers are
// well-formed; the read paths do not use them.
func NewSource(passwords PasswordProvider, clock domain.Clock, newID IDGenerator) *Source {
	return &Source{passwords: passwords, clock: clock, newID: newID}
}

// connect dials and logs in using the account's stored keychain password. It is used by the fetch
// operations, which run against a saved account.
func (s *Source) connect(ctx context.Context, account domain.Account) (*imapclient.Client, error) {
	password, err := s.passwords.Password(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("imap: password for %q: %w", account.ID(), err)
	}
	return s.login(account, password)
}

// login dials the account's incoming server and authenticates with the given password.
func (s *Source) login(account domain.Account, password string) (*imapclient.Client, error) {
	incoming := account.Incoming()
	address := fmt.Sprintf("%s:%d", incoming.Host(), incoming.Port())

	var (
		client *imapclient.Client
		err    error
	)
	switch incoming.Security() {
	case domain.SecurityStartTLS:
		client, err = imapclient.DialStartTLS(address, nil)
	case domain.SecurityNone:
		client, err = imapclient.DialInsecure(address, nil)
	default:
		client, err = imapclient.DialTLS(address, nil)
	}
	if err != nil {
		// A dial failure means the server is unreachable: mark it offline so the caller can queue.
		return nil, fmt.Errorf("imap: dial %s: %w", address, errors.Join(err, domain.ErrOffline))
	}

	if err := client.Login(account.Address().Address(), password).Wait(); err != nil {
		_ = client.Logout().Wait()
		return nil, fmt.Errorf("imap: login %q: %w", account.ID(), err)
	}
	return client, nil
}

// Verify proves a candidate password against the account's incoming server by logging in and out
// again. It satisfies application.AccountVerifier and runs before an account is persisted, so the
// keychain is never written with an unverified secret.
func (s *Source) Verify(_ context.Context, account domain.Account, password string) error {
	client, err := s.login(account, password)
	if err != nil {
		return err
	}
	if err := client.Logout().Wait(); err != nil {
		return fmt.Errorf("imap: logout %q: %w", account.ID(), err)
	}
	return nil
}

// FetchFolders lists the selectable mailboxes on the server for an account.
func (s *Source) FetchFolders(ctx context.Context, account domain.Account) ([]domain.Folder, error) {
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout().Wait() }()

	list, err := client.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("imap: list mailboxes: %w", err)
	}

	folders := make([]domain.Folder, 0, len(list))
	for _, data := range list {
		if hasAttr(data.Attrs, imap.MailboxAttrNonExistent) || hasAttr(data.Attrs, imap.MailboxAttrNoSelect) {
			continue
		}
		folder, err := buildFolder(account.ID(), data)
		if err != nil {
			return nil, fmt.Errorf("imap: build folder %q: %w", data.Mailbox, err)
		}
		folders = append(folders, folder)
	}
	return folders, nil
}

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

// SaveDraft appends a message to the account's Drafts mailbox, flagged \Draft and \Seen, so it is
// available from any device. It satisfies application.DraftSaver. The message is rendered to RFC 5322
// bytes with a generated Date and Message-ID so the draft is a well-formed message on the server.
func (s *Source) SaveDraft(ctx context.Context, account domain.Account, draftsPath string, msg domain.OutgoingMessage) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	now := s.clock.Now()
	raw := message.BuildMIME(msg, now, s.newID())
	options := &imap.AppendOptions{Flags: []imap.Flag{imap.FlagDraft, imap.FlagSeen}, Time: now}
	cmd := client.Append(draftsPath, int64(len(raw)), options)
	if _, err := cmd.Write(raw); err != nil {
		_ = cmd.Close()
		return fmt.Errorf("imap: write draft to %q: %w", draftsPath, err)
	}
	if err := cmd.Close(); err != nil {
		return fmt.Errorf("imap: close draft append to %q: %w", draftsPath, err)
	}
	if _, err := cmd.Wait(); err != nil {
		return fmt.Errorf("imap: append draft to %q: %w", draftsPath, err)
	}
	return nil
}
