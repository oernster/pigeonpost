package domain

import "strings"

// OutgoingMessage is a validated message ready to be handed to a transport for sending. It is
// immutable once constructed.
type OutgoingMessage struct {
	from        EmailAddress
	to          []EmailAddress
	cc          []EmailAddress
	bcc         []EmailAddress
	subject     string
	body        string
	htmlBody    string
	attachments []Attachment
}

// OutgoingMessageInput carries the fields needed to build an OutgoingMessage. Body is the plain-text
// content and is always present; HTMLBody is optional and, when set, is sent as the rich alternative.
type OutgoingMessageInput struct {
	From        EmailAddress
	To          []EmailAddress
	Cc          []EmailAddress
	Bcc         []EmailAddress
	Subject     string
	Body        string
	HTMLBody    string
	Attachments []Attachment
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
	bcc, err := cleanRecipients(in.Bcc)
	if err != nil {
		return OutgoingMessage{}, err
	}
	return OutgoingMessage{
		from:        in.From,
		to:          to,
		cc:          cc,
		bcc:         bcc,
		subject:     strings.TrimSpace(in.Subject),
		body:        in.Body,
		htmlBody:    in.HTMLBody,
		attachments: append([]Attachment(nil), in.Attachments...),
	}, nil
}

// NewDraftMessage validates and constructs a message for saving as a draft. Unlike a message bound for
// sending, a draft is allowed to be incomplete: it may have no recipients and an empty body, because
// the user is still composing it. A sender is still required (it identifies the drafting account) and
// any recipient that is present must be a valid, non-zero address.
func NewDraftMessage(in OutgoingMessageInput) (OutgoingMessage, error) {
	if in.From.IsZero() {
		return OutgoingMessage{}, ErrNoSender
	}
	to, err := cleanRecipients(in.To)
	if err != nil {
		return OutgoingMessage{}, err
	}
	cc, err := cleanRecipients(in.Cc)
	if err != nil {
		return OutgoingMessage{}, err
	}
	bcc, err := cleanRecipients(in.Bcc)
	if err != nil {
		return OutgoingMessage{}, err
	}
	return OutgoingMessage{
		from:        in.From,
		to:          to,
		cc:          cc,
		bcc:         bcc,
		subject:     strings.TrimSpace(in.Subject),
		body:        in.Body,
		htmlBody:    in.HTMLBody,
		attachments: append([]Attachment(nil), in.Attachments...),
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

// Bcc returns a copy of the blind-carbon-copy recipients. These are delivered to (they appear in
// Recipients) but never written to the message headers, so other recipients cannot see them.
func (m OutgoingMessage) Bcc() []EmailAddress { return append([]EmailAddress(nil), m.bcc...) }

// Attachments returns a copy of the files carried by the message.
func (m OutgoingMessage) Attachments() []Attachment {
	return append([]Attachment(nil), m.attachments...)
}

// Subject returns the subject line.
func (m OutgoingMessage) Subject() string { return m.subject }

// Body returns the plain-text body.
func (m OutgoingMessage) Body() string { return m.body }

// HTMLBody returns the optional rich-text (HTML) body. It is empty for a plain-text-only message.
func (m OutgoingMessage) HTMLBody() string { return m.htmlBody }

// Recipients returns every distinct address the message must be delivered to (To plus Cc plus Bcc),
// compared case-insensitively. An address listed in more than one of those yields a single envelope
// recipient, so the mailbox is delivered one copy rather than the transport issuing a duplicate RCPT
// for it. To ordering is kept first, then Cc, then any Bcc addresses not already present.
func (m OutgoingMessage) Recipients() []EmailAddress {
	total := len(m.to) + len(m.cc) + len(m.bcc)
	out := make([]EmailAddress, 0, total)
	seen := make(map[string]struct{}, total)
	add := func(addr EmailAddress) {
		key := strings.ToLower(addr.Address())
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, addr)
	}
	for _, addr := range m.to {
		add(addr)
	}
	for _, addr := range m.cc {
		add(addr)
	}
	for _, addr := range m.bcc {
		add(addr)
	}
	return out
}
