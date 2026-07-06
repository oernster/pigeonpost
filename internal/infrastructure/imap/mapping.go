// Package imap implements the application MailSource interface against a live IMAP server using
// emersion/go-imap v2. The pure mapping between IMAP wire types and domain types lives in this file
// so it can be unit-tested without a network connection.
package imap

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// folderIDSeparator joins account, mailbox and uid into stable local identifiers. It is a control
// character that does not appear in mailbox names or email addresses.
const folderIDSeparator = "\x1f"

func makeFolderID(accountID, mailbox string) string {
	return accountID + folderIDSeparator + mailbox
}

func makeMessageID(folderID, uid string) string {
	return fmt.Sprintf("%s%s%s", folderID, folderIDSeparator, uid)
}

// folderKindByLeafName classifies well-known mailboxes by their leaf name, for servers that do not
// advertise the RFC 6154 special-use attributes (so a delete still finds Trash, a sent copy Sent, and
// so on). Keys are lowercased leaf names.
var folderKindByLeafName = map[string]domain.FolderKind{
	"sent":             domain.FolderSent,
	"sent items":       domain.FolderSent,
	"sent mail":        domain.FolderSent,
	"sent messages":    domain.FolderSent,
	"drafts":           domain.FolderDrafts,
	"draft":            domain.FolderDrafts,
	"trash":            domain.FolderTrash,
	"deleted":          domain.FolderTrash,
	"deleted items":    domain.FolderTrash,
	"deleted messages": domain.FolderTrash,
	"bin":              domain.FolderTrash,
	"junk":             domain.FolderJunk,
	"junk email":       domain.FolderJunk,
	"junk e-mail":      domain.FolderJunk,
	"spam":             domain.FolderJunk,
	"archive":          domain.FolderArchive,
	"archives":         domain.FolderArchive,
}

// folderKindFor classifies a mailbox from its name and RFC 6154 special-use attributes. Attributes win
// when present; otherwise the leaf name is matched against the well-known names, so servers that omit
// special-use still get a usable Trash/Sent/Drafts/Junk/Archive classification.
func folderKindFor(mailbox, leaf string, attrs []imap.MailboxAttr) domain.FolderKind {
	if strings.EqualFold(mailbox, "INBOX") {
		return domain.FolderInbox
	}
	for _, attr := range attrs {
		switch attr {
		case imap.MailboxAttrSent:
			return domain.FolderSent
		case imap.MailboxAttrDrafts:
			return domain.FolderDrafts
		case imap.MailboxAttrTrash:
			return domain.FolderTrash
		case imap.MailboxAttrJunk:
			return domain.FolderJunk
		case imap.MailboxAttrArchive:
			return domain.FolderArchive
		}
	}
	if kind, ok := folderKindByLeafName[strings.ToLower(strings.TrimSpace(leaf))]; ok {
		return kind
	}
	return domain.FolderCustom
}

// buildFolder maps a LIST response into a domain folder. Counts are unknown from LIST alone and
// default to zero until a later STATUS/SELECT sync fills them in.
func buildFolder(accountID string, data *imap.ListData) (domain.Folder, error) {
	separator := ""
	if data.Delim != 0 {
		separator = string(data.Delim)
	}
	leaf := data.Mailbox
	if separator != "" {
		if idx := strings.LastIndex(data.Mailbox, separator); idx >= 0 {
			leaf = data.Mailbox[idx+len(separator):]
		}
	}
	kind := folderKindFor(data.Mailbox, leaf, data.Attrs)
	return domain.NewFolderWithSeparator(
		makeFolderID(accountID, data.Mailbox), accountID, data.Mailbox, separator, kind, 0, 0)
}

// mapFlags converts IMAP flags into the domain flag set, ignoring flags the domain does not model.
func mapFlags(flags []imap.Flag) domain.Flags {
	out := domain.NewFlags(0)
	for _, flag := range flags {
		switch flag {
		case imap.FlagSeen:
			out = out.With(domain.FlagSeen)
		case imap.FlagAnswered:
			out = out.With(domain.FlagAnswered)
		case imap.FlagFlagged:
			out = out.With(domain.FlagFlagged)
		case imap.FlagDraft:
			out = out.With(domain.FlagDraft)
		case imap.FlagDeleted:
			out = out.With(domain.FlagDeleted)
		}
	}
	return out
}

// firstAddress maps the first envelope address into a domain address. An empty list or an
// unparseable address yields the zero address rather than an error, because a missing sender must
// not fail a whole sync.
func firstAddress(addrs []imap.Address) domain.EmailAddress {
	if len(addrs) == 0 {
		return domain.EmailAddress{}
	}
	a := addrs[0]
	email, err := domain.NewEmailAddress(mailparse.DecodeHeader(a.Name), a.Mailbox+"@"+a.Host)
	if err != nil {
		return domain.EmailAddress{}
	}
	return email
}

// allAddresses maps every parseable envelope address into domain addresses, skipping any that do not
// parse. Used for the To and Cc lists so reply-all can address the whole conversation.
func allAddresses(addrs []imap.Address) []domain.EmailAddress {
	out := make([]domain.EmailAddress, 0, len(addrs))
	for _, a := range addrs {
		email, err := domain.NewEmailAddress(mailparse.DecodeHeader(a.Name), a.Mailbox+"@"+a.Host)
		if err != nil {
			continue
		}
		out = append(out, email)
	}
	return out
}

// buildMessage maps a FETCH buffer into a domain message summary.
func buildMessage(folderID string, buf *imapclient.FetchMessageBuffer) (domain.MessageSummary, error) {
	uid := strconv.FormatUint(uint64(buf.UID), 10)
	in := domain.MessageSummaryInput{
		ID:       makeMessageID(folderID, uid),
		FolderID: folderID,
		UID:      uid,
		Size:     int(buf.RFC822Size),
		Flags:    mapFlags(buf.Flags),
	}
	in.HasAttachments = hasAttachment(buf.BodyStructure)
	if buf.Envelope != nil {
		in.Subject = mailparse.DecodeHeader(buf.Envelope.Subject)
		in.Date = buf.Envelope.Date
		in.MessageID = buf.Envelope.MessageID
		in.From = firstAddress(buf.Envelope.From)
		in.To = allAddresses(buf.Envelope.To)
		in.Cc = allAddresses(buf.Envelope.Cc)
	}
	return domain.NewMessageSummary(in)
}

// hasAttachment reports whether a message's body structure carries a saveable attachment, so the list can
// show the paperclip. A part counts when its content disposition is "attachment", matching what the body
// parser later extracts as a saveable file. A text/calendar part is excluded because the reader surfaces
// it as a meeting invite rather than an attachment. A nil structure (not fetched, or a bodyless message)
// yields false.
func hasAttachment(bs imap.BodyStructure) bool {
	if bs == nil {
		return false
	}
	found := false
	bs.Walk(func(_ []int, part imap.BodyStructure) bool {
		if found {
			return false
		}
		single, ok := part.(*imap.BodyStructureSinglePart)
		if !ok {
			return true // a multipart container: descend into its children
		}
		if strings.EqualFold(single.MediaType(), "text/calendar") {
			return false
		}
		if disp := single.Disposition(); disp != nil && strings.EqualFold(disp.Value, "attachment") {
			found = true
		}
		return false
	})
	return found
}

func hasAttr(attrs []imap.MailboxAttr, want imap.MailboxAttr) bool {
	for _, attr := range attrs {
		if attr == want {
			return true
		}
	}
	return false
}
