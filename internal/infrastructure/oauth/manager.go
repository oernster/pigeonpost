package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Store reads and writes an account's stored secret in the OS keychain. For an OAuth account the secret
// is the JSON token blob rather than a password. It is the subset of the keychain vault the token manager
// needs, so the manager depends on this narrow contract rather than the whole vault.
type Store interface {
	Password(ctx context.Context, account domain.Account) (string, error)
	SetPassword(ctx context.Context, account domain.Account, secret string) error
}

// TokenManager yields a currently-valid access token for an OAuth account, refreshing it silently and
// persisting the renewed token when the stored one has expired. It is the runtime credential provider the
// IMAP and SMTP adapters use for OAuth accounts, in place of reading a static password.
type TokenManager struct {
	store  Store
	config Config
	client *http.Client
	clock  domain.Clock
}

// NewTokenManager constructs the manager with the keychain store, provider config, HTTP client and clock.
func NewTokenManager(store Store, config Config, client *http.Client, clock domain.Clock) *TokenManager {
	return &TokenManager{store: store, config: config, client: client, clock: clock}
}

// AccessToken returns a valid bearer access token for the account. It reads the stored token, returns its
// access token when still valid, and otherwise exchanges the refresh token for a new one, persisting the
// result so the next call reuses it.
func (m *TokenManager) AccessToken(ctx context.Context, account domain.Account) (string, error) {
	blob, err := m.store.Password(ctx, account)
	if err != nil {
		return "", fmt.Errorf("oauth: read token for %q: %w", account.ID(), err)
	}
	token, err := Unmarshal(blob)
	if err != nil {
		return "", err
	}
	if token.Valid(m.clock.Now()) {
		return token.AccessToken, nil
	}
	refreshed, err := m.refresh(ctx, token)
	if err != nil {
		return "", fmt.Errorf("oauth: refresh token for %q: %w", account.ID(), err)
	}
	stored, err := Marshal(refreshed)
	if err != nil {
		return "", err
	}
	if err := m.store.SetPassword(ctx, account, stored); err != nil {
		return "", fmt.Errorf("oauth: persist refreshed token for %q: %w", account.ID(), err)
	}
	return refreshed.AccessToken, nil
}

// refresh exchanges the stored refresh token for a fresh access token, carrying the previous email and
// refresh token forward when the reply omits them.
func (m *TokenManager) refresh(ctx context.Context, token Token) (Token, error) {
	if token.RefreshToken == "" {
		return Token{}, fmt.Errorf("oauth: no refresh token stored")
	}
	form := url.Values{
		"client_id":     {m.config.ClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
		"scope":         {m.config.Scopes},
	}
	return exchange(ctx, m.client, m.config, form, m.clock.Now(), token)
}
