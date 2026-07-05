package domain

import (
	"errors"
	"testing"
)

func mustAddress(t *testing.T, address string) EmailAddress {
	t.Helper()
	a, err := NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("NewEmailAddress(%q): %v", address, err)
	}
	return a
}

func TestParseRole(t *testing.T) {
	cases := map[string]struct {
		in   string
		want Role
	}{
		"empty defaults to required": {"", RoleRequired},
		"chair":                      {"CHAIR", RoleChair},
		"required":                   {"REQ-PARTICIPANT", RoleRequired},
		"optional lowercased":        {"opt-participant", RoleOptional},
		"non-participant padded":     {"  NON-PARTICIPANT  ", RoleNonParticipant},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseRole(c.in)
			if err != nil {
				t.Fatalf("ParseRole(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseRole(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
	if _, err := ParseRole("MODERATOR"); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("unknown role err = %v, want ErrInvalidRole", err)
	}
}

func TestParseParticipationStatus(t *testing.T) {
	cases := map[string]struct {
		in   string
		want ParticipationStatus
	}{
		"empty defaults to needs-action": {"", PartStatNeedsAction},
		"accepted lowercased":            {"accepted", PartStatAccepted},
		"declined":                       {"DECLINED", PartStatDeclined},
		"tentative padded":               {"  TENTATIVE ", PartStatTentative},
		"delegated":                      {"DELEGATED", PartStatDelegated},
		"needs-action explicit":          {"NEEDS-ACTION", PartStatNeedsAction},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseParticipationStatus(c.in)
			if err != nil {
				t.Fatalf("ParseParticipationStatus(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseParticipationStatus(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
	if _, err := ParseParticipationStatus("MAYBE"); !errors.Is(err, ErrInvalidParticipationStatus) {
		t.Errorf("unknown status err = %v, want ErrInvalidParticipationStatus", err)
	}
}

func TestNewOrganizer(t *testing.T) {
	addr := mustAddress(t, "chair@example.com")
	o, err := NewOrganizer(addr, "  Meeting Chair ")
	if err != nil {
		t.Fatalf("NewOrganizer: %v", err)
	}
	if o.Address().Address() != "chair@example.com" {
		t.Errorf("Address() = %q", o.Address().Address())
	}
	if o.CommonName() != "Meeting Chair" {
		t.Errorf("CommonName() = %q, want trimmed", o.CommonName())
	}
	if o.IsZero() {
		t.Errorf("organizer with an address reported zero")
	}
	if _, err := NewOrganizer(EmailAddress{}, "No Address"); !errors.Is(err, ErrEmptyOrganizerAddress) {
		t.Errorf("zero-address err = %v, want ErrEmptyOrganizerAddress", err)
	}
	if !(Organizer{}).IsZero() {
		t.Errorf("empty organizer not reported zero")
	}
}

func TestNewAttendee(t *testing.T) {
	addr := mustAddress(t, "guest@example.com")
	a, err := NewAttendee(AttendeeInput{
		Address: addr, CommonName: "  A Guest ", Role: RoleOptional, Status: PartStatAccepted, RSVP: true,
	})
	if err != nil {
		t.Fatalf("NewAttendee: %v", err)
	}
	if a.Address().Address() != "guest@example.com" || a.CommonName() != "A Guest" {
		t.Errorf("address/name wrong: %q %q", a.Address().Address(), a.CommonName())
	}
	if a.Role() != RoleOptional || a.Status() != PartStatAccepted || !a.RSVP() {
		t.Errorf("role/status/rsvp wrong: %q %q %v", a.Role(), a.Status(), a.RSVP())
	}
}

func TestNewAttendeeDefaultsAndValidation(t *testing.T) {
	addr := mustAddress(t, "guest@example.com")
	a, err := NewAttendee(AttendeeInput{Address: addr})
	if err != nil {
		t.Fatalf("NewAttendee: %v", err)
	}
	if a.Role() != RoleRequired {
		t.Errorf("default role = %q, want REQ-PARTICIPANT", a.Role())
	}
	if a.Status() != PartStatNeedsAction {
		t.Errorf("default status = %q, want NEEDS-ACTION", a.Status())
	}
	if a.RSVP() {
		t.Errorf("default rsvp = true, want false")
	}
	if _, err := NewAttendee(AttendeeInput{Address: EmailAddress{}}); !errors.Is(err, ErrEmptyAttendeeAddress) {
		t.Errorf("zero-address err = %v, want ErrEmptyAttendeeAddress", err)
	}
}

func TestAttendeeWithStatus(t *testing.T) {
	addr := mustAddress(t, "guest@example.com")
	a, err := NewAttendee(AttendeeInput{Address: addr, Status: PartStatNeedsAction})
	if err != nil {
		t.Fatalf("NewAttendee: %v", err)
	}
	updated := a.WithStatus(PartStatDeclined)
	if updated.Status() != PartStatDeclined {
		t.Errorf("WithStatus = %q, want DECLINED", updated.Status())
	}
	if a.Status() != PartStatNeedsAction {
		t.Errorf("WithStatus mutated the receiver: %q", a.Status())
	}
}
