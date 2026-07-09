package oauth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fixedClock is a domain.Clock returning a constant time.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// makeIDToken builds a fake unsigned JWT with the given JSON payload, enough for email extraction (which
// does not verify the signature).
func makeIDToken(payload string) string {
	seg := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return seg(`{"alg":"none"}`) + "." + seg(payload) + "." + seg("sig")
}

// tokenServer starts an httptest server that replies to the token endpoint with the given status and body,
// recording the last posted form.
func tokenServer(t *testing.T, status int, body string, gotForm *map[string][]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if gotForm != nil {
			*gotForm = r.PostForm
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTokenValid(t *testing.T) {
	now := time.Unix(1000, 0)
	cases := []struct {
		name  string
		token Token
		want  bool
	}{
		{"fresh", Token{AccessToken: "a", Expiry: now.Add(10 * time.Minute)}, true},
		{"empty access token", Token{Expiry: now.Add(10 * time.Minute)}, false},
		{"expired", Token{AccessToken: "a", Expiry: now.Add(-time.Minute)}, false},
		{"within skew", Token{AccessToken: "a", Expiry: now.Add(30 * time.Second)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.token.Valid(now); got != tc.want {
				t.Errorf("Valid = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	token := Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Unix(1234, 0).UTC(), Email: "x@y.z"}
	blob, err := Marshal(token)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	back, err := Unmarshal(blob)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back != token {
		t.Errorf("round trip = %+v, want %+v", back, token)
	}
}

func TestUnmarshalError(t *testing.T) {
	if _, err := Unmarshal("not json"); err == nil {
		t.Error("expected an error unmarshalling junk")
	}
}

func TestEmailFromIDToken(t *testing.T) {
	cases := []struct {
		name  string
		token string
		want  string
	}{
		{"email claim", makeIDToken(`{"email":"a@b.com"}`), "a@b.com"},
		{"preferred_username fallback", makeIDToken(`{"preferred_username":"c@d.com"}`), "c@d.com"},
		{"email preferred over username", makeIDToken(`{"email":"a@b.com","preferred_username":"c@d.com"}`), "a@b.com"},
		{"no address", makeIDToken(`{"sub":"123"}`), ""},
		{"too few segments", "only.two", ""},
		{"bad base64 payload", "aaa." + "!!!bad!!!" + ".ccc", ""},
		{"bad json payload", makeIDToken("not json"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := emailFromIDToken(tc.token); got != tc.want {
				t.Errorf("emailFromIDToken = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExchangeSuccess(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	body := `{"access_token":"acc","refresh_token":"ref","expires_in":3600,"id_token":"` +
		makeIDToken(`{"email":"user@example.com"}`) + `"}`
	srv := tokenServer(t, http.StatusOK, body, nil)
	cfg := Config{TokenURL: srv.URL, ClientID: "cid"}

	token, err := exchange(context.Background(), srv.Client(), cfg, map[string][]string{}, now, Token{})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if token.AccessToken != "acc" || token.RefreshToken != "ref" {
		t.Errorf("tokens = %q/%q, want acc/ref", token.AccessToken, token.RefreshToken)
	}
	if !token.Expiry.Equal(now.Add(3600 * time.Second)) {
		t.Errorf("expiry = %v, want now+3600s", token.Expiry)
	}
	if token.Email != "user@example.com" {
		t.Errorf("email = %q, want user@example.com", token.Email)
	}
}

func TestExchangeCarriesPrevForwardOnRefresh(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	// A refresh reply commonly omits the refresh token and carries no id token.
	srv := tokenServer(t, http.StatusOK, `{"access_token":"acc2","expires_in":10}`, nil)
	cfg := Config{TokenURL: srv.URL}
	prev := Token{RefreshToken: "keep", Email: "prev@example.com"}

	token, err := exchange(context.Background(), srv.Client(), cfg, map[string][]string{}, now, prev)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if token.RefreshToken != "keep" {
		t.Errorf("refresh token = %q, want carried-forward keep", token.RefreshToken)
	}
	if token.Email != "prev@example.com" {
		t.Errorf("email = %q, want carried-forward prev", token.Email)
	}
}

func TestExchangeErrors(t *testing.T) {
	now := time.Unix(0, 0)
	t.Run("endpoint error payload", func(t *testing.T) {
		srv := tokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"bad"}`, nil)
		if _, err := exchange(context.Background(), srv.Client(), Config{TokenURL: srv.URL}, map[string][]string{}, now, Token{}); err == nil ||
			!strings.Contains(err.Error(), "invalid_grant") {
			t.Errorf("error = %v, want invalid_grant", err)
		}
	})
	t.Run("no access token", func(t *testing.T) {
		srv := tokenServer(t, http.StatusOK, `{"token_type":"Bearer"}`, nil)
		if _, err := exchange(context.Background(), srv.Client(), Config{TokenURL: srv.URL}, map[string][]string{}, now, Token{}); err == nil {
			t.Error("expected an error when no access token is returned")
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		srv := tokenServer(t, http.StatusOK, `not json`, nil)
		if _, err := exchange(context.Background(), srv.Client(), Config{TokenURL: srv.URL}, map[string][]string{}, now, Token{}); err == nil {
			t.Error("expected an error decoding a malformed response")
		}
	})
	t.Run("unreachable endpoint", func(t *testing.T) {
		cfg := Config{TokenURL: "http://127.0.0.1:0/"}
		if _, err := exchange(context.Background(), http.DefaultClient, cfg, map[string][]string{}, now, Token{}); err == nil {
			t.Error("expected an error reaching an unreachable endpoint")
		}
	})
	t.Run("bad request url", func(t *testing.T) {
		cfg := Config{TokenURL: "://bad"}
		if _, err := exchange(context.Background(), http.DefaultClient, cfg, map[string][]string{}, now, Token{}); err == nil {
			t.Error("expected an error building a request with a bad url")
		}
	})
}

func TestMicrosoftConfig(t *testing.T) {
	cfg := MicrosoftConfig()
	if cfg.ClientID == "" || !strings.Contains(cfg.AuthorizeURL, "login.microsoftonline.com") {
		t.Errorf("unexpected Microsoft config: %+v", cfg)
	}
	if !strings.Contains(cfg.Scopes, "IMAP.AccessAsUser.All") || !strings.Contains(cfg.Scopes, "SMTP.Send") {
		t.Errorf("scopes missing mailbox access: %q", cfg.Scopes)
	}
}
