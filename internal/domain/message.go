package domain

import (
	"encoding/hex"
	"strings"
	"time"
)

// Flag is a single IMAP-style flag held as a bit within Flags: an RFC 3501 system flag or the $Forwarded
// keyword.
type Flag uint8

const (
	FlagSeen Flag = 1 << iota
	FlagAnswered
	FlagFlagged
	FlagDraft
	FlagDeleted
	// FlagForwarded records that the message has been forwarded. Unlike the others it is not an RFC 3501
	// system flag but the widely used $Forwarded keyword (RFC 5788); it is modelled as a bit here so it rides
	// the same Flags value, persistence and IMAP mapping as the system flags.
	FlagForwarded
)

// Flags is an immutable set of system flags on a message.
type Flags struct {
	bits Flag
}

// NewFlags constructs a flag set from a raw bitmask.
func NewFlags(bits Flag) Flags { return Flags{bits: bits} }

// Has reports whether the given flag is set.
func (f Flags) Has(flag Flag) bool { return f.bits&flag != 0 }

// With returns a copy with the given flag set.
func (f Flags) With(flag Flag) Flags { return Flags{bits: f.bits | flag} }

// Without returns a copy with the given flag cleared.
func (f Flags) Without(flag Flag) Flags { return Flags{bits: f.bits &^ flag} }

// IsSeen reports whether the message has been read.
func (f Flags) IsSeen() bool { return f.Has(FlagSeen) }

// Raw returns the underlying bitmask, for persistence.
func (f Flags) Raw() Flag { return f.bits }

// Tag is a user-defined, coloured label that can be attached to messages. Its keyword is the stable IMAP
// keyword it round-trips as on the server; it is assigned once at creation (see KeywordForName) and frozen,
// so a later rename never changes it.
type Tag struct {
	id      string
	name    string
	colour  Colour
	keyword string
}

// NewTag validates and constructs a tag. The keyword is stored verbatim (frozen); callers derive it once
// from the name via KeywordForName when the tag is first created and preserve it across renames.
func NewTag(id, name string, colour Colour, keyword string) (Tag, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Tag{}, ErrEmptyTagID
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Tag{}, ErrEmptyTagName
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return Tag{}, ErrEmptyTagKeyword
	}
	return Tag{id: id, name: name, colour: colour, keyword: keyword}, nil
}

// ID returns the tag identifier.
func (t Tag) ID() string { return t.id }

// Name returns the tag name.
func (t Tag) Name() string { return t.name }

// Colour returns the tag colour.
func (t Tag) Colour() Colour { return t.colour }

// tagKeywordPrefix namespaces PigeonPost's tag keywords on the server so they are told apart from IMAP
// system flags and other clients' keywords when a message's flags are read back.
const tagKeywordPrefix = "$PPtag_"

// Keyword returns the tag's stable IMAP keyword (frozen at creation), the label it round-trips as on the
// server.
func (t Tag) Keyword() string { return t.keyword }

// KeywordForName derives the IMAP keyword for a tag of the given name. It is used ONCE, when a tag is
// created; the result is then stored on the tag and frozen, so a later rename does not change it (which
// would orphan the tag's server-side assignments and make a reconcile strip them). It is name-derived so a
// tag with the same name on another device maps to the same keyword and assignments therefore sync across
// devices; matching is case-sensitive (same-case names share a keyword). The trimmed name is hex-encoded,
// so the keyword is always a valid IMAP atom whatever characters (spaces, punctuation, non-ASCII) the
// display name carries, and it matches the migration backfill's lower(hex(name)) byte for byte for every
// name, unlike a Unicode-aware lower-casing which SQLite's ASCII-only lower() cannot reproduce.
func KeywordForName(name string) string {
	return tagKeywordPrefix + hex.EncodeToString([]byte(strings.TrimSpace(name)))
}

// IsTagKeyword reports whether an IMAP keyword is one of PigeonPost's tag keywords, as opposed to a system
// flag or a keyword owned by another client, so incoming server keywords can be filtered to ours.
func IsTagKeyword(keyword string) bool {
	return strings.HasPrefix(keyword, tagKeywordPrefix)
}

// PendingTagOp is a tag assignment or removal recorded locally but not yet confirmed on the server, so it
// can be replayed on a later sync and can guard the local state during a reconcile until the server agrees.
// Assigned is true for a pending assignment and false for a pending removal of the (MessageID, TagID) pair.
type PendingTagOp struct {
	messageID string
	tagID     string
	assigned  bool
}

// NewPendingTagOp constructs a pending tag operation.
func NewPendingTagOp(messageID, tagID string, assigned bool) PendingTagOp {
	return PendingTagOp{messageID: messageID, tagID: tagID, assigned: assigned}
}

// MessageID returns the message the operation targets.
func (p PendingTagOp) MessageID() string { return p.messageID }

// TagID returns the tag the operation targets.
func (p PendingTagOp) TagID() string { return p.tagID }

// Assigned reports whether the pending intent is to assign (true) or to remove (false) the tag.
func (p PendingTagOp) Assigned() bool { return p.assigned }

