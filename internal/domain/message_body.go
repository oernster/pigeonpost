package domain

import "strings"

// MessageBody is the full content of a message: the plain-text body and, when the message provided
// one, the original HTML. It also carries the raw text/calendar payload (an iMIP scheduling object such
// as a meeting invite) when the message contained one, so a message reads offline after its first fetch
// and the invite renders without a re-fetch. It is immutable once constructed.
type MessageBody struct {
	messageID   string
	plain       string
	html        string
	invite      []byte
	attachments []Attachment
}

// NewMessageBody validates and constructs a message body. Only the message id is required; a message
// may legitimately have an empty body. Use WithInvite to attach a scheduling payload.
func NewMessageBody(messageID, plain, html string) (MessageBody, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return MessageBody{}, ErrEmptyMessageID
	}
	return MessageBody{messageID: messageID, plain: plain, html: html}, nil
}

// MessageID returns the identifier of the message this body belongs to.
func (b MessageBody) MessageID() string { return b.messageID }

// Plain returns the plain-text body.
func (b MessageBody) Plain() string { return b.plain }

// HTML returns the original HTML body, or an empty string when the message had none.
func (b MessageBody) HTML() string { return b.html }

// Invite returns a copy of the raw text/calendar payload the message carried, or nil when it carried
// none, so callers cannot mutate the body.
func (b MessageBody) Invite() []byte { return append([]byte(nil), b.invite...) }

// HasInvite reports whether the message carried a text/calendar payload, so the reader should offer the
// scheduling actions.
func (b MessageBody) HasInvite() bool { return len(b.invite) > 0 }

// WithInvite returns a copy of the body with its scheduling payload replaced. The bytes are copied so
// neither the receiver nor the caller's slice is shared. The body stays immutable: the receiver is
// unchanged.
func (b MessageBody) WithInvite(invite []byte) MessageBody {
	b.invite = append([]byte(nil), invite...)
	return b
}

// Attachments returns a copy of the files the message carried, so the reader can list them and the user
// can save them. The slice is copied so callers cannot mutate the body.
func (b MessageBody) Attachments() []Attachment {
	return append([]Attachment(nil), b.attachments...)
}

// HasAttachments reports whether the message carried any files.
func (b MessageBody) HasAttachments() bool { return len(b.attachments) > 0 }

// WithAttachments returns a copy of the body carrying the given attachments. The slice is copied so
// neither the receiver nor the caller's slice is shared, keeping the body immutable.
func (b MessageBody) WithAttachments(attachments []Attachment) MessageBody {
	b.attachments = append([]Attachment(nil), attachments...)
	return b
}
