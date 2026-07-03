// Package message renders a domain OutgoingMessage into RFC 5322 wire bytes. It is shared by the SMTP
// transport (which sends the bytes) and the IMAP source (which appends them to the Drafts mailbox), so
// the message-format logic lives in one place rather than being duplicated across the two adapters.
package message

import (
	stdmime "mime"
	"net/mail"
	"strings"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// BuildMIME renders an outgoing message into RFC 5322 wire bytes. The date and message id are passed
// in (rather than read from the clock here) so the output is deterministic and testable. When the
// message carries an HTML body, a multipart/alternative body is emitted with the plain text first and
// the HTML second (the boundary is derived from the message id so it stays deterministic and unique).
func BuildMIME(msg domain.OutgoingMessage, date time.Time, messageID string) []byte {
	var b strings.Builder
	writeAddressHeader(&b, "From", []domain.EmailAddress{msg.From()})
	writeAddressHeader(&b, "To", msg.To())
	if cc := msg.Cc(); len(cc) > 0 {
		writeAddressHeader(&b, "Cc", cc)
	}
	b.WriteString("Subject: " + stdmime.QEncoding.Encode("utf-8", msg.Subject()) + "\r\n")
	b.WriteString("Date: " + date.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + messageID + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	if html := msg.HTMLBody(); html != "" {
		writeMultipartBody(&b, msg.Body(), html, messageID)
	} else {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(normaliseBody(msg.Body()))
	}
	return []byte(b.String())
}

// writeMultipartBody writes a multipart/alternative body carrying the plain-text and HTML variants.
func writeMultipartBody(b *strings.Builder, plain, html, messageID string) {
	boundary := "=_pigeonpost_" + messageID
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")
	writeMIMEPart(b, boundary, "text/plain; charset=utf-8", plain)
	writeMIMEPart(b, boundary, "text/html; charset=utf-8", html)
	b.WriteString("--" + boundary + "--\r\n")
}

// writeMIMEPart writes one body part with the given content type and CRLF-normalised content.
func writeMIMEPart(b *strings.Builder, boundary, contentType, content string) {
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: " + contentType + "\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(normaliseBody(content))
	b.WriteString("\r\n")
}

func writeAddressHeader(b *strings.Builder, name string, addrs []domain.EmailAddress) {
	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		formatted := (&mail.Address{Name: addr.Display(), Address: addr.Address()}).String()
		parts = append(parts, formatted)
	}
	b.WriteString(name + ": " + strings.Join(parts, ", ") + "\r\n")
}

// normaliseBody converts any line endings to CRLF for the wire.
func normaliseBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}
