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
