package domain

import (
	"errors"
	"testing"
)

func TestNewContactEmail(t *testing.T) {
	e, err := NewContactEmail("  work  ", "jo@example.com")
	if err != nil {
		t.Fatalf("NewContactEmail: %v", err)
	}
	if e.Label() != "work" {
		t.Errorf("label = %q, want work (trimmed)", e.Label())
	}
	if e.Address().Address() != "jo@example.com" {
		t.Errorf("address = %q", e.Address().Address())
	}
}

func TestNewContactEmailInvalid(t *testing.T) {
	if _, err := NewContactEmail("home", "not-an-email"); !errors.Is(err, ErrInvalidEmailAddress) {
		t.Errorf("err = %v, want ErrInvalidEmailAddress", err)
	}
}

func TestNewContactPhone(t *testing.T) {
	p, err := NewContactPhone("  mobile ", "  +44 7700 900000 ")
	if err != nil {
		t.Fatalf("NewContactPhone: %v", err)
	}
	if p.Label() != "mobile" {
		t.Errorf("label = %q, want mobile (trimmed)", p.Label())
	}
	if p.Number() != "+44 7700 900000" {
		t.Errorf("number = %q, want trimmed", p.Number())
	}
}

func TestNewContactPhoneEmpty(t *testing.T) {
	if _, err := NewContactPhone("home", "   "); !errors.Is(err, ErrEmptyPhoneNumber) {
		t.Errorf("err = %v, want ErrEmptyPhoneNumber", err)
	}
}

func TestNewContactAddress(t *testing.T) {
	a, err := NewContactAddress("  home ", "  1 High St ", "  London ", "  Greater London ", "  E1 1AA ", "  UK ")
	if err != nil {
		t.Fatalf("NewContactAddress: %v", err)
	}
	checks := map[string]string{
		a.Label():      "home",
		a.Street():     "1 High St",
		a.Locality():   "London",
		a.Region():     "Greater London",
		a.PostalCode(): "E1 1AA",
		a.Country():    "UK",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("component = %q, want %q (trimmed)", got, want)
		}
	}
}

func TestNewContactAddressPartialIsAllowed(t *testing.T) {
	// A single non-empty component (here just the country) is enough; the label may be empty.
	a, err := NewContactAddress("", "", "", "", "", "UK")
	if err != nil {
		t.Fatalf("NewContactAddress: %v", err)
	}
	if a.Country() != "UK" || a.Label() != "" || a.Street() != "" {
		t.Errorf("partial address not mapped: %+v", a)
	}
}

func TestNewContactAddressAllEmpty(t *testing.T) {
	if _, err := NewContactAddress("  home ", "  ", "", " ", "", "   "); !errors.Is(err, ErrEmptyAddress) {
		t.Errorf("err = %v, want ErrEmptyAddress", err)
	}
}

func TestNewContactStoresAndTrimsBirthday(t *testing.T) {
	c, err := NewContact(ContactInput{ID: "c1", FormattedName: "Jo", Birthday: "  1990-05-17 "})
	if err != nil {
		t.Fatalf("NewContact: %v", err)
	}
	if c.Birthday() != "1990-05-17" {
		t.Errorf("birthday = %q, want trimmed 1990-05-17", c.Birthday())
	}
}

func TestContactAddressesAreCopies(t *testing.T) {
	addr, _ := NewContactAddress("home", "1 High St", "London", "", "E1 1AA", "UK")
	in := []ContactAddress{addr}
	c, _ := NewContact(ContactInput{ID: "c1", FormattedName: "Jo", Addresses: in})

	// Mutating the input slice after construction must not change the contact.
	other, _ := NewContactAddress("work", "2 Low St", "Leeds", "", "LS1 1AA", "UK")
	in[0] = other
	if c.Addresses()[0].Street() != "1 High St" {
		t.Errorf("contact shares its backing array with the input slice")
	}
	// Mutating the returned slice must not change the contact either.
	got := c.Addresses()
	got[0] = other
	if c.Addresses()[0].Street() != "1 High St" {
		t.Errorf("contact shares its backing array with the returned slice")
	}
}

