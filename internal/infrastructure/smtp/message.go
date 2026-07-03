// Package smtp implements the application MailTransport interface using emersion/go-smtp. The RFC
// 5322 message construction lives in this file so it can be unit-tested without a network.
package smtp

import (
	"mime"
	"net/mail"
	"strings"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// BuildMIME renders an outgoing message into RFC 5322 wire bytes. The date and message id are passed
// in (rather than read from the clock here) so the output is deterministic and testable.
func BuildMIME(msg domain.OutgoingMessage, date time.Time, messageID string) []byte {
	var b strings.Builder
	writeAddressHeader(&b, "From", []domain.EmailAddress{msg.From()})
	writeAddressHeader(&b, "To", msg.To())
	if cc := msg.Cc(); len(cc) > 0 {
		writeAddressHeader(&b, "Cc", cc)
	}
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject()) + "\r\n")
	b.WriteString("Date: " + date.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + messageID + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(normaliseBody(msg.Body()))
	return []byte(b.String())
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
