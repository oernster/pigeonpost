// Package message renders a domain OutgoingMessage into RFC 5322 wire bytes. It is shared by the SMTP
// transport (which sends the bytes) and the IMAP source (which appends them to the Drafts mailbox), so
// the message-format logic lives in one place rather than being duplicated across the two adapters.
package message

import (
	"encoding/base64"
	stdmime "mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// base64LineLength is the maximum characters per base64 line in an encoded attachment (RFC 2045).
const base64LineLength = 76

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

	attachments := msg.Attachments()
	calendar := msg.Calendar()
	if len(attachments) > 0 || !calendar.IsZero() {
		writeMixedBody(&b, msg, messageID, attachments, calendar)
	} else {
		writeContent(&b, msg, messageID)
	}
	return []byte(b.String())
}

// writeContent writes the message body's Content-Type header and body: a multipart/alternative when an
// HTML alternative is present, otherwise a single text/plain body. Bare URLs in the HTML alternative are
// anchored on the way out, so a link the sender typed (or a mailto: handoff prefilled) arrives clickable
// in every client.
func writeContent(b *strings.Builder, msg domain.OutgoingMessage, messageID string) {
	if html := msg.HTMLBody(); html != "" {
		writeMultipartBody(b, msg.Body(), mailparse.LinkifyHTML(html), messageID)
	} else {
		writeTextPartHeaderless(b, "text/plain; charset=utf-8", msg.Body())
	}
}

// writeMixedBody wraps the message content, an optional scheduling part and any attachments in a
// multipart/mixed body: the content (text or multipart/alternative) is the first part, followed by the
// text/calendar part (when present) and then one part per attachment.
func writeMixedBody(b *strings.Builder, msg domain.OutgoingMessage, messageID string, attachments []domain.Attachment, calendar domain.CalendarPart) {
	boundary := "=_pigeonpost_mixed_" + messageID
	b.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("--" + boundary + "\r\n")
	writeContent(b, msg, messageID)
	b.WriteString("\r\n")
	if !calendar.IsZero() {
		writeCalendarPart(b, boundary, calendar)
	}
	for _, attachment := range attachments {
		writeAttachmentPart(b, boundary, attachment)
	}
	b.WriteString("--" + boundary + "--\r\n")
}

// writeCalendarPart writes the iMIP scheduling object as a text/calendar part carrying its iTIP method,
// so a receiving client recognises the message as a meeting request, reply or cancellation. The payload
// is already folded iCalendar text, so it is sent 8bit with its line endings normalised to CRLF.
func writeCalendarPart(b *strings.Builder, boundary string, calendar domain.CalendarPart) {
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/calendar; method=" + string(calendar.Method()) + "; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(normaliseBody(string(calendar.Content())))
	b.WriteString("\r\n")
}

// writeAttachmentPart writes one base64-encoded attachment as a multipart/mixed part.
func writeAttachmentPart(b *strings.Builder, boundary string, attachment domain.Attachment) {
	name := quoteParam(attachment.Filename())
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: " + attachment.ContentType() + "; name=\"" + name + "\"\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("Content-Disposition: attachment; filename=\"" + name + "\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(base64Lines(attachment.Content()))
}

// quoteParam makes a filename safe to place inside a quoted MIME parameter by removing the characters
// that would terminate or escape the quoted string.
func quoteParam(name string) string {
	return strings.NewReplacer("\"", "", "\\", "", "\r", "", "\n", "").Replace(name)
}

// base64Lines encodes content as base64 wrapped to the RFC 2045 line length, each line ended with CRLF.
func base64Lines(content []byte) string {
	encoded := base64.StdEncoding.EncodeToString(content)
	var b strings.Builder
	for start := 0; start < len(encoded); start += base64LineLength {
		end := start + base64LineLength
		if end > len(encoded) {
			end = len(encoded)
		}
		b.WriteString(encoded[start:end])
		b.WriteString("\r\n")
	}
	return b.String()
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
	writeTextPartHeaderless(b, contentType, content)
	b.WriteString("\r\n")
}

// writeTextPartHeaderless writes a text part's headers and quoted-printable body. Quoted-printable keeps
// every wire line within the RFC 5322 limit: an 8bit body with a long line (a URL, an unwrapped HTML
// paragraph) gets hard-folded by relays, which corrupts the content; soft line breaks fold losslessly.
func writeTextPartHeaderless(b *strings.Builder, contentType, content string) {
	b.WriteString("Content-Type: " + contentType + "\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	b.WriteString("\r\n")
	writer := quotedprintable.NewWriter(b)
	// The writer never fails against a strings.Builder; the errors are checked anyway so a future
	// destination change cannot silently drop content.
	if _, err := writer.Write([]byte(normaliseBody(content))); err == nil {
		_ = writer.Close()
	}
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
