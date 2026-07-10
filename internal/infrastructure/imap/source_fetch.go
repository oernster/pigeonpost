package imap

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// FetchBody fetches and parses the full body of one message by UID, returning its plain-text and HTML
// forms, any text/calendar scheduling payload, and its attachments. It satisfies application.MailSource.
func (s *Source) FetchBody(ctx context.Context, account domain.Account, folder domain.Folder, uid string) (string, string, []byte, []domain.Attachment, error) {
	client, err := s.connect(ctx, account)
	if err != nil {
		return "", "", nil, nil, err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return "", "", nil, nil, fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	u, err := parseUID(uid)
	if err != nil {
		return "", "", nil, nil, err
	}
	uidSet := imap.UIDSet{}
	uidSet.AddNum(u)
	section := &imap.FetchItemBodySection{}
	options := &imap.FetchOptions{UID: true, BodySection: []*imap.FetchItemBodySection{section}}

	buffers, err := client.Fetch(uidSet, options).Collect()
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("imap: fetch body uid %q: %w", uid, err)
	}
	if len(buffers) == 0 {
		return "", "", nil, nil, nil
	}
	raw := buffers[0].FindBodySection(section)
	if raw == nil {
		return "", "", nil, nil, nil
	}
	parsed, err := mailparse.ParseBody(raw)
	if err != nil {
		return "", "", nil, nil, err
	}
	attachments, err := mailparse.DomainAttachments(parsed.Attachments)
	if err != nil {
		return "", "", nil, nil, err
	}
	return parsed.Plain, parsed.HTML, parsed.Invite, attachments, nil
}

// FetchRaw returns the full raw RFC822 bytes of a message by UID, for export (.eml) and for attaching
// an existing message to a new one. It fetches the entire body section without parsing it.
func (s *Source) FetchRaw(ctx context.Context, account domain.Account, folder domain.Folder, uid string) ([]byte, error) {
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return nil, fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	u, err := parseUID(uid)
	if err != nil {
		return nil, err
	}
	uidSet := imap.UIDSet{}
	uidSet.AddNum(u)
	section := &imap.FetchItemBodySection{}
	options := &imap.FetchOptions{UID: true, BodySection: []*imap.FetchItemBodySection{section}}

	buffers, err := client.Fetch(uidSet, options).Collect()
	if err != nil {
		return nil, fmt.Errorf("imap: fetch raw uid %q: %w", uid, err)
	}
	if len(buffers) == 0 {
		return nil, fmt.Errorf("imap: message uid %q not found in %q", uid, folder.Path())
	}
	raw := buffers[0].FindBodySection(section)
	if raw == nil {
		return nil, fmt.Errorf("imap: message uid %q has no body section", uid)
	}
	return raw, nil
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
	// The extended body structure carries each part's content disposition, which is what tells the list
	// whether a message has a saveable attachment (for the paperclip), without fetching any bodies.
	options := &imap.FetchOptions{
		Envelope:      true,
		Flags:         true,
		RFC822Size:    true,
		UID:           true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
	}

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

	selectable := make([]*imap.ListData, 0, len(list))
	for _, data := range list {
		if hasAttr(data.Attrs, imap.MailboxAttrNonExistent) || hasAttr(data.Attrs, imap.MailboxAttrNoSelect) {
			continue
		}
		selectable = append(selectable, data)
	}
	folders, err := buildFolders(account.ID(), selectable)
	if err != nil {
		return nil, fmt.Errorf("imap: build folders: %w", err)
	}
	return folders, nil
}
