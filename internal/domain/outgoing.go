package domain

import "strings"

// OutgoingMessage is a validated message ready to be handed to a transport for sending. It is
// immutable once constructed.
type OutgoingMessage struct {
	from     EmailAddress
	to       []EmailAddress
	cc       []EmailAddress
	subject  string
	body     string
	htmlBody string
}

// OutgoingMessageInput carries the fields needed to build an OutgoingMessage. Body is the plain-text
// content and is always present; HTMLBody is optional and, when set, is sent as the rich alternative.
type OutgoingMessageInput struct {
	From     EmailAddress
	To       []EmailAddress
	Cc       []EmailAddress
	Subject  string
	Body     string
	HTMLBody string
}

// NewOutgoingMessage validates and constructs a message. It requires a sender and at least one
// valid recipient; any zero address in the recipient lists is rejected.
func NewOutgoingMessage(in OutgoingMessageInput) (OutgoingMessage, error) {
	if in.From.IsZero() {
		return OutgoingMessage{}, ErrNoSender
	}
	to, err := cleanRecipients(in.To)
	if err != nil {
		return OutgoingMessage{}, err
	}
	if len(to) == 0 {
		return OutgoingMessage{}, ErrNoRecipients
	}
	cc, err := cleanRecipients(in.Cc)
	if err != nil {
		return OutgoingMessage{}, err
	}
	return OutgoingMessage{
		from:     in.From,
		to:       to,
		cc:       cc,
		subject:  strings.TrimSpace(in.Subject),
		body:     in.Body,
		htmlBody: in.HTMLBody,
	}, nil
}

func cleanRecipients(addrs []EmailAddress) ([]EmailAddress, error) {
	out := make([]EmailAddress, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IsZero() {
			return nil, ErrNoRecipients
		}
		out = append(out, addr)
	}
	return out, nil
}

// From returns the sender.
func (m OutgoingMessage) From() EmailAddress { return m.from }

// To returns a copy of the primary recipients.
func (m OutgoingMessage) To() []EmailAddress { return append([]EmailAddress(nil), m.to...) }

// Cc returns a copy of the carbon-copy recipients.
func (m OutgoingMessage) Cc() []EmailAddress { return append([]EmailAddress(nil), m.cc...) }

// Subject returns the subject line.
func (m OutgoingMessage) Subject() string { return m.subject }

// Body returns the plain-text body.
func (m OutgoingMessage) Body() string { return m.body }

// HTMLBody returns the optional rich-text (HTML) body. It is empty for a plain-text-only message.
func (m OutgoingMessage) HTMLBody() string { return m.htmlBody }

// Recipients returns every address the message must be delivered to (To plus Cc).
func (m OutgoingMessage) Recipients() []EmailAddress {
	out := make([]EmailAddress, 0, len(m.to)+len(m.cc))
	out = append(out, m.to...)
	out = append(out, m.cc...)
	return out
}
