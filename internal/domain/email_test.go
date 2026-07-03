package domain

import (
	"errors"
	"testing"
)

func TestNewEmailAddressValid(t *testing.T) {
	e, err := NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Local() != "user" {
		t.Errorf("Local = %q, want user", e.Local())
	}
	if e.Domain() != "example.com" {
		t.Errorf("Domain = %q, want example.com", e.Domain())
	}
	if e.Address() != "user@example.com" {
		t.Errorf("Address = %q", e.Address())
	}
	if e.Display() != "" {
		t.Errorf("Display = %q, want empty", e.Display())
	}
	if e.String() != "user@example.com" {
		t.Errorf("String = %q", e.String())
	}
	if e.IsZero() {
		t.Error("valid address reported IsZero")
	}
}

func TestNewEmailAddressWithDisplay(t *testing.T) {
	e, err := NewEmailAddress("  Jane Doe  ", "jane@example.co.uk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Display() != "Jane Doe" {
		t.Errorf("Display = %q, want trimmed Jane Doe", e.Display())
	}
	if e.String() != "Jane Doe <jane@example.co.uk>" {
		t.Errorf("String = %q", e.String())
	}
}

func TestNewEmailAddressInvalid(t *testing.T) {
	cases := map[string]struct {
		input string
		want  error
	}{
		"blank":            {"   ", ErrEmptyEmailAddress},
		"no at":            {"noat", ErrInvalidEmailAddress},
		"at at start":      {"@example.com", ErrInvalidEmailAddress},
		"empty domain":     {"local@", ErrInvalidEmailAddress},
		"double at":        {"a@b@c.com", ErrInvalidEmailAddress},
		"space in local":   {"a b@c.com", ErrInvalidEmailAddress},
		"space in domain":  {"a@c d.com", ErrInvalidEmailAddress},
		"no dot in domain": {"a@bcom", ErrInvalidEmailAddress},
		"leading dot":      {"a@.com", ErrInvalidEmailAddress},
		"trailing dot":     {"a@com.", ErrInvalidEmailAddress},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewEmailAddress("", tc.input)
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestEmailAddressZeroValue(t *testing.T) {
	var e EmailAddress
	if !e.IsZero() {
		t.Error("zero value should report IsZero")
	}
	if e.Address() != "" {
		t.Errorf("zero Address = %q, want empty", e.Address())
	}
	if e.String() != "" {
		t.Errorf("zero String = %q, want empty", e.String())
	}
}
