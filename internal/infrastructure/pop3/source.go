package pop3

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// ErrUnsupported is returned for mailbox operations POP3 has no equivalent of, such as moving or
// copying a message between server folders. The UI hides these for POP3 accounts, so it is a guard
// rather than a path a user reaches.
var ErrUnsupported = errors.New("pop3: operation not supported")

// PasswordProvider yields the secret used to authenticate an account. It is backed by the OS keychain
// so passwords never touch the local database.
type PasswordProvider interface {
	Password(ctx context.Context, account domain.Account) (string, error)
}

// Source is the POP3-backed read surface: it implements the folder and message fetch operations plus
// account verification for POP3 accounts. POP3 has no server-side folders, flags or mailbox actions,
// so it exposes a single synthetic Inbox and nothing writable.
type Source struct {
	passwords PasswordProvider
}

// NewSource constructs the source with its injected password provider.
func NewSource(passwords PasswordProvider) *Source {
	return &Source{passwords: passwords}
}

// connect dials and authenticates using the account's stored keychain password.
func (s *Source) connect(ctx context.Context, account domain.Account) (*Client, error) {
	password, err := s.passwords.Password(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("pop3: password for %q: %w", account.ID(), err)
	}
	return s.login(account, password)
}

// login dials the incoming server and authenticates with the given password.
func (s *Source) login(account domain.Account, password string) (*Client, error) {
	client, err := dial(account.Incoming())
	if err != nil {
		return nil, err
	}
	if err := client.Auth(account.Address().Address(), password); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("pop3: login %q: %w", account.ID(), err)
	}
	return client, nil
}

// Verify proves a candidate password against the incoming server by authenticating and quitting. It
// satisfies application.AccountVerifier and runs before an account is persisted.
func (s *Source) Verify(_ context.Context, account domain.Account, password string) error {
	client, err := s.login(account, password)
	if err != nil {
		return err
	}
	return client.Quit()
}

// FetchFolders returns the single synthetic Inbox for a POP3 account; POP3 has no server-side folders.
func (s *Source) FetchFolders(_ context.Context, account domain.Account) ([]domain.Folder, error) {
	folder, err := domain.NewFolderWithSeparator(
		inboxID(account.ID()), account.ID(), inboxPath, "", domain.FolderInbox, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("pop3: build inbox folder: %w", err)
	}
	return []domain.Folder{folder}, nil
}

// FetchMessages lists the mailbox by UIDL and builds a summary for each message from its headers. It
// prefers TOP (headers only) and falls back to RETR for servers that do not support TOP.
func (s *Source) FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error) {
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Quit() }()

	items, err := client.UIDL()
	if err != nil {
		return nil, err
	}
	sizes, err := client.List()
	if err != nil {
		return nil, err
	}

	messages := make([]domain.MessageSummary, 0, len(items))
	for _, item := range items {
		header, err := client.Top(item.Number, topHeaderLines)
		if err != nil {
			// TOP is optional in RFC 1939; fall back to the full message when a server rejects it.
			header, err = client.Retr(item.Number)
			if err != nil {
				return nil, err
			}
		}
		message, err := buildSummary(folder.ID(), item.UID, header, sizes[item.Number])
		if err != nil {
			return nil, fmt.Errorf("pop3: build message %q: %w", item.UID, err)
		}
		messages = append(messages, message)
	}
	return messages, nil
}

// FetchBody fetches and parses a message's body into its plain-text and HTML forms plus any
// text/calendar scheduling payload.
func (s *Source) FetchBody(ctx context.Context, account domain.Account, _ domain.Folder, uid string) (string, string, []byte, error) {
	raw, err := s.fetchRaw(ctx, account, uid)
	if err != nil {
		return "", "", nil, err
	}
	parsed, err := mailparse.ParseBody(raw)
	if err != nil {
		return "", "", nil, err
	}
	return parsed.Plain, parsed.HTML, parsed.Invite, nil
}

// FetchRaw returns the full raw RFC822 bytes of a message by its UIDL, for export and forwarding.
func (s *Source) FetchRaw(ctx context.Context, account domain.Account, _ domain.Folder, uid string) ([]byte, error) {
	return s.fetchRaw(ctx, account, uid)
}

// fetchRaw resolves a UIDL to its current session number and retrieves the full message. A fresh
// connection is opened because POP3 renumbers messages per session, so the mapping must be current.
func (s *Source) fetchRaw(ctx context.Context, account domain.Account, uid string) ([]byte, error) {
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Quit() }()

	items, err := client.UIDL()
	if err != nil {
		return nil, err
	}
	number, ok := numberForUID(items, uid)
	if !ok {
		return nil, fmt.Errorf("pop3: message %q not found", uid)
	}
	return client.Retr(number)
}

// SetSeen is a no-op on the server: POP3 has no server-side flags, so a message's read state lives in
// the local cache alone (and is preserved across syncs by the sync service).
func (s *Source) SetSeen(context.Context, domain.Account, domain.Folder, string, bool) error {
	return nil
}

// SetFlagged is a no-op on the server, for the same reason as SetSeen: POP3 has no server-side flags.
func (s *Source) SetFlagged(context.Context, domain.Account, domain.Folder, string, bool) error {
	return nil
}

// Delete permanently removes a message from the server with DELE, committed when the session quits.
// POP3 has no Trash mailbox, so trashPath is ignored and every delete is permanent; leaving the
// message on the server would only re-download it on the next sync.
func (s *Source) Delete(ctx context.Context, account domain.Account, _ domain.Folder, uid string, _ string) error {
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Quit() }()

	items, err := client.UIDL()
	if err != nil {
		return err
	}
	number, ok := numberForUID(items, uid)
	if !ok {
		return fmt.Errorf("pop3: message %q not found", uid)
	}
	return client.Dele(number)
}

// Move is unsupported: POP3 exposes a single mailbox, so there is nowhere to move a message to.
func (s *Source) Move(context.Context, domain.Account, domain.Folder, string, string) error {
	return fmt.Errorf("pop3: move message: %w", ErrUnsupported)
}

// Copy is unsupported, for the same reason as Move.
func (s *Source) Copy(context.Context, domain.Account, domain.Folder, string, string) error {
	return fmt.Errorf("pop3: copy message: %w", ErrUnsupported)
}
