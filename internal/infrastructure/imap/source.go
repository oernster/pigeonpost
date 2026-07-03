package imap

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

// PasswordProvider yields the secret used to authenticate an account. It is backed by the OS
// keychain so passwords never touch the local database.
type PasswordProvider interface {
	Password(ctx context.Context, account domain.Account) (string, error)
}

// Source is a MailSource backed by a live IMAP server.
type Source struct {
	passwords PasswordProvider
}

// NewSource constructs the source with its injected password provider.
func NewSource(passwords PasswordProvider) *Source {
	return &Source{passwords: passwords}
}

func (s *Source) connect(ctx context.Context, account domain.Account) (*imapclient.Client, error) {
	incoming := account.Incoming()
	address := fmt.Sprintf("%s:%d", incoming.Host(), incoming.Port())

	password, err := s.passwords.Password(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("imap: password for %q: %w", account.ID(), err)
	}

	var client *imapclient.Client
	switch incoming.Security() {
	case domain.SecurityStartTLS:
		client, err = imapclient.DialStartTLS(address, nil)
	case domain.SecurityNone:
		client, err = imapclient.DialInsecure(address, nil)
	default:
		client, err = imapclient.DialTLS(address, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("imap: dial %s: %w", address, err)
	}

	if err := client.Login(account.Address().Address(), password).Wait(); err != nil {
		_ = client.Logout().Wait()
		return nil, fmt.Errorf("imap: login %q: %w", account.ID(), err)
	}
	return client, nil
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

// FetchMessages returns the header-level summaries for every message in a folder.
func (s *Source) FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error) {
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout().Wait() }()

	selected, err := client.Select(folder.Path(), &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}
	if selected.NumMessages == 0 {
		return nil, nil
	}

	seqSet := imap.SeqSet{}
	seqSet.AddRange(1, selected.NumMessages)
	options := &imap.FetchOptions{Envelope: true, Flags: true, RFC822Size: true, UID: true}

	buffers, err := client.Fetch(seqSet, options).Collect()
	if err != nil {
		return nil, fmt.Errorf("imap: fetch %q: %w", folder.Path(), err)
	}

	messages := make([]domain.MessageSummary, 0, len(buffers))
	for _, buf := range buffers {
		message, err := buildMessage(folder.ID(), buf)
		if err != nil {
			return nil, fmt.Errorf("imap: build message uid %d: %w", uint32(buf.UID), err)
		}
		messages = append(messages, message)
	}
	return messages, nil
}
