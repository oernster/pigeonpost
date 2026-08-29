package imap

import (
	"errors"
	"testing"
)

// The detector reads the server's own words, so these cases are the contract: what Microsoft actually
// returns for a mailbox with IMAP switched off, then the errors that must not be mistaken for it. A false
// positive would tell someone to change a setting that is not their problem, which is worse than the
// unhelpful message it replaces.
func TestIsIMAPRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"the response Microsoft returns", errors.New("imap: NO User is authenticated but not connected."), true},
		{"wrapped by the adapter", errors.New(`imap: xoauth2 "a@hotmail.com": imap: NO User is authenticated but not connected.`), true},
		{"a different case from the server", errors.New("NO user is AUTHENTICATED BUT NOT CONNECTED"), true},
		{"a rejected credential", errors.New("imap: NO AUTHENTICATE failed"), false},
		{"a missing mailbox", errors.New("imap: NO mailbox does not exist"), false},
		{"no error at all", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := isIMAPRefused(c.err); got != c.want {
				t.Fatalf("isIMAPRefused(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
