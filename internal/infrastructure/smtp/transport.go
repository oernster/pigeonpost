package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/message"
)

// PasswordProvider yields the secret used to authenticate an account, backed by the OS keychain.
type PasswordProvider interface {
	Password(ctx context.Context, account domain.Account) (string, error)
}

// IDGenerator produces the local part of a Message-ID for each sent message.
type IDGenerator func() string

// Transport is a MailTransport backed by a live SMTP server.
type Transport struct {
	passwords PasswordProvider
	clock     domain.Clock
	newID     IDGenerator
}

// NewTransport constructs the transport with its injected dependencies.
func NewTransport(passwords PasswordProvider, clock domain.Clock, newID IDGenerator) *Transport {
	return &Transport{passwords: passwords, clock: clock, newID: newID}
}

// Send authenticates to the account's outgoing server and delivers the message.
func (t *Transport) Send(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) error {
	out := account.Outgoing()
	addr := fmt.Sprintf("%s:%d", out.Host(), out.Port())

	password, err := t.passwords.Password(ctx, account)
	if err != nil {
		return fmt.Errorf("smtp: password for %q: %w", account.ID(), err)
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

	auth := sasl.NewPlainClient("", account.Address().Address(), password)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp: authenticate: %w", err)
	}

	body := message.BuildMIME(msg, t.clock.Now(), t.newID())
	recipients := addressStrings(msg.Recipients())
	if err := client.SendMail(msg.From().Address(), recipients, bytes.NewReader(body)); err != nil {
		return fmt.Errorf("smtp: send: %w", err)
	}
	return client.Quit()
}

func addressStrings(addrs []domain.EmailAddress) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.Address())
	}
	return out
}
