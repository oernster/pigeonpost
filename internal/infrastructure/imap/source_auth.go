package imap

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/oauth"
)

// PasswordProvider yields the secret used to authenticate a password account. It is backed by the OS
// keychain so passwords never touch the local database.
type PasswordProvider interface {
	Password(ctx context.Context, account domain.Account) (string, error)
}

// TokenProvider yields a currently-valid OAuth access token for an account, refreshing it silently when
// the stored one has expired. It is used for OAuth accounts in place of a password.
type TokenProvider interface {
	AccessToken(ctx context.Context, account domain.Account) (string, error)
}

// connect dials and authenticates using the account's stored credential (a keychain password, or a
// silently-refreshed OAuth access token for an OAuth account). It is used by the operations that run
// against a saved account.
func (s *Source) connect(ctx context.Context, account domain.Account) (*imapclient.Client, error) {
	secret, err := s.secret(ctx, account)
	if err != nil {
		return nil, err
	}
	return s.authWith(account, secret)
}

// secret returns the credential to authenticate with: a refreshed OAuth access token for an OAuth
// account, otherwise the stored keychain password.
func (s *Source) secret(ctx context.Context, account domain.Account) (string, error) {
	return credentialFor(ctx, s.passwords, s.tokens, account, "imap")
}

// credentialFor selects the secret to authenticate an account with: a refreshed OAuth access token for an
// OAuth account, otherwise the stored keychain password. errLabel prefixes the wrapped error so the
// one-shot fetch path and the long-lived IDLE watcher stay distinguishable, since both authenticate the
// same way and would otherwise drift apart.
func credentialFor(ctx context.Context, passwords PasswordProvider, tokens TokenProvider, account domain.Account, errLabel string) (string, error) {
	if account.Auth() == domain.AuthOAuth2 {
		token, err := tokens.AccessToken(ctx, account)
		if err != nil {
			return "", fmt.Errorf("%s: token for %q: %w", errLabel, account.ID(), err)
		}
		return token, nil
	}
	password, err := passwords.Password(ctx, account)
	if err != nil {
		return "", fmt.Errorf("%s: password for %q: %w", errLabel, account.ID(), err)
	}
	return password, nil
}

// authWith dials the account's incoming server and authenticates with the given secret, using XOAUTH2 for
// an OAuth account (the secret is the bearer access token) and LOGIN otherwise (the secret is the
// password). It is shared by connect, which reads the stored credential, and Verify, which is handed a
// candidate secret before anything is persisted.
func (s *Source) authWith(account domain.Account, secret string) (*imapclient.Client, error) {
	client, err := dial(account.Incoming(), nil)
	if err != nil {
		return nil, err
	}
	if err := authenticate(client, account, secret); err != nil {
		_ = client.Logout().Wait()
		return nil, err
	}
	return client, nil
}

// authenticate presents the secret to an already-dialled client: XOAUTH2 for an OAuth account (the secret
// is the bearer access token) and LOGIN otherwise (the secret is the password). It is shared by the
// one-shot fetch path and the long-lived IDLE watcher, which each dial the connection differently.
func authenticate(client *imapclient.Client, account domain.Account, secret string) error {
	if account.Auth() == domain.AuthOAuth2 {
		if err := client.Authenticate(oauth.NewXOAUTH2Client(account.Address().Address(), secret)); err != nil {
			return fmt.Errorf("imap: xoauth2 %q: %w", account.ID(), err)
		}
		return nil
	}
	if err := client.Login(account.Address().Address(), secret).Wait(); err != nil {
		return fmt.Errorf("imap: login %q: %w", account.ID(), err)
	}
	return nil
}

// Verify proves a candidate secret against the account's incoming server by authenticating and logging
// out again. The secret is a password for a password account and a bearer access token for an OAuth
// account. It satisfies application.AccountVerifier and runs before an account is persisted, so the
// keychain is never written with an unverified secret.
func (s *Source) Verify(_ context.Context, account domain.Account, password string) error {
	client, err := s.authWith(account, password)
	if err != nil {
		return err
	}
	if err := client.Logout().Wait(); err != nil {
		return fmt.Errorf("imap: logout %q: %w", account.ID(), err)
	}
	return nil
}
