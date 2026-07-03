package domain

import (
	"strings"
	"time"
)

// Flag is a single IMAP-style system flag, held as a bit within Flags.
type Flag uint8

const (
	FlagSeen Flag = 1 << iota
	FlagAnswered
	FlagFlagged
	FlagDraft
	FlagDeleted
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

// Tag is a user-defined, coloured label that can be attached to messages.
type Tag struct {
	id     string
	name   string
	colour Colour
}

// NewTag validates and constructs a tag.
func NewTag(id, name string, colour Colour) (Tag, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Tag{}, ErrEmptyTagID
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Tag{}, ErrEmptyTagName
	}
	return Tag{id: id, name: name, colour: colour}, nil
}

// ID returns the tag identifier.
func (t Tag) ID() string { return t.id }

// Name returns the tag name.
func (t Tag) Name() string { return t.name }

// Colour returns the tag colour.
func (t Tag) Colour() Colour { return t.colour }

// MessageSummary is the header-level view of a message shown in the message list. Bodies and
// attachments are loaded separately and lazily.
type MessageSummary struct {
	id             string
	folderID       string
	uid            uint32
	messageID      string
	from           EmailAddress
	subject        string
	date           time.Time
	size           int
	flags          Flags
	hasAttachments bool
	snippet        string
}

// MessageSummaryInput carries the fields needed to build a MessageSummary. It keeps the constructor
// signature readable given the number of fields.
type MessageSummaryInput struct {
	ID             string
	FolderID       string
	UID            uint32
	MessageID      string
	From           EmailAddress
	Subject        string
	Date           time.Time
	Size           int
	Flags          Flags
	HasAttachments bool
	Snippet        string
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
	if in.UID == 0 {
		return MessageSummary{}, ErrInvalidUID
	}
	if in.Size < 0 {
		return MessageSummary{}, ErrNegativeSize
	}
	return MessageSummary{
		id:             id,
		folderID:       folderID,
		uid:            in.UID,
		messageID:      strings.TrimSpace(in.MessageID),
		from:           in.From,
		subject:        in.Subject,
		date:           in.Date,
		size:           in.Size,
		flags:          in.Flags,
		hasAttachments: in.HasAttachments,
		snippet:        in.Snippet,
	}, nil
}

// ID returns the local identifier.
func (m MessageSummary) ID() string { return m.id }

// FolderID returns the owning folder identifier.
func (m MessageSummary) FolderID() string { return m.folderID }

// UID returns the server-assigned IMAP UID.
func (m MessageSummary) UID() uint32 { return m.uid }

// MessageID returns the RFC Message-ID header value.
func (m MessageSummary) MessageID() string { return m.messageID }

// From returns the sender address.
func (m MessageSummary) From() EmailAddress { return m.from }

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

// IsRead reports whether the message has been read.
func (m MessageSummary) IsRead() bool { return m.flags.IsSeen() }

// IsFlagged reports whether the message is flagged (starred / important).
func (m MessageSummary) IsFlagged() bool { return m.flags.Has(FlagFlagged) }

// WithFlags returns a copy carrying a new flag set.
func (m MessageSummary) WithFlags(flags Flags) MessageSummary {
	copied := m
	copied.flags = flags
	return copied
}
