package domain

import "strings"

// ContactEmail is one labelled email address on a contact (home, work, and so on). The label is
// optional free text, mirroring the vCard EMAIL TYPE parameter.
type ContactEmail struct {
	label   string
	address EmailAddress
}

// NewContactEmail validates and constructs a labelled email. The address must be a valid email; the
// label is optional.
func NewContactEmail(label, address string) (ContactEmail, error) {
	addr, err := NewEmailAddress("", address)
	if err != nil {
		return ContactEmail{}, err
	}
	return ContactEmail{label: strings.TrimSpace(label), address: addr}, nil
}

// Label returns the optional label, which may be empty.
func (e ContactEmail) Label() string { return e.label }

// Address returns the validated email address.
func (e ContactEmail) Address() EmailAddress { return e.address }

// ContactPhone is one labelled phone number on a contact. Numbers are kept as free text because phone
// formats vary too widely to validate usefully; only a non-empty value is required.
type ContactPhone struct {
	label  string
	number string
}

// NewContactPhone validates and constructs a labelled phone number. The number must be non-empty; the
// label is optional.
func NewContactPhone(label, number string) (ContactPhone, error) {
	number = strings.TrimSpace(number)
	if number == "" {
		return ContactPhone{}, ErrEmptyPhoneNumber
	}
	return ContactPhone{label: strings.TrimSpace(label), number: number}, nil
}

// Label returns the optional label, which may be empty.
func (p ContactPhone) Label() string { return p.label }

// Number returns the phone number.
func (p ContactPhone) Number() string { return p.number }

// ContactInput carries the fields for constructing a Contact. Only ID and FormattedName are required;
// the rest are optional.
type ContactInput struct {
	ID            string
	UID           string
	FormattedName string
	GivenName     string
	FamilyName    string
	Organization  string
	Title         string
	Note          string
	Emails        []ContactEmail
	Phones        []ContactPhone
}

// Contact is a single address-book entry. It is immutable once constructed; the slice getters return
// copies so callers cannot reach in and mutate its state. FormattedName maps to the vCard FN, which is
// mandatory, so it must be present; UID carries the vCard UID for a lossless round-trip and may be
// empty on a new contact (the store assigns one on save).
type Contact struct {
	id            string
	uid           string
	formattedName string
	givenName     string
	familyName    string
	organization  string
	title         string
	note          string
	emails        []ContactEmail
	phones        []ContactPhone
}

// NewContact validates and constructs a contact from its input.
func NewContact(in ContactInput) (Contact, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return Contact{}, ErrEmptyContactID
	}
	name := strings.TrimSpace(in.FormattedName)
	if name == "" {
		return Contact{}, ErrEmptyContactName
	}
	return Contact{
		id:            id,
		uid:           strings.TrimSpace(in.UID),
		formattedName: name,
		givenName:     strings.TrimSpace(in.GivenName),
		familyName:    strings.TrimSpace(in.FamilyName),
		organization:  strings.TrimSpace(in.Organization),
		title:         strings.TrimSpace(in.Title),
		note:          strings.TrimSpace(in.Note),
		emails:        cloneEmails(in.Emails),
		phones:        clonePhones(in.Phones),
	}, nil
}

// ID returns the local contact identifier.
func (c Contact) ID() string { return c.id }

// UID returns the vCard UID for round-trip, which may be empty.
func (c Contact) UID() string { return c.uid }

// FormattedName returns the display (vCard FN) name.
func (c Contact) FormattedName() string { return c.formattedName }

// GivenName returns the optional given (first) name.
func (c Contact) GivenName() string { return c.givenName }

// FamilyName returns the optional family (last) name.
func (c Contact) FamilyName() string { return c.familyName }

// Organization returns the optional organization.
func (c Contact) Organization() string { return c.organization }

// Title returns the optional job title.
func (c Contact) Title() string { return c.title }

// Note returns the optional free-text note.
func (c Contact) Note() string { return c.note }

// Emails returns a copy of the contact's labelled email addresses.
func (c Contact) Emails() []ContactEmail { return cloneEmails(c.emails) }

// Phones returns a copy of the contact's labelled phone numbers.
func (c Contact) Phones() []ContactPhone { return clonePhones(c.phones) }

// PrimaryEmail returns the first email address, or the zero EmailAddress when the contact has none.
func (c Contact) PrimaryEmail() EmailAddress {
	if len(c.emails) == 0 {
		return EmailAddress{}
	}
	return c.emails[0].address
}

// cloneEmails returns an independent copy of the emails, or nil when there are none, so a Contact never
// shares its backing array with a caller.
func cloneEmails(in []ContactEmail) []ContactEmail {
	if len(in) == 0 {
		return nil
	}
	out := make([]ContactEmail, len(in))
	copy(out, in)
	return out
}

// clonePhones returns an independent copy of the phones, or nil when there are none.
func clonePhones(in []ContactPhone) []ContactPhone {
	if len(in) == 0 {
		return nil
	}
	out := make([]ContactPhone, len(in))
	copy(out, in)
	return out
}
