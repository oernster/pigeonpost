package smtp

import (
	"errors"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The detector is matched on the server's own words because a 535 means several things. What matters is
// the split: the mailbox refusing client submission is translated for the user, while an ordinary bad
// credential keeps saying what the server said. Getting that wrong in the generous direction would tell
// someone with a wrong password to wait for Microsoft.
func TestIsSMTPRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "the mailbox refusing client submission, as Microsoft words it",
			text: "SMTP error 535: Authentication unsuccessful, SmtpClientAuthentication is disabled for " +
				"the Mailbox. Visit https://aka.ms/smtp_auth_disabled for more information.",
			want: true,
		},
		{
			// Matched case-insensitively, so a change of casing in the server's response cannot make the
			// match lapse in silence.
			name: "the same response in another casing",
			text: "smtpclientauthentication is disabled for the mailbox",
			want: true,
		},
		{
			name: "an ordinary wrong password",
			text: "SMTP error 535: Authentication unsuccessful, the user name or password is incorrect",
			want: false,
		},
		{
			name: "an unrelated failure",
			text: "SMTP error 550: mailbox unavailable",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isSMTPRefused(errorOf(tc.text)); got != tc.want {
				t.Errorf("isSMTPRefused(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
	if isSMTPRefused(nil) {
		t.Error("a nil error is not a refusal")
	}
}

// errorOf builds an error carrying text, so the table above reads as the server responses it describes.
func errorOf(text string) error { return simpleError(text) }

type simpleError string

func (e simpleError) Error() string { return string(e) }

// The detector being right is not enough: it has to be wired to the error the send returns. This is
// what holds the two together; it is also why authError is a function rather than three lines inside a
// send that would need a mail server to reach.
func TestAuthErrorMarksAMailboxThatRefusedSubmission(t *testing.T) {
	t.Parallel()
	server := errorOf("SMTP error 535: Authentication unsuccessful, SmtpClientAuthentication is " +
		"disabled for the Mailbox.")

	got := authError(server)
	if !errors.Is(got, domain.ErrSMTPRefused) {
		t.Fatalf("a refused submission was not marked: %v", got)
	}
	// The server's own words survive the wrapping, since they are what the error log keeps when the
	// interface replaces the message.
	if !strings.Contains(got.Error(), "SmtpClientAuthentication is disabled") {
		t.Errorf("the server's response was lost: %v", got)
	}
}

func TestAuthErrorLeavesAnOrdinaryFailureUnmarked(t *testing.T) {
	t.Parallel()
	got := authError(errorOf("SMTP error 535: Authentication unsuccessful, the user name or password is incorrect"))
	if errors.Is(got, domain.ErrSMTPRefused) {
		t.Fatalf("a wrong password was marked as a refused mailbox: %v", got)
	}
	if !strings.Contains(got.Error(), "password is incorrect") {
		t.Errorf("the server's response was lost: %v", got)
	}
}
