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

// specialRoleFor returns the well-known role a mailbox's RFC 6154 special-use attributes declare, and
// whether it declared one. INBOX is always the inbox. A server's declaration is authoritative, so a role
// it flags is never also given to a folder that merely shares a well-known name.
func specialRoleFor(mailbox string, attrs []imap.MailboxAttr) (domain.FolderKind, bool) {
	if strings.EqualFold(mailbox, "INBOX") {
		return domain.FolderInbox, true
	}
	for _, attr := range attrs {
		switch attr {
		case imap.MailboxAttrSent:
			return domain.FolderSent, true
		case imap.MailboxAttrDrafts:
			return domain.FolderDrafts, true
		case imap.MailboxAttrTrash:
			return domain.FolderTrash, true
		case imap.MailboxAttrJunk:
			return domain.FolderJunk, true
		case imap.MailboxAttrArchive:
			return domain.FolderArchive, true
		}
	}
	return domain.FolderCustom, false
}

// namedRoleFor returns the well-known role a mailbox's leaf name matches, and whether it matched one. It
// is the fallback for servers that do not advertise special-use.
func namedRoleFor(leaf string) (domain.FolderKind, bool) {
	kind, ok := folderKindByLeafName[strings.ToLower(strings.TrimSpace(leaf))]
	return kind, ok
}

// isNonInboxWellKnown reports whether a role is one of the special mailboxes that must not host a sibling
// role by name: a folder named "Sent" nested under Drafts (or Trash, Junk, Archive) is not the account's
// Sent. INBOX is excluded because servers legitimately nest the special folders under it.
func isNonInboxWellKnown(k domain.FolderKind) bool {
	switch k {
	case domain.FolderSent, domain.FolderDrafts, domain.FolderTrash, domain.FolderJunk, domain.FolderArchive:
		return true
	}
	return false
}

// buildFolders maps the selectable LIST responses into domain folders, giving each well-known role to
// exactly one folder. RFC 6154 special-use attributes are authoritative: a role the server flags is given
// only to the flagged folder, so other folders that merely share a well-known name stay Custom. A role the
// server does not flag falls back to the well-known leaf names; among several name matches the shallowest
// (then the first listed) wins, and a name match nested under a different well-known folder is rejected,
// so a stray "Sent" under Drafts never becomes the account Sent. Every other mailbox is Custom.
func buildFolders(accountID string, list []*imap.ListData) ([]domain.Folder, error) {
	type folderInfo struct {
		mailbox   string
		separator string
		special   domain.FolderKind
		hasSpec   bool
		named     domain.FolderKind
		hasNamed  bool
		depth     int
	}
	infos := make([]folderInfo, 0, len(list))
	// barrier holds the paths of folders that are a non-inbox well-known mailbox (by special use or by
	// name); a name match nested beneath one of these is rejected.
	barrier := map[string]bool{}
	for _, data := range list {
		separator := ""
		if data.Delim != 0 {
			separator = string(data.Delim)
		}
		leaf := data.Mailbox
		depth := 0
		if separator != "" {
			if idx := strings.LastIndex(data.Mailbox, separator); idx >= 0 {
				leaf = data.Mailbox[idx+len(separator):]
			}
			depth = strings.Count(data.Mailbox, separator)
		}
		special, hasSpec := specialRoleFor(data.Mailbox, data.Attrs)
		named, hasNamed := namedRoleFor(leaf)
		infos = append(infos, folderInfo{data.Mailbox, separator, special, hasSpec, named, hasNamed, depth})
		role := domain.FolderCustom
		if hasSpec {
			role = special
		} else if hasNamed {
			role = named
		}
		if isNonInboxWellKnown(role) {
			barrier[data.Mailbox] = true
		}
	}
	// underBarrier reports whether a folder sits inside a non-inbox well-known subtree.
	underBarrier := func(in folderInfo) bool {
		if in.separator == "" {
			return false
		}
		for path := in.mailbox; ; {
			idx := strings.LastIndex(path, in.separator)
			if idx < 0 {
				return false
			}
			path = path[:idx]
			if barrier[path] {
				return true
			}
		}
	}
	// winner maps a mailbox path to the non-inbox well-known role it wins.
	winner := map[string]domain.FolderKind{}
	roles := []domain.FolderKind{
		domain.FolderSent, domain.FolderDrafts, domain.FolderTrash, domain.FolderJunk, domain.FolderArchive,
	}
	for _, role := range roles {
		best := -1
		for i := range infos {
			if infos[i].hasSpec && infos[i].special == role {
				best = i
				break // a flagged role is authoritative; the first flagged folder wins.
			}
		}
		if best < 0 {
			for i := range infos {
				in := infos[i]
				if in.hasSpec || !in.hasNamed || in.named != role || underBarrier(in) {
					continue
				}
				if best < 0 || in.depth < infos[best].depth {
					best = i
				}
			}
		}
		if best >= 0 {
			winner[infos[best].mailbox] = role
		}
	}
	folders := make([]domain.Folder, 0, len(infos))
	for _, in := range infos {
		kind := domain.FolderCustom
		if in.hasSpec && in.special == domain.FolderInbox {
			kind = domain.FolderInbox
		} else if role, ok := winner[in.mailbox]; ok {
			kind = role
		}
		folder, err := domain.NewFolderWithSeparator(
			makeFolderID(accountID, in.mailbox), accountID, in.mailbox, in.separator, kind, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("build folder %q: %w", in.mailbox, err)
		}
		// Carry whether the server itself declared the role, so reconciliation respects the server's own
		// placement of a declared folder and only relocates one that was classified by name.
		folders = append(folders, folder.WithSpecialUse(in.hasSpec))
	}
	return folders, nil
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
