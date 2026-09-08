package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/message"
	"github.com/oernster/pigeonpost/internal/infrastructure/oauth"
)

// PasswordProvider yields the secret used to authenticate a password account, backed by the OS keychain.
type PasswordProvider interface {
	Password(ctx context.Context, account domain.Account) (string, error)
}

// TokenProvider yields a currently-valid OAuth access token for an account, refreshing it silently when
// the stored one has expired. It is used for OAuth accounts in place of a password.
type TokenProvider interface {
	AccessToken(ctx context.Context, account domain.Account) (string, error)
}

// IDGenerator produces the local part of a Message-ID for each sent message.
type IDGenerator func() string

// Transport is a MailTransport backed by a live SMTP server.
type Transport struct {
	passwords PasswordProvider
	tokens    TokenProvider
	clock     domain.Clock
	newID     IDGenerator
}

// NewTransport constructs the transport with its injected dependencies.
func NewTransport(passwords PasswordProvider, tokens TokenProvider, clock domain.Clock, newID IDGenerator) *Transport {
	return &Transport{passwords: passwords, tokens: tokens, clock: clock, newID: newID}
}

// Send authenticates to the account's outgoing server and delivers the message.
func (t *Transport) Send(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) error {
	out := account.Outgoing()
	addr := net.JoinHostPort(out.Host(), strconv.Itoa(out.Port()))

	auth, err := t.authClient(ctx, account)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{ServerName: out.Host()}
	var client *gosmtp.Client
	switch out.Security() {
	case domain.SecurityStartTLS:
		client, err = gosmtp.DialStartTLS(addr, tlsConfig)
	case domain.SecurityNone:
		client, err = gosmtp.Dial(addr)
	default:
		client, err = gosmtp.DialTLS(addr, tlsConfig)
	}
	if err != nil {
		// A dial failure means the server is unreachable: mark it offline so the caller can queue.
		return fmt.Errorf("smtp: dial %s: %w", addr, errors.Join(err, domain.ErrOffline))
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return authError(err)
	}

	body := message.BuildMIME(msg, t.clock.Now(), t.newID())
	recipients := addressStrings(msg.Recipients())
	if err := client.SendMail(msg.From().Address(), recipients, bytes.NewReader(body)); err != nil {
		return fmt.Errorf("smtp: send: %w", err)
	}
	return client.Quit()
}

// authError wraps a failure to authenticate, marking the case where the mailbox refused to take mail
// from a client at all so the interface can say something the user can act on. It is a function of its
// own rather than three lines inside the send, so the marking can be tested without a mail server:
// without that, the detector below could be right and never wired to anything.
func authError(err error) error {
	if isSMTPRefused(err) {
		return fmt.Errorf("smtp: authenticate: %w", errors.Join(err, domain.ErrSMTPRefused))
	}
	return fmt.Errorf("smtp: authenticate: %w", err)
}

// smtpRefusedResponse is the phrase the server uses when the mailbox will not accept mail from a client,
// matched on the server's own words because the SMTP reply carries a 535 that means several things. It is
// matched case-insensitively, which costs nothing and removes one way for the match to lapse silently.
//
// The response does not say WHY submission is refused and neither does this constant. Microsoft's own
// link alongside it describes a per-mailbox setting an administrator controls, which a personal account
// has no administrator for, so the words are not read here as naming a cause.
const smtpRefusedResponse = "smtpclientauthentication is disabled"

// isSMTPRefused reports whether err is the server refusing to take mail from an authenticated client. A
// false negative only means the server's own words are shown instead, so the match is kept narrow rather
// than clever: a plain wrong password answers 535 too and must keep saying so.
func isSMTPRefused(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), smtpRefusedResponse)
}

// authClient builds the SASL client for the account: XOAUTH2 carrying a silently-refreshed OAuth access
// token for an OAuth account, otherwise PLAIN carrying the stored keychain password.
func (t *Transport) authClient(ctx context.Context, account domain.Account) (sasl.Client, error) {
	if account.Auth() == domain.AuthOAuth2 {
		token, err := t.tokens.AccessToken(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("smtp: token for %q: %w", account.ID(), err)
		}
		return oauth.NewXOAUTH2Client(account.Address().Address(), token), nil
	}
	password, err := t.passwords.Password(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("smtp: password for %q: %w", account.ID(), err)
	}
	return sasl.NewPlainClient("", account.Address().Address(), password), nil
}

func addressStrings(addrs []domain.EmailAddress) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.Address())
	}
	return out
}
