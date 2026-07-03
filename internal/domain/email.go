package domain

import "strings"

// EmailAddress is a validated address with an optional display name. It is immutable once created.
type EmailAddress struct {
	display string
	local   string
	domain  string
}

// NewEmailAddress validates and constructs an address. The display name is optional and may be empty.
func NewEmailAddress(display, address string) (EmailAddress, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return EmailAddress{}, ErrEmptyEmailAddress
	}
	at := strings.LastIndex(address, "@")
	if at <= 0 {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	local := address[:at]
	domain := address[at+1:]
	if domain == "" {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	if strings.Contains(local, "@") {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	if strings.ContainsAny(local, " \t") || strings.ContainsAny(domain, " \t") {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	if !strings.Contains(domain, ".") {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	return EmailAddress{display: strings.TrimSpace(display), local: local, domain: domain}, nil
}

// Local returns the part before the @.
func (e EmailAddress) Local() string { return e.local }

// Domain returns the part after the @.
func (e EmailAddress) Domain() string { return e.domain }

// Display returns the optional display name, which may be empty.
func (e EmailAddress) Display() string { return e.display }

// Address returns the bare local@domain form.
func (e EmailAddress) Address() string {
	if e.IsZero() {
		return ""
	}
	return e.local + "@" + e.domain
}

// String returns "Display <local@domain>" when a display name is present, otherwise the bare address.
func (e EmailAddress) String() string {
	if e.display != "" {
		return e.display + " <" + e.Address() + ">"
	}
	return e.Address()
}

// IsZero reports whether this is the empty value.
func (e EmailAddress) IsZero() bool { return e.local == "" && e.domain == "" }