// MessageSummary is the header-level view of a message shown in the message list. Bodies and
// attachments are loaded separately and lazily.
type MessageSummary struct {
	id             string
	folderID       string
	uid            string
	messageID      string
	from           EmailAddress
	to             []EmailAddress
	cc             []EmailAddress
	subject        string
	date           time.Time
	size           int
	flags          Flags
	hasAttachments bool
	snippet        string
	keywords       []string
}

// MessageSummaryInput carries the fields needed to build a MessageSummary. It keeps the constructor
// signature readable given the number of fields.
type MessageSummaryInput struct {
	ID             string
	FolderID       string
	UID            string
	MessageID      string
	From           EmailAddress
	To             []EmailAddress
	Cc             []EmailAddress
	Subject        string
	Date           time.Time
	Size           int
	Flags          Flags
	HasAttachments bool
	Snippet        string
	// Keywords are the PigeonPost tag keywords the server reported on the message (see IsTagKeyword). They
	// are carried from a fetch so the tag reconcile can align local tag assignments with the server; they
	// are not persisted in the local cache.
	Keywords []string
}

// NewMessageSummary validates and constructs a message summary.
func NewMessageSummary(in MessageSummaryInput) (MessageSummary, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return MessageSummary{}, ErrEmptyMessageID
	}
	folderID := strings.TrimSpace(in.FolderID)
	if folderID == "" {
		return MessageSummary{}, ErrEmptyFolderID
	}
	uid := strings.TrimSpace(in.UID)
	if uid == "" {
		return MessageSummary{}, ErrInvalidUID
	}
	if in.Size < 0 {
		return MessageSummary{}, ErrNegativeSize
	}
	return MessageSummary{
		id:             id,
		folderID:       folderID,
		uid:            uid,
		messageID:      strings.TrimSpace(in.MessageID),
		from:           in.From,
		to:             append([]EmailAddress(nil), in.To...),
		cc:             append([]EmailAddress(nil), in.Cc...),
		subject:        in.Subject,
		date:           in.Date,
		size:           in.Size,
		flags:          in.Flags,
		hasAttachments: in.HasAttachments,
		snippet:        in.Snippet,
		keywords:       append([]string(nil), in.Keywords...),
	}, nil
}

// ID returns the local identifier.
func (m MessageSummary) ID() string { return m.id }

// FolderID returns the owning folder identifier.
func (m MessageSummary) FolderID() string { return m.folderID }

// UID returns the opaque server handle for the message: an IMAP UID held as a decimal string, or a
// POP3 UIDL. It is used to fetch the body and to target server-side actions; the UI never sees it.
func (m MessageSummary) UID() string { return m.uid }

// MessageID returns the RFC Message-ID header value.
func (m MessageSummary) MessageID() string { return m.messageID }

// From returns the sender address.
func (m MessageSummary) From() EmailAddress { return m.from }

// To returns a copy of the primary recipients (the To header), used by reply-all.
func (m MessageSummary) To() []EmailAddress { return append([]EmailAddress(nil), m.to...) }

// Cc returns a copy of the carbon-copy recipients (the Cc header), used by reply-all.
func (m MessageSummary) Cc() []EmailAddress { return append([]EmailAddress(nil), m.cc...) }

// Subject returns the subject line.
func (m MessageSummary) Subject() string { return m.subject }

// Date returns the message date.
func (m MessageSummary) Date() time.Time { return m.date }

// Size returns the message size in bytes.
func (m MessageSummary) Size() int { return m.size }

// Flags returns the system flags.
func (m MessageSummary) Flags() Flags { return m.flags }

// HasAttachments reports whether the message carries attachments.
func (m MessageSummary) HasAttachments() bool { return m.hasAttachments }

// Snippet returns a short plaintext preview.
func (m MessageSummary) Snippet() string { return m.snippet }

// Keywords returns a copy of the PigeonPost tag keywords the server reported on the message (see
// IsTagKeyword), used by the tag reconcile to align local tag assignments with the server. It is empty
// unless the summary came from a fresh fetch that captured them.
func (m MessageSummary) Keywords() []string { return append([]string(nil), m.keywords...) }

// IsRead reports whether the message has been read.
func (m MessageSummary) IsRead() bool { return m.flags.IsSeen() }

// IsFlagged reports whether the message is flagged (starred / important).
func (m MessageSummary) IsFlagged() bool { return m.flags.Has(FlagFlagged) }

// IsAnswered reports whether the message has been replied to (the \Answered system flag).
func (m MessageSummary) IsAnswered() bool { return m.flags.Has(FlagAnswered) }

// IsForwarded reports whether the message has been forwarded (the $Forwarded keyword).
func (m MessageSummary) IsForwarded() bool { return m.flags.Has(FlagForwarded) }

// WithFlags returns a copy carrying a new flag set.
func (m MessageSummary) WithFlags(flags Flags) MessageSummary {
	copied := m
	copied.flags = flags
	return copied
}
