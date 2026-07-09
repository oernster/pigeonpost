package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// flakyReader serves ok bytes of entropy and then fails, so both PKCE and state generation can be made to
// fail in turn.
type flakyReader struct{ ok int }

func (r *flakyReader) Read(p []byte) (int, error) {
	if r.ok <= 0 {
		return 0, errors.New("no entropy")
	}
	n := len(p)
	if n > r.ok {
		n = r.ok
	}
	r.ok -= n
	return n, nil
}

// redeemOpener returns a browser opener that drives the loopback redirect with the given code and the
// authorizer's own state, simulating the user completing consent. When probe is true it first hits the
// redirect with no parameters, exercising the handler's ignore-stray-request path.
func redeemOpener(t *testing.T, code string, probe bool) browserOpener {
	t.Helper()
	return func(rawurl string) error {
		u, err := url.Parse(rawurl)
		if err != nil {
			return err
		}
		redirect := u.Query().Get("redirect_uri")
		state := u.Query().Get("state")
		if probe {
			resp, err := http.Get(redirect)
			if err != nil {
				return err
			}
			_ = resp.Body.Close()
		}
		resp, err := http.Get(redirect + "?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state))
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}
}

func newTestAuthorizer(t *testing.T, tokenURL string, open browserOpener) *Authorizer {
	t.Helper()
	cfg := Config{
		AuthorizeURL: "https://login.example.com/authorize",
		TokenURL:     tokenURL,
		ClientID:     "cid",
		Scopes:       "openid email",
	}
	return NewAuthorizer(cfg, http.DefaultClient, open, fixedClock{t: time.Unix(9000, 0).UTC()})
}

func TestAuthorizeSuccess(t *testing.T) {
	var form map[string][]string
	body := `{"access_token":"acc","refresh_token":"ref","expires_in":3600,"id_token":"` +
		makeIDToken(`{"email":"user@example.com"}`) + `"}`
	srv := tokenServer(t, http.StatusOK, body, &form)

	auth := newTestAuthorizer(t, srv.URL, redeemOpener(t, "auth-code-123", true))
	auth.client = srv.Client()

	cred, err := auth.Authorize(context.Background())
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if cred.Email != "user@example.com" || cred.AccessToken != "acc" {
		t.Errorf("credential = %+v, want user@example.com/acc", cred)
	}
	// The stored secret round-trips back to the full token.
	token, err := Unmarshal(cred.Secret)
	if err != nil {
		t.Fatalf("unmarshal secret: %v", err)
	}
	if token.RefreshToken != "ref" {
		t.Errorf("secret refresh token = %q, want ref", token.RefreshToken)
	}
	// The exchange used the authorization-code grant with the returned code and a PKCE verifier.
	if form["grant_type"][0] != "authorization_code" || form["code"][0] != "auth-code-123" {
		t.Errorf("token form = %v", form)
	}
	if form["code_verifier"][0] == "" {
		t.Error("expected a PKCE code verifier in the exchange")
	}
}

func TestAuthorizeExchangeError(t *testing.T) {
	srv := tokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`, nil)
	auth := newTestAuthorizer(t, srv.URL, redeemOpener(t, "code", false))
	auth.client = srv.Client()

	if _, err := auth.Authorize(context.Background()); err == nil {
		t.Error("expected an error when the token exchange fails")
	}
}

func TestAuthorizeNoEmail(t *testing.T) {
	srv := tokenServer(t, http.StatusOK, `{"access_token":"acc","expires_in":3600}`, nil)
	auth := newTestAuthorizer(t, srv.URL, redeemOpener(t, "code", false))
	auth.client = srv.Client()

	if _, err := auth.Authorize(context.Background()); err == nil {
		t.Error("expected an error when the token carries no signed-in address")
	}
}

func TestAuthorizeOpenError(t *testing.T) {
	auth := newTestAuthorizer(t, "http://unused", func(string) error { return errors.New("no browser") })
	if _, err := auth.Authorize(context.Background()); err == nil {
		t.Error("expected an error when the browser cannot be opened")
	}
}

func TestAuthorizeContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	auth := newTestAuthorizer(t, "http://unused", func(string) error { return nil })
	if _, err := auth.Authorize(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestAuthorizeEntropyErrors(t *testing.T) {
	t.Run("pkce", func(t *testing.T) {
		auth := newTestAuthorizer(t, "http://unused", func(string) error { return nil })
		auth.entropy = &flakyReader{ok: 0}
		if _, err := auth.Authorize(context.Background()); err == nil {
			t.Error("expected an error when PKCE entropy fails")
		}
	})
	t.Run("state", func(t *testing.T) {
		auth := newTestAuthorizer(t, "http://unused", func(string) error { return nil })
		auth.entropy = &flakyReader{ok: pkceVerifierBytes}
		if _, err := auth.Authorize(context.Background()); err == nil {
			t.Error("expected an error when state entropy fails")
		}
	})
}

func TestParseCallback(t *testing.T) {
	auth := newTestAuthorizer(t, "http://unused", func(string) error { return nil })
	const state = "expected-state"

	t.Run("state mismatch", func(t *testing.T) {
		res := auth.parseCallback(url.Values{"state": {"wrong"}, "code": {"c"}}, state)
		if res.err == nil {
			t.Error("expected a state-mismatch error")
		}
	})
	t.Run("provider error", func(t *testing.T) {
		res := auth.parseCallback(url.Values{"state": {state}, "error": {"access_denied"}}, state)
		if res.err == nil {
			t.Error("expected an authorization-denied error")
		}
	})
	t.Run("no code", func(t *testing.T) {
		res := auth.parseCallback(url.Values{"state": {state}}, state)
		if res.err == nil {
			t.Error("expected a missing-code error")
		}
	})
	t.Run("valid", func(t *testing.T) {
		res := auth.parseCallback(url.Values{"state": {state}, "code": {"good"}}, state)
		if res.err != nil || res.code != "good" {
			t.Errorf("parseCallback = %+v, want code good", res)
		}
	})
}
