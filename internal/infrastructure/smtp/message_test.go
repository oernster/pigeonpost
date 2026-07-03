package smtp

import (
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
