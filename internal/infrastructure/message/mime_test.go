package message

import (
	"encoding/base64"
	"io"
	"mime/quotedprintable"
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func addr(t *testing.T, display, address string) domain.EmailAddress {
	t.Helper()
	a, err := domain.NewEmailAddress(display, address)
	if err != nil {
		t.Fatalf("address %q: %v", address, err)
	}
	return a
}

func TestBuildMIME(t *testing.T) {
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:    addr(t, "Me", "me@example.com"),
		To:      []domain.EmailAddress{addr(t, "", "a@example.com")},
		Cc:      []domain.EmailAddress{addr(t, "", "c@example.com")},
		Subject: "Hi there",
		Body:    "line1\nline2",
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	date := time.Date(2026, time.July, 3, 10, 0, 0, 0, time.UTC)
	out := string(BuildMIME(msg, date, "abc123@pigeonpost"))

	wants := []string{
		`From: "Me" <me@example.com>` + "\r\n",
		"To: <a@example.com>\r\n",
		"Cc: <c@example.com>\r\n",
		"Subject: Hi there\r\n",
		"Message-ID: <abc123@pigeonpost>\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n",
		"\r\nline1\r\nline2",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n---\n%s", w, out)
		}
	}
	if !strings.Contains(out, "\r\n\r\n") {
		t.Error("missing header/body separator")
	}
}

func TestBuildMIMEEmitsCalendarPart(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nMETHOD:REPLY\r\nBEGIN:VEVENT\r\nUID:m1\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	part, err := domain.NewCalendarPart(domain.MethodReply, []byte(ics))
	if err != nil {
		t.Fatalf("calendar part: %v", err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:     addr(t, "", "me@example.com"),
		To:       []domain.EmailAddress{addr(t, "", "chair@example.com")},
		Subject:  "Re: Sync",
		Body:     "Accepted.",
		Calendar: part,
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	date := time.Date(2026, time.July, 3, 10, 0, 0, 0, time.UTC)
	out := string(BuildMIME(msg, date, "abc123@pigeonpost"))

	wants := []string{
		"Content-Type: multipart/mixed;",
		"Content-Type: text/calendar; method=REPLY; charset=utf-8\r\n",
		"Content-Transfer-Encoding: 8bit\r\n",
		"METHOD:REPLY\r\n",
		"Accepted.",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n---\n%s", w, out)
		}
	}
	// The calendar payload must not be base64-encoded (it is an 8bit part), so the raw METHOD line shows.
	if strings.Contains(out, base64.StdEncoding.EncodeToString([]byte(ics))) {
		t.Errorf("calendar part should be 8bit, not base64\n---\n%s", out)
	}
}

func TestBuildMIMEMultipartAlternative(t *testing.T) {
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:     addr(t, "", "me@example.com"),
		To:       []domain.EmailAddress{addr(t, "", "a@example.com")},
		Subject:  "Rich",
		Body:     "plain text",
		HTMLBody: "<p>rich <b>text</b></p>",
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	out := string(BuildMIME(msg, time.Unix(0, 0).UTC(), "mid42"))

	wants := []string{
		`Content-Type: multipart/alternative; boundary="=_pigeonpost_mid42"` + "\r\n",
		"--=_pigeonpost_mid42\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n",
		"\r\nplain text\r\n",
		"Content-Type: text/html; charset=utf-8\r\n",
		"<p>rich <b>text</b></p>",
		"--=_pigeonpost_mid42--\r\n",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("multipart output missing %q\n---\n%s", w, out)
		}
	}
	// The plain part must come before the HTML part (least-to-most rich per RFC 2046).
	if strings.Index(out, "text/plain") > strings.Index(out, "text/html") {
		t.Error("text/plain part must precede text/html part")
	}
}

func TestBuildMIMEAnchorsBareURLsInHTML(t *testing.T) {
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:     addr(t, "", "me@example.com"),
		To:       []domain.EmailAddress{addr(t, "", "a@example.com")},
		Subject:  "Link",
		Body:     "see https://example.org/page",
		HTMLBody: "<p>see https://example.org/page and www.example.org.</p>",
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	out := string(BuildMIME(msg, time.Unix(0, 0).UTC(), "mid7"))
	decoded := decodeQuotedPrintableParts(t, out)
	wants := []string{
		`<a href="https://example.org/page">https://example.org/page</a>`,
		`<a href="http://www.example.org">www.example.org</a>`,
	}
	for _, w := range wants {
		if !strings.Contains(decoded, w) {
			t.Errorf("outgoing HTML missing %q\n---\n%s", w, decoded)
		}
	}
	// The existing anchor policy: the trailing sentence dot stays outside the link text.
	if strings.Contains(decoded, `www.example.org.</a>`) {
		t.Errorf("trailing punctuation leaked into the link\n---\n%s", decoded)
	}
}

