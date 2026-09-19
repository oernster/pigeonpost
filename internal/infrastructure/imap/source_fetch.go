package imap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// FetchBody fetches and parses the full body of one message by UID, returning its plain-text and HTML
// forms, any text/calendar scheduling payload and its attachments. It satisfies application.MailSource.
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

// bodyStructureTokens name the grammar productions the IMAP client reports when it cannot read a
// server's BODYSTRUCTURE. The library gives no sentinel error for that failure, so these tokens are the
// only handle on it. A library that renames them costs the fallback below, never correctness, so
// TestFetchMessagesFallsBackWhenBodyStructureUnreadable proves the match against an error the library
// itself produced rather than against a string written here.
var bodyStructureTokens = [...]string{
	"body-type-1part",
	"body-type-mpart",
	"body-ext-1part",
	"body-ext-mpart",
	"body-fld-",
}

// isBodyStructureError reports whether a FETCH failed because the server's BODYSTRUCTURE did not fit the
// grammar the client parses. A measured case is a message/rfc822 part carrying NIL where its envelope
// belongs: the client stops mid-response and closes the connection, so one such message costs the whole
// folder its summaries unless the caller asks again without the structure.
func isBodyStructureError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, token := range bodyStructureTokens {
		if strings.Contains(message, token) {
			return true
		}
	}
	return false
}

// FetchMessages returns the header-level summaries for every message in a folder.
func (s *Source) FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error) {
	buffers, err := s.fetchSummaries(ctx, account, folder, true)
	if isBodyStructureError(err) {
		// One unreadable body structure ends the whole FETCH and the connection with it, so without this
		// a single malformed message would cost the folder every summary it holds. Ask again on a fresh
		// connection for everything except the structure: every message then arrives and the only loss is
		// the paperclip that marks an attachment.
		buffers, err = s.fetchSummaries(ctx, account, folder, false)
	}
	if err != nil {
		return nil, err
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

// fetchSummaries opens a connection, selects the folder read-only and collects the FETCH buffers for
// every message in it. withStructure asks for the extended body structure, which carries each part's
// content disposition and is what tells the list whether a message has a saveable attachment (for the
// paperclip), without fetching any bodies; the fallback path leaves it out so a structure the client
// cannot read does not cost the folder its summaries.
func (s *Source) fetchSummaries(ctx context.Context, account domain.Account, folder domain.Folder, withStructure bool) ([]*imapclient.FetchMessageBuffer, error) {
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
	if withStructure {
		options.BodyStructure = &imap.FetchItemBodyStructure{Extended: true}
	}

	buffers, err := client.Fetch(seqSet, options).Collect()
	if err != nil {
		return nil, fmt.Errorf("imap: fetch %q: %w", folder.Path(), markUnreadable(err))
	}
	return buffers, nil
}

// decoderPrefix is what the mail client puts in front of every failure to decode a server's reply. It is
// the one handle on that class of failure, since the library raises no sentinel for it.
const decoderPrefix = "imapwire:"

// markUnreadable labels a reply the client could not decode, so the interface can say that in a sentence
// instead of showing a reader the grammar production that ran out. Anything else is returned untouched.
func markUnreadable(err error) error {
	if err != nil && strings.Contains(err.Error(), decoderPrefix) {
		return errors.Join(err, domain.ErrUnreadableResponse)
	}
	return err
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
