package domain

import "strings"

const (
	minPort = 1
	maxPort = 65535
)

// Protocol is the mail retrieval protocol for an account.
type Protocol int

const (
	ProtocolIMAP Protocol = iota
	ProtocolPOP3
)

// String returns the lower-case protocol name.
func (p Protocol) String() string {
	switch p {
	case ProtocolIMAP:
		return "imap"
	case ProtocolPOP3:
		return "pop3"
	default:
		return "unknown"
	}
}

// Security is the transport security applied to a server connection.
type Security int

const (
	SecurityTLS      Security = iota // implicit TLS, e.g. ports 993/995/465
	SecurityStartTLS                 // upgrade a plaintext connection with STARTTLS
	SecurityNone                     // plaintext, discouraged
)

// String returns a stable identifier for the security mode.
func (s Security) String() string {
	switch s {
	case SecurityTLS:
		return "tls"
	case SecurityStartTLS:
		return "starttls"
	case SecurityNone:
		return "none"
	default:
		return "unknown"
	}
}

// AuthMethod is how credentials are presented to a server.
type AuthMethod int

const (
	AuthPassword AuthMethod = iota
	AuthOAuth2
)

// String returns a stable identifier for the auth method.
func (a AuthMethod) String() string {
	switch a {
	case AuthPassword:
		return "password"
	case AuthOAuth2:
		return "oauth2"
	default:
		return "unknown"
	}
}

// ServerConfig is a validated host/port/security triple for one endpoint.
type ServerConfig struct {
	host     string
	port     int
	security Security
}

// NewServerConfig validates and constructs a server endpoint configuration.
func NewServerConfig(host string, port int, security Security) (ServerConfig, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return ServerConfig{}, ErrEmptyHost
	}
	if port < minPort || port > maxPort {
		return ServerConfig{}, ErrInvalidPort
	}
	return ServerConfig{host: host, port: port, security: security}, nil
}

// Host returns the server host.
func (s ServerConfig) Host() string { return s.host }

// Port returns the server port.
func (s ServerConfig) Port() int { return s.port }

// Security returns the transport security mode.
func (s ServerConfig) Security() Security { return s.security }

// Account is a mail account. Credentials are never held here; they live in the OS keychain and are
// referenced separately by the infrastructure layer.
type Account struct {
	id          string
	displayName string
	address     EmailAddress
	protocol    Protocol
	incoming    ServerConfig
	outgoing    ServerConfig
	auth        AuthMethod
	signature   string
	identities  []EmailAddress
}

// NewAccount validates and constructs an account.
func NewAccount(
	id string,
	displayName string,
	address EmailAddress,
	protocol Protocol,
	incoming ServerConfig,
	outgoing ServerConfig,
	auth AuthMethod,
) (Account, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Account{}, ErrEmptyAccountID
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return Account{}, ErrEmptyDisplayName
	}
	if address.IsZero() {
		return Account{}, ErrEmptyEmailAddress
	}
	return Account{
		id:          id,
		displayName: displayName,
		address:     address,
		protocol:    protocol,
		incoming:    incoming,
		outgoing:    outgoing,
		auth:        auth,
	}, nil
}

// ID returns the account identifier.
func (a Account) ID() string { return a.id }

// DisplayName returns the human-readable account name.
func (a Account) DisplayName() string { return a.displayName }

// WithDisplayName returns a copy of the account carrying the given display name, validated non-empty as
// in NewAccount. It updates the sender name shown on outgoing mail without re-running server setup, so
// an OAuth account (whose credentials must not be re-verified with a password) can still be renamed.
func (a Account) WithDisplayName(displayName string) (Account, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return Account{}, ErrEmptyDisplayName
	}
	a.displayName = displayName
	return a, nil
}

// Address returns the account's email address.
func (a Account) Address() EmailAddress { return a.address }

// Protocol returns the retrieval protocol.
func (a Account) Protocol() Protocol { return a.protocol }

// Incoming returns the incoming (IMAP/POP3) server configuration.
func (a Account) Incoming() ServerConfig { return a.incoming }

// Outgoing returns the outgoing (SMTP) server configuration.
func (a Account) Outgoing() ServerConfig { return a.outgoing }

// Auth returns the authentication method.
func (a Account) Auth() AuthMethod { return a.auth }

// Signature returns the account's compose signature as HTML, empty when none is set.
func (a Account) Signature() string { return a.signature }

// WithSignature returns a copy of the account carrying the given HTML signature.
func (a Account) WithSignature(signature string) Account {
	a.signature = signature
	return a
}

// Identities returns a copy of the account's alternate sender addresses (aliases the account may send
// as, beyond its primary address), empty when none are configured.
func (a Account) Identities() []EmailAddress {
	return append([]EmailAddress(nil), a.identities...)
}

// WithIdentities returns a copy of the account carrying the given alternate sender addresses.
func (a Account) WithIdentities(identities []EmailAddress) Account {
	a.identities = append([]EmailAddress(nil), identities...)
	return a
}

// Senders returns every address the account is allowed to send as: its primary address first, then its
// configured identities in order.
func (a Account) Senders() []EmailAddress {
	out := make([]EmailAddress, 0, 1+len(a.identities))
	out = append(out, a.address)
	out = append(out, a.identities...)
	return out
}

// ResolveSender returns the account address matching the given address (case-insensitive) and whether one
// was found. An empty address resolves to the primary. It is how a chosen From is validated at send time:
// only an address the account owns (its primary or an identity) can be sent as, so a foreign From is
// rejected rather than forged.
func (a Account) ResolveSender(address string) (EmailAddress, bool) {
	address = strings.TrimSpace(address)
	if address == "" {
		return a.address, true
	}
	for _, sender := range a.Senders() {
		if strings.EqualFold(sender.Address(), address) {
			return sender, true
		}
	}
	return EmailAddress{}, false
}
