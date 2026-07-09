package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Microsoft's common (multi-tenant plus personal account) OAuth 2.0 endpoints, the public client id
// registered for PigeonPost and the delegated scopes it requests. The client id is a public identifier,
// safe to embed; a public client holds no secret. IMAP.AccessAsUser.All and SMTP.Send grant mailbox
// access, offline_access yields a refresh token for silent renewal, and openid plus email return the
// signed-in address in the id token.
const (
	microsoftAuthorizeURL = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	microsoftTokenURL     = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	microsoftClientID     = "f62852e4-f9dd-4e09-8f59-984ca04f6b91"
	microsoftScopes       = "offline_access openid email " +
		"https://outlook.office.com/IMAP.AccessAsUser.All " +
		"https://outlook.office.com/SMTP.Send"
)

// refreshSkew is subtracted from a token's real expiry so a token is treated as expired slightly early,
// leaving room to refresh before a request would be rejected for using an only-just-expired token.
const refreshSkew = 60 * time.Second

// Config holds the OAuth endpoints, public client id and scopes for a provider. It is a value so it can
// be copied into the authorizer and the token manager, and overridden in tests to point at a stub server.
type Config struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	Scopes       string
}

// MicrosoftConfig returns the configuration for Microsoft personal and work/school accounts.
func MicrosoftConfig() Config {
	return Config{
		AuthorizeURL: microsoftAuthorizeURL,
		TokenURL:     microsoftTokenURL,
		ClientID:     microsoftClientID,
		Scopes:       microsoftScopes,
	}
}

// Token is a stored OAuth credential: the current bearer access token, the long-lived refresh token used
// to renew it, the moment the access token stops being accepted and the signed-in address. It is
// persisted to the OS keychain as JSON, in place of a password, so an OAuth account holds no reusable
// secret in the app database.
type Token struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	Expiry       time.Time `json:"expiry"`
	Email        string    `json:"email"`
}

// Valid reports whether the access token is still usable at now, accounting for the refresh skew so a
// token about to expire is renewed rather than used.
func (t Token) Valid(now time.Time) bool {
	return t.AccessToken != "" && now.Before(t.Expiry.Add(-refreshSkew))
}

// Marshal serialises a token to the JSON blob stored in the keychain.
func Marshal(token Token) (string, error) {
	data, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("oauth: marshal token: %w", err)
	}
	return string(data), nil
}

// Unmarshal parses the JSON blob stored in the keychain back into a token.
func Unmarshal(blob string) (Token, error) {
	var token Token
	if err := json.Unmarshal([]byte(blob), &token); err != nil {
		return Token{}, fmt.Errorf("oauth: unmarshal token: %w", err)
	}
	return token, nil
}

// tokenResponse is the token endpoint's JSON reply. expires_in is the access token's lifetime in seconds
// from now; id_token is a JWT whose payload carries the signed-in address.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// exchange posts an OAuth token request (authorization-code or refresh-token grant) to the token endpoint
// and maps the reply to a Token. now stamps the absolute expiry from the returned lifetime, and prev
// carries forward the refresh token and email when a refresh reply omits them (Microsoft may return the
// same refresh token by omission, and a refresh reply carries no id token).
func exchange(ctx context.Context, client *http.Client, cfg Config, form url.Values, now time.Time, prev Token) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("oauth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("oauth: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("oauth: read token response: %w", err)
	}
	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Token{}, fmt.Errorf("oauth: decode token response: %w", err)
	}
	if parsed.Error != "" {
		return Token{}, fmt.Errorf("oauth: token endpoint: %s: %s", parsed.Error, parsed.ErrorDesc)
	}
	if parsed.AccessToken == "" {
		return Token{}, fmt.Errorf("oauth: token endpoint returned no access token")
	}

	token := Token{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		Expiry:       now.Add(time.Duration(parsed.ExpiresIn) * time.Second),
		Email:        prev.Email,
	}
	if token.RefreshToken == "" {
		token.RefreshToken = prev.RefreshToken
	}
	if parsed.IDToken != "" {
		if email := emailFromIDToken(parsed.IDToken); email != "" {
			token.Email = email
		}
	}
	return token, nil
}

// emailFromIDToken reads the signed-in address from a JWT id token without verifying its signature: the
// token arrived directly from Microsoft over TLS and is used only to name the account, not to authorise
// anything. It decodes the payload segment and prefers the email claim, falling back to
// preferred_username (which is the address for Microsoft accounts). It returns "" when no address is
// present, so the caller can fail cleanly.
func emailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) < 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if claims.Email != "" {
		return claims.Email
	}
	return claims.PreferredUsername
}
