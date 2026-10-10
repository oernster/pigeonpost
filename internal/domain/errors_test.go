package domain

import (
	"errors"
	"fmt"
	"testing"
)

// The phrase a provider uses when it wants an application-specific password is matched here rather than
// in each adapter, so both the mail reader and the mail sender read one spelling of it. The match is
// deliberately narrow: a miss costs the specific message and leaves the general refusal in its place,
// while a false hit would tell somebody to create a password the server never asked for.
func TestIsAppPasswordRequired(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		err  error
		want bool
	}{
		"nothing went wrong":   {nil, false},
		"the alert form":       {errors.New("imap: NO [ALERT] Application-specific password required"), true},
		"lower case":           {errors.New("535 5.7.8 application-specific password required"), true},
		"wrapped several deep": {errors.New(`imap: login "a@x": Application-specific password required`), true},
		"a plain refusal":      {errors.New("imap: NO [AUTHENTICATIONFAILED] Invalid credentials (Failure)"), false},
		"an unrelated failure": {errors.New("dial tcp: connection refused"), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := IsAppPasswordRequired(tc.err); got != tc.want {
				t.Errorf("IsAppPasswordRequired(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// A failure worth trying again later is one where the server could not be reached or said it was down;
// anything else (a refusal, a fault) is final and must not sit in the Outbox retrying forever.
func TestIsRetryLater(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		err  error
		want bool
	}{
		"nothing went wrong":     {nil, false},
		"unreachable":            {fmt.Errorf("smtp: dial: %w", ErrOffline), true},
		"down behind the server": {fmt.Errorf("smtp: authenticate: %w", ErrServerUnavailable), true},
		"a refused sign-in":      {fmt.Errorf("imap: login: %w", ErrSignInRefused), false},
		"an unrelated failure":   {errors.New("smtp: send: 550 no such user"), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetryLater(tc.err); got != tc.want {
				t.Errorf("IsRetryLater(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// The sentinels the mail adapters raise are distinct values, so a caller matching one never matches
// another by accident. An app-password refusal is the one case that deliberately carries two.
func TestMailSentinelsAreDistinct(t *testing.T) {
	t.Parallel()
	for _, sentinel := range []error{ErrSignInRefused, ErrAppPasswordRequired, ErrUnreadableResponse, ErrServerUnavailable} {
		for _, other := range []error{ErrOffline, ErrIMAPRefused, ErrSMTPRefused} {
			if errors.Is(sentinel, other) {
				t.Errorf("%v matches %v, so one refusal would be read as another", sentinel, other)
			}
		}
	}
	if errors.Is(ErrServerUnavailable, ErrSignInRefused) {
		t.Error("an unavailable server matches a refused sign-in, so an outage would be read as a bad password")
	}
}
