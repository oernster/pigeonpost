package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// A refused sign-in is the commonest mail failure there is and it used to reach the reader as the
// server's own tagged line. The message that replaces it must not assert which of the several causes
// applies, so what is asserted here is that it names the two places worth looking and nothing more.
func TestFriendlyMailErrorTranslatesARefusedSignIn(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("sync: fetch folders: %w",
		fmt.Errorf("imap: login %q: %w", "a@gmail.com",
			errors.Join(errors.New("imap: NO [AUTHENTICATIONFAILED] Invalid credentials (Failure)"), domain.ErrSignInRefused)))
	got := friendlyMailError(wrapped)
	if !errors.Is(got, errSignInRefused) {
		t.Fatalf("friendlyMailError did not translate a refused sign-in, got %v", got)
	}
	for _, want := range []string{"refused", "password", "app password"} {
		if !strings.Contains(got.Error(), want) {
			t.Fatalf("message %q does not name %q, so it cannot be acted on", got.Error(), want)
		}
	}
	// The failure names no cause, so neither may the message: claiming a wrong password against a
	// revoked app password or a provider that has dropped plain passwords is the mistake this file
	// already carries a measurement of.
	if strings.Contains(strings.ToLower(got.Error()), "wrong password") {
		t.Fatal("the message asserts a cause the refusal does not carry")
	}
}

// Where the server states the remedy itself, the reader gets that rather than the general refusal.
func TestFriendlyMailErrorTranslatesAnAppPasswordRefusal(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("imap: login %q: %w", "a@gmail.com",
		errors.Join(
			errors.New("imap: NO [ALERT] Application-specific password required"),
			domain.ErrAppPasswordRequired, domain.ErrSignInRefused))
	got := friendlyMailError(wrapped)
	if !errors.Is(got, errAppPasswordRequired) {
		t.Fatalf("the server's own statement was not translated, got %v", got)
	}
	if !strings.Contains(got.Error(), "app password") {
		t.Fatalf("message %q does not name the app password the server asked for", got.Error())
	}
}

// The reply the client cannot decode is the failure that prompted all of this: the reader was shown the
// grammar production that ran out. It becomes a sentence; it stays free of any cause.
func TestFriendlyMailErrorTranslatesAnUnreadableReply(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("sync: fetch messages for %q: %w", "[Google Mail]/All Mail",
		errors.Join(
			errors.New(`imap: fetch: in body-type-1part: imapwire: expected '(', got "N"`),
			domain.ErrUnreadableResponse))
	got := friendlyMailError(wrapped)
	if !errors.Is(got, errUnreadableResponse) {
		t.Fatalf("an unreadable reply was not translated, got %v", got)
	}
	for _, leak := range []string{"imapwire", "body-type", "expected"} {
		if strings.Contains(got.Error(), leak) {
			t.Fatalf("message %q still shows the reader %q", got.Error(), leak)
		}
	}
}
