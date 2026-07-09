package oauth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeStore is a hand-written keychain Store with error injection. It records the last secret written so a
// persisted refresh can be asserted.
type fakeStore struct {
	secret    string
	written   string
	getErr    error
	setErr    error
	setCalled bool
}

func (s *fakeStore) Password(context.Context, domain.Account) (string, error) {
	return s.secret, s.getErr
}

func (s *fakeStore) SetPassword(_ context.Context, _ domain.Account, secret string) error {
	s.setCalled = true
	if s.setErr != nil {
		return s.setErr
	}
	s.written = secret
	return nil
}

// oauthAccount builds a minimal OAuth account for the manager tests.
func oauthAccount(t *testing.T) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	sc, err := domain.NewServerConfig("outlook.office365.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("server config: %v", err)
	}
	account, err := domain.NewAccount("user@example.com", "User", addr, domain.ProtocolIMAP, sc, sc, domain.AuthOAuth2)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

func storedToken(t *testing.T, token Token) string {
	t.Helper()
	blob, err := Marshal(token)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return blob
}

func TestAccessTokenStillValid(t *testing.T) {
	now := time.Unix(5000, 0)
	store := &fakeStore{secret: storedToken(t, Token{AccessToken: "live", Expiry: now.Add(time.Hour)})}
	mgr := NewTokenManager(store, Config{}, http.DefaultClient, fixedClock{t: now})

	got, err := mgr.AccessToken(context.Background(), oauthAccount(t))
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if got != "live" {
		t.Errorf("token = %q, want live", got)
	}
	if store.setCalled {
		t.Error("a valid token must not be refreshed or re-persisted")
	}
}

func TestAccessTokenRefreshesAndPersists(t *testing.T) {
	now := time.Unix(6000, 0).UTC()
	var form map[string][]string
	srv := tokenServer(t, http.StatusOK, `{"access_token":"fresh","refresh_token":"newref","expires_in":3600}`, &form)
	store := &fakeStore{secret: storedToken(t, Token{AccessToken: "stale", RefreshToken: "oldref", Email: "user@example.com", Expiry: now.Add(-time.Minute)})}
	mgr := NewTokenManager(store, Config{TokenURL: srv.URL, ClientID: "cid", Scopes: "s"}, srv.Client(), fixedClock{t: now})

	got, err := mgr.AccessToken(context.Background(), oauthAccount(t))
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if got != "fresh" {
		t.Errorf("token = %q, want fresh", got)
	}
	if form["grant_type"][0] != "refresh_token" || form["refresh_token"][0] != "oldref" {
		t.Errorf("refresh form = %v, want grant_type=refresh_token refresh_token=oldref", form)
	}
	// The refreshed token is persisted, keeping the carried-forward email.
	persisted, err := Unmarshal(store.written)
	if err != nil {
		t.Fatalf("unmarshal persisted: %v", err)
	}
	if persisted.AccessToken != "fresh" || persisted.RefreshToken != "newref" || persisted.Email != "user@example.com" {
		t.Errorf("persisted token = %+v", persisted)
	}
}

func TestAccessTokenErrors(t *testing.T) {
	now := time.Unix(7000, 0)
	account := oauthAccount(t)
	boom := errors.New("boom")

	t.Run("store read error", func(t *testing.T) {
		mgr := NewTokenManager(&fakeStore{getErr: boom}, Config{}, http.DefaultClient, fixedClock{t: now})
		if _, err := mgr.AccessToken(context.Background(), account); !errors.Is(err, boom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
	t.Run("stored blob unmarshal error", func(t *testing.T) {
		mgr := NewTokenManager(&fakeStore{secret: "junk"}, Config{}, http.DefaultClient, fixedClock{t: now})
		if _, err := mgr.AccessToken(context.Background(), account); err == nil {
			t.Error("expected an error unmarshalling a junk blob")
		}
	})
	t.Run("no refresh token", func(t *testing.T) {
		store := &fakeStore{secret: storedToken(t, Token{AccessToken: "stale", Expiry: now.Add(-time.Hour)})}
		mgr := NewTokenManager(store, Config{}, http.DefaultClient, fixedClock{t: now})
		if _, err := mgr.AccessToken(context.Background(), account); err == nil {
			t.Error("expected an error refreshing with no refresh token")
		}
	})
	t.Run("refresh endpoint error", func(t *testing.T) {
		srv := tokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`, nil)
		store := &fakeStore{secret: storedToken(t, Token{RefreshToken: "r", Expiry: now.Add(-time.Hour)})}
		mgr := NewTokenManager(store, Config{TokenURL: srv.URL}, srv.Client(), fixedClock{t: now})
		if _, err := mgr.AccessToken(context.Background(), account); err == nil {
			t.Error("expected an error when the refresh fails")
		}
	})
	t.Run("persist error", func(t *testing.T) {
		srv := tokenServer(t, http.StatusOK, `{"access_token":"fresh","expires_in":3600}`, nil)
		store := &fakeStore{secret: storedToken(t, Token{RefreshToken: "r", Expiry: now.Add(-time.Hour)}), setErr: boom}
		mgr := NewTokenManager(store, Config{TokenURL: srv.URL}, srv.Client(), fixedClock{t: now})
		if _, err := mgr.AccessToken(context.Background(), account); !errors.Is(err, boom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}
