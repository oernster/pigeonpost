// Package imap implements the application MailSource interface against a live IMAP server using
// emersion/go-imap v2. The pure mapping between IMAP wire types and domain types lives in this file
// so it can be unit-tested without a network connection.
package imap

import (
	"fmt"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

// folderIDSeparator joins account, mailbox and uid into stable local identifiers. It is a control
// character that does not appear in mailbox names or email addresses.
const folderIDSeparator = "\x1f"

func makeFolderID(accountID, mailbox string) string {
	return accountID + folderIDSeparator + mailbox
}

func makeMessageID(folderID string, uid uint32) string {
	return fmt.Sprintf("%s%s%d", folderID, folderIDSeparator, uid)
}

// folderKindFor classifies a mailbox from its name and RFC 6154 special-use attributes.
func folderKindFor(mailbox string, attrs []imap.MailboxAttr) domain.FolderKind {
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
	return domain.FolderCustom
}

// buildFolder maps a LIST response into a domain folder. Counts are unknown from LIST alone and
// default to zero until a later STATUS/SELECT sync fills them in.
func buildFolder(accountID string, data *imap.ListData) (domain.Folder, error) {
	kind := folderKindFor(data.Mailbox, data.Attrs)
	return domain.NewFolder(makeFolderID(accountID, data.Mailbox), accountID, data.Mailbox, kind, 0, 0)
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
	email, err := domain.NewEmailAddress(a.Name, a.Mailbox+"@"+a.Host)
	if err != nil {
		return domain.EmailAddress{}
	}
	return email
}

// buildMessage maps a FETCH buffer into a domain message summary.
func buildMessage(folderID string, buf *imapclient.FetchMessageBuffer) (domain.MessageSummary, error) {
	uid := uint32(buf.UID)
	in := domain.MessageSummaryInput{
		ID:       makeMessageID(folderID, uid),
		FolderID: folderID,
		UID:      uid,
		Size:     int(buf.RFC822Size),
		Flags:    mapFlags(buf.Flags),
	}
	if buf.Envelope != nil {
		in.Subject = buf.Envelope.Subject
		in.Date = buf.Envelope.Date
		in.MessageID = buf.Envelope.MessageID
		in.From = firstAddress(buf.Envelope.From)
	}
	return domain.NewMessageSummary(in)
}

func hasAttr(attrs []imap.MailboxAttr, want imap.MailboxAttr) bool {
	for _, attr := range attrs {
		if attr == want {
			return true
		}
	}
	return false
}
