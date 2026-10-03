package domain

import "strings"

// The two URL schemes a DAV base URL may carry. A scheme is case-insensitive (RFC 3986), so both are
// matched without regard to case.
const (
	secureCalendarScheme   = "https://"
	insecureCalendarScheme = "http://"
)

// hasSchemePrefix reports whether url starts with scheme, ignoring case.
func hasSchemePrefix(url, scheme string) bool {
	return len(url) >= len(scheme) && strings.EqualFold(url[:len(scheme)], scheme)
}

// IsInsecureCalendarURL reports whether a DAV base URL is plain http://, over which password (Basic)
// authentication sends the password unencrypted. Such a URL is accepted, since a server on the local
// machine or network is a real setup; the user is warned before relying on it.
func IsInsecureCalendarURL(baseURL string) bool {
	return hasSchemePrefix(strings.TrimSpace(baseURL), insecureCalendarScheme)
}

// CalendarAccount is a remote CalDAV or CardDAV account. Credentials are never held here; they live in the
// OS keychain and are referenced by the infrastructure layer, exactly as for a mail Account. It is kept
// separate from Account so a DAV host can differ from the mail host and the two evolve independently.
type CalendarAccount struct {
	id          string
	displayName string
	baseURL     string
	username    string
	auth        AuthMethod
}

// NewCalendarAccount validates and constructs a CalDAV/CardDAV account. baseURL must be an http or https
// URL (the endpoint discovery starts from) and the username identifies the principal.
func NewCalendarAccount(id, displayName, baseURL, username string, auth AuthMethod) (CalendarAccount, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return CalendarAccount{}, ErrEmptyAccountID
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return CalendarAccount{}, ErrEmptyDisplayName
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return CalendarAccount{}, ErrEmptyBaseURL
	}
	if !hasSchemePrefix(baseURL, secureCalendarScheme) && !hasSchemePrefix(baseURL, insecureCalendarScheme) {
		return CalendarAccount{}, ErrInvalidBaseURL
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return CalendarAccount{}, ErrEmptyUsername
	}
	return CalendarAccount{
		id:          id,
		displayName: displayName,
		baseURL:     baseURL,
		username:    username,
		auth:        auth,
	}, nil
}

// ID returns the account identifier.
func (a CalendarAccount) ID() string { return a.id }

// DisplayName returns the human-readable account name.
func (a CalendarAccount) DisplayName() string { return a.displayName }

// BaseURL returns the DAV server endpoint the account is discovered from.
func (a CalendarAccount) BaseURL() string { return a.baseURL }

// Username returns the account's login or principal name.
func (a CalendarAccount) Username() string { return a.username }

// Auth returns the authentication method (password or OAuth2).
func (a CalendarAccount) Auth() AuthMethod { return a.auth }

// IsInsecure reports whether the account talks to its server over plain http://, so its password travels
// unencrypted (see IsInsecureCalendarURL).
func (a CalendarAccount) IsInsecure() bool { return IsInsecureCalendarURL(a.baseURL) }
