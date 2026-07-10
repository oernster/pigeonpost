package imap

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

// parseUID converts an opaque message handle back into the numeric IMAP UID the server expects. On
// IMAP the handle is the UID held as a decimal string, so a value that is not a uint32 is a
// programming error rather than a server condition.
func parseUID(uid string) (imap.UID, error) {
	n, err := strconv.ParseUint(uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("imap: invalid uid %q: %w", uid, err)
	}
	return imap.UID(n), nil
}

// IDGenerator produces the local part of a Message-ID for a drafted message.
type IDGenerator func() string

// Source is a MailSource backed by a live IMAP server.
type Source struct {
	passwords PasswordProvider
	tokens    TokenProvider
	clock     domain.Clock
	newID     IDGenerator
}

// NewSource constructs the source with its injected password provider, OAuth token provider, clock and id
// generator. The clock and id generator are used only when appending a draft, so its Date and Message-ID
// headers are well-formed; the read paths do not use them.
func NewSource(passwords PasswordProvider, tokens TokenProvider, clock domain.Clock, newID IDGenerator) *Source {
	return &Source{passwords: passwords, tokens: tokens, clock: clock, newID: newID}
}

// dial opens a connection to the incoming server with the account's transport security and the given
// client options (nil for the one-shot fetch path; the IDLE watcher passes a unilateral-data handler). A
// dial failure is wrapped with ErrOffline so a caller can treat the server as unreachable.
func dial(incoming domain.ServerConfig, options *imapclient.Options) (*imapclient.Client, error) {
	address := fmt.Sprintf("%s:%d", incoming.Host(), incoming.Port())
	var (
		client *imapclient.Client
		err    error
	)
	switch incoming.Security() {
	case domain.SecurityStartTLS:
		client, err = imapclient.DialStartTLS(address, options)
	case domain.SecurityNone:
		client, err = imapclient.DialInsecure(address, options)
	default:
		client, err = imapclient.DialTLS(address, options)
	}
	if err != nil {
		return nil, fmt.Errorf("imap: dial %s: %w", address, errors.Join(err, domain.ErrOffline))
	}
	return client, nil
}
