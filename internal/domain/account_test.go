package domain

import (
	"errors"
	"testing"
)

func TestProtocolString(t *testing.T) {
	cases := map[Protocol]string{
		ProtocolIMAP: "imap",
		ProtocolPOP3: "pop3",
		Protocol(99): "unknown",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Protocol(%d).String() = %q, want %q", p, got, want)
		}
	}
}

func TestSecurityString(t *testing.T) {
	cases := map[Security]string{
		SecurityTLS:      "tls",
		SecurityStartTLS: "starttls",
		SecurityNone:     "none",
		Security(99):     "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("Security(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestAuthMethodString(t *testing.T) {
	cases := map[AuthMethod]string{
		AuthPassword:   "password",
		AuthOAuth2:     "oauth2",
		AuthMethod(99): "unknown",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("AuthMethod(%d).String() = %q, want %q", a, got, want)
		}
	}
}

func TestNewServerConfig(t *testing.T) {
	sc, err := NewServerConfig("  imap.example.com  ", 993, SecurityTLS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sc.Host() != "imap.example.com" {
		t.Errorf("Host = %q", sc.Host())
	}
	if sc.Port() != 993 {
		t.Errorf("Port = %d", sc.Port())
	}
	if sc.Security() != SecurityTLS {
		t.Errorf("Security = %v", sc.Security())
	}
}

func TestNewServerConfigInvalid(t *testing.T) {
	if _, err := NewServerConfig("   ", 993, SecurityTLS); !errors.Is(err, ErrEmptyHost) {
		t.Errorf("empty host error = %v", err)
	}
	if _, err := NewServerConfig("h", 0, SecurityTLS); !errors.Is(err, ErrInvalidPort) {
		t.Errorf("low port error = %v", err)
	}
	if _, err := NewServerConfig("h", 70000, SecurityTLS); !errors.Is(err, ErrInvalidPort) {
		t.Errorf("high port error = %v", err)
	}
}

func validServerConfig(t *testing.T) ServerConfig {
	t.Helper()
	sc, err := NewServerConfig("host.example.com", 993, SecurityTLS)
	if err != nil {
		t.Fatalf("building server config: %v", err)
	}
	return sc
}

func TestNewAccount(t *testing.T) {
	addr, _ := NewEmailAddress("Me", "me@example.com")
	in := validServerConfig(t)
	out := validServerConfig(t)
	a, err := NewAccount(" acc-1 ", "  Personal  ", addr, ProtocolIMAP, in, out, AuthPassword)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID() != "acc-1" {
		t.Errorf("ID = %q", a.ID())
	}
	if a.DisplayName() != "Personal" {
		t.Errorf("DisplayName = %q", a.DisplayName())
	}
	if a.Address().Address() != "me@example.com" {
		t.Errorf("Address = %q", a.Address().Address())
	}
	if a.Protocol() != ProtocolIMAP {
		t.Errorf("Protocol = %v", a.Protocol())
	}
	if a.Incoming().Host() != "host.example.com" {
		t.Errorf("Incoming host = %q", a.Incoming().Host())
	}
	if a.Outgoing().Port() != 993 {
		t.Errorf("Outgoing port = %d", a.Outgoing().Port())
	}
	if a.Auth() != AuthPassword {
		t.Errorf("Auth = %v", a.Auth())
	}
}

func TestNewAccountInvalid(t *testing.T) {
	addr, _ := NewEmailAddress("Me", "me@example.com")
	sc := validServerConfig(t)

	if _, err := NewAccount("  ", "name", addr, ProtocolIMAP, sc, sc, AuthPassword); !errors.Is(err, ErrEmptyAccountID) {
		t.Errorf("empty id error = %v", err)
	}
	if _, err := NewAccount("id", "  ", addr, ProtocolIMAP, sc, sc, AuthPassword); !errors.Is(err, ErrEmptyDisplayName) {
		t.Errorf("empty name error = %v", err)
	}
	if _, err := NewAccount("id", "name", EmailAddress{}, ProtocolIMAP, sc, sc, AuthPassword); !errors.Is(err, ErrEmptyEmailAddress) {
		t.Errorf("zero address error = %v", err)
	}
}
