package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// pkceVerifierBytes is the number of random bytes behind a PKCE code verifier. 32 bytes base64url-encode
// to 43 characters, inside RFC 7636's 43 to 128 range.
const pkceVerifierBytes = 32

// stateBytes is the number of random bytes behind the anti-forgery state value echoed through the
// redirect.
const stateBytes = 16

// browserOpener launches the system browser at a URL. It is injected so the authorizer stays free of the
// Wails runtime, which owns the real browser-open call.
type browserOpener func(url string) error

// Authorizer runs the authorization-code-with-PKCE flow over a loopback redirect: it opens the system
// browser at the provider's consent page, receives the redirect on a local listener, and exchanges the
// returned code for tokens. It implements application.OAuthAuthorizer.
type Authorizer struct {
	config  Config
	client  *http.Client
	open    browserOpener
	clock   domain.Clock
	entropy io.Reader
}

// NewAuthorizer constructs the authorizer with the provider config, HTTP client, a browser opener and a
// clock. The random source defaults to crypto/rand.
func NewAuthorizer(config Config, client *http.Client, open browserOpener, clock domain.Clock) *Authorizer {
	return &Authorizer{config: config, client: client, open: open, clock: clock, entropy: rand.Reader}
}

// authResult carries the outcome of the redirect back to Authorize: the authorization code on success, or
// an error describing a denied or malformed callback.
type authResult struct {
	code string
	err  error
}

// Authorize performs the full interactive flow and returns the signed-in address, a live access token and
// the JSON token blob to persist. It blocks until the user completes consent in the browser or ctx is
// cancelled.
func (a *Authorizer) Authorize(ctx context.Context) (application.OAuthCredential, error) {
	verifier, challenge, err := a.pkce()
	if err != nil {
		return application.OAuthCredential{}, err
	}
	state, err := a.randomString(stateBytes)
	if err != nil {
		return application.OAuthCredential{}, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return application.OAuthCredential{}, fmt.Errorf("oauth: open loopback listener: %w", err)
	}
	redirectURI := fmt.Sprintf("http://localhost:%d/", listener.Addr().(*net.TCPAddr).Port)

	results := make(chan authResult, 1)
	server := &http.Server{Handler: a.callbackHandler(state, results)}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Shutdown(context.Background()) }()

	if err := a.open(a.authURL(redirectURI, challenge, state)); err != nil {
		return application.OAuthCredential{}, fmt.Errorf("oauth: open browser: %w", err)
	}

	select {
	case <-ctx.Done():
		return application.OAuthCredential{}, ctx.Err()
	case result := <-results:
		if result.err != nil {
			return application.OAuthCredential{}, result.err
		}
		return a.redeem(ctx, result.code, redirectURI, verifier)
	}
}

// callbackHandler serves the loopback redirect: it validates the state, extracts the authorization code
// or a returned error, shows the user a plain page telling them to return to PigeonPost, and hands the
// outcome to Authorize. It ignores requests carrying neither a code nor an error (such as a browser
// favicon fetch) so a stray request does not resolve the flow.
func (a *Authorizer) callbackHandler(state string, results chan<- authResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("error") == "" && query.Get("code") == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		result := a.parseCallback(query, state)
		if result.err != nil {
			_, _ = io.WriteString(w, callbackPage("Sign-in failed. You can close this tab and return to PigeonPost."))
		} else {
			_, _ = io.WriteString(w, callbackPage("Signed in. You can close this tab and return to PigeonPost."))
		}
		select {
		case results <- result:
		default:
		}
	})
}

// parseCallback turns the redirect query into a result, rejecting a mismatched state (a possible forgery)
// and a provider-returned error.
func (a *Authorizer) parseCallback(query url.Values, state string) authResult {
	if query.Get("state") != state {
		return authResult{err: fmt.Errorf("oauth: redirect state mismatch")}
	}
	if e := query.Get("error"); e != "" {
		return authResult{err: fmt.Errorf("oauth: authorization denied: %s: %s", e, query.Get("error_description"))}
	}
	code := query.Get("code")
	if code == "" {
		return authResult{err: fmt.Errorf("oauth: redirect carried no authorization code")}
	}
	return authResult{code: code}
}

// redeem exchanges the authorization code for tokens and packs them into the credential the setup service
// stores.
func (a *Authorizer) redeem(ctx context.Context, code, redirectURI, verifier string) (application.OAuthCredential, error) {
	form := url.Values{
		"client_id":     {a.config.ClientID},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
		"scope":         {a.config.Scopes},
	}
	token, err := exchange(ctx, a.client, a.config, form, a.clock.Now(), Token{})
	if err != nil {
		return application.OAuthCredential{}, err
	}
	if token.Email == "" {
		return application.OAuthCredential{}, fmt.Errorf("oauth: token carried no signed-in address")
	}
	secret, err := Marshal(token)
	if err != nil {
		return application.OAuthCredential{}, err
	}
	return application.OAuthCredential{Email: token.Email, AccessToken: token.AccessToken, Secret: secret}, nil
}

// authURL builds the provider consent URL for the loopback redirect, requesting a query-mode code
// response and prompting the user to pick which account to sign in with.
func (a *Authorizer) authURL(redirectURI, challenge, state string) string {
	query := url.Values{
		"client_id":             {a.config.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"response_mode":         {"query"},
		"scope":                 {a.config.Scopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	return a.config.AuthorizeURL + "?" + query.Encode()
}

// pkce generates a PKCE code verifier and its S256 challenge.
func (a *Authorizer) pkce() (verifier, challenge string, err error) {
	verifier, err = a.randomString(pkceVerifierBytes)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// randomString returns n random bytes encoded as an unpadded base64url string, suitable for a PKCE
// verifier or a state value.
func (a *Authorizer) randomString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(a.entropy, buf); err != nil {
		return "", fmt.Errorf("oauth: read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// callbackPage wraps a short message in a minimal dark HTML page shown in the browser tab after the
// redirect.
func callbackPage(message string) string {
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>PigeonPost</title></head>" +
		"<body style=\"background:#161b22;color:#e6edf3;font-family:sans-serif;text-align:center;padding-top:4rem\">" +
		"<h2>PigeonPost</h2><p>" + message + "</p></body></html>"
}
