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
	if a.Signature() != "" {
		t.Errorf("default Signature = %q, want empty", a.Signature())
	}
}

func TestAccountWithSignature(t *testing.T) {
	addr, _ := NewEmailAddress("Me", "me@example.com")
	in := validServerConfig(t)
	out := validServerConfig(t)
	a, err := NewAccount("acc-1", "Personal", addr, ProtocolIMAP, in, out, AuthPassword)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	signed := a.WithSignature("<p>Best, Me</p>")
	if signed.Signature() != "<p>Best, Me</p>" {
		t.Errorf("Signature = %q", signed.Signature())
	}
	if a.Signature() != "" {
		t.Errorf("original account mutated: Signature = %q", a.Signature())
	}
}

func TestAccountWithDisplayName(t *testing.T) {
	addr, _ := NewEmailAddress("Me", "me@example.com")
	in := validServerConfig(t)
	out := validServerConfig(t)
	a, err := NewAccount("acc-1", "Personal", addr, ProtocolIMAP, in, out, AuthPassword)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	renamed, err := a.WithDisplayName("  New Name  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if renamed.DisplayName() != "New Name" {
		t.Errorf("DisplayName = %q, want New Name (trimmed)", renamed.DisplayName())
	}
	if a.DisplayName() != "Personal" {
		t.Errorf("original account mutated: DisplayName = %q", a.DisplayName())
	}

	if _, err := a.WithDisplayName("   "); !errors.Is(err, ErrEmptyDisplayName) {
		t.Errorf("WithDisplayName(blank) error = %v, want ErrEmptyDisplayName", err)
	}
}

func TestAccountIdentities(t *testing.T) {
	addr, _ := NewEmailAddress("Me", "me@example.com")
	alias, _ := NewEmailAddress("Alias", "me@alias.example")
	in := validServerConfig(t)
	out := validServerConfig(t)
	a, err := NewAccount("acc-1", "Personal", addr, ProtocolIMAP, in, out, AuthPassword)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(a.Identities()) != 0 {
		t.Errorf("default identities = %v, want none", a.Identities())
	}

	withAlias := a.WithIdentities([]EmailAddress{alias})
	if len(a.Identities()) != 0 {
		t.Error("WithIdentities mutated the original account")
	}
	got := withAlias.Identities()
	if len(got) != 1 || got[0].Address() != "me@alias.example" {
		t.Errorf("Identities = %v", got)
	}
	// Identities returns a copy: mutating it must not touch the account.
	got[0] = EmailAddress{}
	if withAlias.Identities()[0].Address() != "me@alias.example" {
		t.Error("Identities returned a live slice, not a copy")
	}

	senders := withAlias.Senders()
	if len(senders) != 2 || senders[0].Address() != "me@example.com" || senders[1].Address() != "me@alias.example" {
		t.Errorf("Senders = %v, want primary then alias", senders)
	}
}

func TestAccountResolveSender(t *testing.T) {
	addr, _ := NewEmailAddress("Me", "me@example.com")
	alias, _ := NewEmailAddress("Alias", "me@alias.example")
	in := validServerConfig(t)
	out := validServerConfig(t)
	base, _ := NewAccount("acc-1", "Personal", addr, ProtocolIMAP, in, out, AuthPassword)
	a := base.WithIdentities([]EmailAddress{alias})

	if got, ok := a.ResolveSender(""); !ok || got.Address() != "me@example.com" {
		t.Errorf("empty resolves to %v ok=%v, want primary", got, ok)
	}
	// Case-insensitive match against an identity, returning the stored address (with its display name).
	if got, ok := a.ResolveSender("ME@ALIAS.EXAMPLE"); !ok || got.Address() != "me@alias.example" || got.Display() != "Alias" {
		t.Errorf("identity resolves to %v ok=%v", got, ok)
	}
	if got, ok := a.ResolveSender("me@example.com"); !ok || got.Address() != "me@example.com" {
		t.Errorf("primary resolves to %v ok=%v", got, ok)
	}
	if _, ok := a.ResolveSender("stranger@nowhere.example"); ok {
		t.Error("a foreign address must not resolve")
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

func TestAccountSavesSentServerSide(t *testing.T) {
	build := func(outgoingHost string) Account {
		in, err := NewServerConfig("imap.example.com", 993, SecurityTLS)
		if err != nil {
			t.Fatalf("incoming: %v", err)
		}
		out, err := NewServerConfig(outgoingHost, 465, SecurityTLS)
		if err != nil {
			t.Fatalf("outgoing: %v", err)
		}
		account, err := NewAccount("id", "Me", mustAddr(t, "me@example.com"), ProtocolIMAP, in, out, AuthPassword)
		if err != nil {
			t.Fatalf("account: %v", err)
		}
		return account
	}
	if !build("smtp.gmail.com").SavesSentServerSide() {
		t.Error("gmail SMTP host should save sent server-side")
	}
	if !build("SMTP.GMAIL.COM").SavesSentServerSide() {
		t.Error("gmail host match should be case-insensitive")
	}
	if build("smtp.fastmail.com").SavesSentServerSide() {
		t.Error("a non-gmail host should not save sent server-side")
	}
}