func TestBuildMIMEKeepsWireLinesWithinLimit(t *testing.T) {
	long := "https://example.org/" + strings.Repeat("segment/", 40) + "end"
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:     addr(t, "", "me@example.com"),
		To:       []domain.EmailAddress{addr(t, "", "a@example.com")},
		Subject:  "Long line",
		Body:     "click " + long,
		HTMLBody: "<p>click " + long + "</p>",
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	out := string(BuildMIME(msg, time.Unix(0, 0).UTC(), "mid8"))
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 78 {
			t.Errorf("wire line exceeds the safe length (%d): %q", len(line), line)
		}
	}
	// The folded content must decode back to the intact URL.
	if !strings.Contains(decodeQuotedPrintableParts(t, out), long) {
		t.Errorf("long URL did not survive the quoted-printable round trip\n---\n%s", out)
	}
}

// decodeQuotedPrintableParts decodes every quoted-printable body section of a built message, so tests
// assert on the content a receiving client reconstructs rather than on the folded wire form.
func decodeQuotedPrintableParts(t *testing.T, wire string) string {
	t.Helper()
	var decoded strings.Builder
	sections := strings.Split(wire, "Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	for _, section := range sections[1:] {
		body, _, _ := strings.Cut(section, "\r\n--")
		content, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
		if err != nil {
			t.Fatalf("decode quoted-printable: %v", err)
		}
		decoded.Write(content)
	}
	return decoded.String()
}

func TestBuildMIMEWithAttachment(t *testing.T) {
	attachment, err := domain.NewAttachment("report.txt", "text/plain", []byte("hello attachment"))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:        addr(t, "", "me@example.com"),
		To:          []domain.EmailAddress{addr(t, "", "a@example.com")},
		Subject:     "With file",
		Body:        "see attached",
		Attachments: []domain.Attachment{attachment},
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	out := string(BuildMIME(msg, time.Unix(0, 0).UTC(), "mid99"))

	wants := []string{
		`Content-Type: multipart/mixed; boundary="=_pigeonpost_mixed_mid99"` + "\r\n",
		"--=_pigeonpost_mixed_mid99\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"\r\nsee attached\r\n",
		`Content-Type: text/plain; name="report.txt"` + "\r\n",
		"Content-Transfer-Encoding: base64\r\n",
		`Content-Disposition: attachment; filename="report.txt"` + "\r\n",
		"--=_pigeonpost_mixed_mid99--\r\n",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("attachment output missing %q\n---\n%s", w, out)
		}
	}
	// The body part must precede the attachment part.
	if strings.Index(out, "see attached") > strings.Index(out, "filename=") {
		t.Error("message body must precede the attachment part")
	}
	// The base64 content must decode back to the original bytes.
	if !strings.Contains(out, base64.StdEncoding.EncodeToString([]byte("hello attachment"))) {
		t.Errorf("attachment base64 content missing\n---\n%s", out)
	}
}

func TestBuildMIMEWithoutCc(t *testing.T) {
	msg, _ := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From: addr(t, "", "me@example.com"),
		To:   []domain.EmailAddress{addr(t, "", "a@example.com")},
	})
	out := string(BuildMIME(msg, time.Unix(0, 0).UTC(), "x@y"))
	if strings.Contains(out, "Cc:") {
		t.Error("did not expect a Cc header")
	}
}

func TestBuildMIMEEncodesUnicodeSubject(t *testing.T) {
	msg, _ := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:    addr(t, "", "me@example.com"),
		To:      []domain.EmailAddress{addr(t, "", "a@example.com")},
		Subject: "Café résumé",
	})
	out := string(BuildMIME(msg, time.Unix(0, 0).UTC(), "x@y"))
	if !strings.Contains(out, "=?utf-8?") {
		t.Errorf("expected encoded-word subject, got:\n%s", out)
	}
}
