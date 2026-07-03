package domain

import "strings"

// MessageBody is the full content of a message: the plain-text body and, when the message provided
// one, the original HTML. It is cached locally so a message reads offline after its first fetch.
type MessageBody struct {
	messageID string
	plain     string
	html      string
}

// NewMessageBody validates and constructs a message body. Only the message id is required; a message
// may legitimately have an empty body.
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
