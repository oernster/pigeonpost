package domain

import (
	"errors"
	"testing"
)

func TestNewCalendarAccountValid(t *testing.T) {
	a, err := NewCalendarAccount(" acc1 ", " Fastmail ", " https://caldav.fastmail.com/ ", " user@fastmail.com ", AuthPassword)
	if err != nil {
		t.Fatalf("NewCalendarAccount: %v", err)
	}
	if a.ID() != "acc1" || a.DisplayName() != "Fastmail" || a.BaseURL() != "https://caldav.fastmail.com/" ||
		a.Username() != "user@fastmail.com" || a.Auth() != AuthPassword {
		t.Errorf("fields not set or trimmed as expected: %+v", a)
	}
}

func TestNewCalendarAccountValidation(t *testing.T) {
	const base = "https://dav.example.com"
	cases := []struct {
		name                   string
		id, display, url, user string
		want                   error
	}{
		{"empty id", "", "n", base, "u", ErrEmptyAccountID},
		{"empty display", "id", "  ", base, "u", ErrEmptyDisplayName},
		{"empty url", "id", "n", "", "u", ErrEmptyBaseURL},
		{"non-http url", "id", "n", "ftp://x", "u", ErrInvalidBaseURL},
		{"empty user", "id", "n", base, "  ", ErrEmptyUsername},
	}
	for _, c := range cases {
		if _, err := NewCalendarAccount(c.id, c.display, c.url, c.user, AuthPassword); !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestNewCalendarAccountAllowsHTTP(t *testing.T) {
	if _, err := NewCalendarAccount("id", "n", "http://localhost:5232", "u", AuthOAuth2); err != nil {
		t.Errorf("an http base url should be allowed: %v", err)
	}
}

func TestCalendarAccountIsInsecureOnlyOverPlainHTTP(t *testing.T) {
	// A plain http:// address is accepted (a local Radicale is a real setup) but flagged, since Basic auth
	// then sends the password unencrypted.
	cases := []struct {
		url  string
		want bool
	}{
		{"http://localhost:5232", true},
		{"HTTP://dav.example.com", true},
		{"https://caldav.fastmail.com", false},
		{"HTTPS://dav.example.com", false},
	}
	for _, c := range cases {
		if got := IsInsecureCalendarURL(c.url); got != c.want {
			t.Errorf("IsInsecureCalendarURL(%q) = %v, want %v", c.url, got, c.want)
		}
		a, err := NewCalendarAccount("id", "n", c.url, "u", AuthPassword)
		if err != nil {
			t.Fatalf("NewCalendarAccount(%q): %v", c.url, err)
		}
		if a.IsInsecure() != c.want {
			t.Errorf("account at %q IsInsecure = %v, want %v", c.url, a.IsInsecure(), c.want)
		}
	}
}