func TestContactAddressesNilWhenNone(t *testing.T) {
	c, err := NewContact(ContactInput{ID: "c1", FormattedName: "Jo"})
	if err != nil {
		t.Fatalf("NewContact: %v", err)
	}
	if c.Addresses() != nil {
		t.Errorf("expected nil addresses for a contact with none, got %+v", c.Addresses())
	}
}

func TestNewContactValidatesRequiredFields(t *testing.T) {
	if _, err := NewContact(ContactInput{ID: "  ", FormattedName: "Jo"}); !errors.Is(err, ErrEmptyContactID) {
		t.Errorf("blank id err = %v, want ErrEmptyContactID", err)
	}
	if _, err := NewContact(ContactInput{ID: "c1", FormattedName: "  "}); !errors.Is(err, ErrEmptyContactName) {
		t.Errorf("blank name err = %v, want ErrEmptyContactName", err)
	}
}

func TestNewContactTrimsAndExposesFields(t *testing.T) {
	email, _ := NewContactEmail("work", "jo@example.com")
	phone, _ := NewContactPhone("mobile", "12345")
	c, err := NewContact(ContactInput{
		ID:            "  c1  ",
		UID:           "  uid-1 ",
		FormattedName: "  Jo Bloggs ",
		GivenName:     " Jo ",
		FamilyName:    " Bloggs ",
		Organization:  " Acme ",
		Title:         " Engineer ",
		Note:          " a note ",
		Emails:        []ContactEmail{email},
		Phones:        []ContactPhone{phone},
	})
	if err != nil {
		t.Fatalf("NewContact: %v", err)
	}
	checks := map[string]string{
		c.ID():            "c1",
		c.UID():           "uid-1",
		c.FormattedName(): "Jo Bloggs",
		c.GivenName():     "Jo",
		c.FamilyName():    "Bloggs",
		c.Organization():  "Acme",
		c.Title():         "Engineer",
		c.Note():          "a note",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("field = %q, want %q", got, want)
		}
	}
	if len(c.Emails()) != 1 || c.Emails()[0].Address().Address() != "jo@example.com" {
		t.Errorf("emails = %+v", c.Emails())
	}
	if len(c.Phones()) != 1 || c.Phones()[0].Number() != "12345" {
		t.Errorf("phones = %+v", c.Phones())
	}
	if c.PrimaryEmail().Address() != "jo@example.com" {
		t.Errorf("primary email = %q", c.PrimaryEmail().Address())
	}
}

func TestContactPrimaryEmailWhenNone(t *testing.T) {
	c, err := NewContact(ContactInput{ID: "c1", FormattedName: "Jo"})
	if err != nil {
		t.Fatalf("NewContact: %v", err)
	}
	if !c.PrimaryEmail().IsZero() {
		t.Errorf("expected zero primary email, got %q", c.PrimaryEmail().Address())
	}
	if c.Emails() != nil || c.Phones() != nil {
		t.Errorf("expected nil slices for a contact with none, got %+v / %+v", c.Emails(), c.Phones())
	}
}

func TestContactSlicesAreCopies(t *testing.T) {
	email, _ := NewContactEmail("work", "jo@example.com")
	in := []ContactEmail{email}
	c, _ := NewContact(ContactInput{ID: "c1", FormattedName: "Jo", Emails: in})

	// Mutating the input slice after construction must not change the contact.
	other, _ := NewContactEmail("home", "x@example.com")
	in[0] = other
	if c.Emails()[0].Address().Address() != "jo@example.com" {
		t.Errorf("contact shares its backing array with the input slice")
	}
	// Mutating the returned slice must not change the contact either.
	got := c.Emails()
	got[0] = other
	if c.Emails()[0].Address().Address() != "jo@example.com" {
		t.Errorf("contact shares its backing array with the returned slice")
	}
}
