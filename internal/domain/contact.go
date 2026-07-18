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

// ContactAddress is one labelled postal address on a contact. The label is optional free text,
// mirroring the vCard ADR TYPE parameter, and the five components map to the vCard ADR structured
// value (street, locality, region, postal code, country). Each component is optional on its own; only
// the address as a whole must carry something.
type ContactAddress struct {
	label      string
	street     string
	locality   string
	region     string
	postalCode string
	country    string
}

// NewContactAddress validates and constructs a labelled postal address. Every component is trimmed; the
// label is optional. An address whose components are all empty after trimming is rejected with
// ErrEmptyAddress so an empty row is never stored.
func NewContactAddress(label, street, locality, region, postalCode, country string) (ContactAddress, error) {
	street = strings.TrimSpace(street)
	locality = strings.TrimSpace(locality)
	region = strings.TrimSpace(region)
	postalCode = strings.TrimSpace(postalCode)
	country = strings.TrimSpace(country)
	if street == "" && locality == "" && region == "" && postalCode == "" && country == "" {
		return ContactAddress{}, ErrEmptyAddress
	}
	return ContactAddress{
		label:      strings.TrimSpace(label),
		street:     street,
		locality:   locality,
		region:     region,
		postalCode: postalCode,
		country:    country,
	}, nil
}

// Label returns the optional label, which may be empty.
func (a ContactAddress) Label() string { return a.label }

// Street returns the optional street component.
func (a ContactAddress) Street() string { return a.street }

// Locality returns the optional locality (city) component.
func (a ContactAddress) Locality() string { return a.locality }

// Region returns the optional region (state or province) component.
func (a ContactAddress) Region() string { return a.region }

// PostalCode returns the optional postal-code component.
func (a ContactAddress) PostalCode() string { return a.postalCode }

// Country returns the optional country component.
func (a ContactAddress) Country() string { return a.country }

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
	Birthday      string
	Emails        []ContactEmail
	Phones        []ContactPhone
	Addresses     []ContactAddress
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
	birthday      string
	emails        []ContactEmail
	phones        []ContactPhone
	addresses     []ContactAddress
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
		birthday:      strings.TrimSpace(in.Birthday),
		emails:        cloneEmails(in.Emails),
		phones:        clonePhones(in.Phones),
		addresses:     cloneAddresses(in.Addresses),
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

// Birthday returns the optional birthday as free text (vCard BDAY), which may be empty.
func (c Contact) Birthday() string { return c.birthday }

// Emails returns a copy of the contact's labelled email addresses.
func (c Contact) Emails() []ContactEmail { return cloneEmails(c.emails) }

// Phones returns a copy of the contact's labelled phone numbers.
func (c Contact) Phones() []ContactPhone { return clonePhones(c.phones) }

// Addresses returns a copy of the contact's labelled postal addresses.
func (c Contact) Addresses() []ContactAddress { return cloneAddresses(c.addresses) }

// PrimaryEmail returns the first email address, or the zero EmailAddress when the contact has none.
func (c Contact) PrimaryEmail() EmailAddress {
	if len(c.emails) == 0 {
		return EmailAddress{}
	}
	return c.emails[0].address
}

// MergedWith returns a copy of the contact enriched with anything the other contact carries that this
// one lacks. Identity (id, uid) and every non-empty scalar value on the receiver win, so merging an
// imported record into a stored one adds detail but never overwrites or removes what is already held.
// Emails, phones and addresses are unioned, the receiver's own entries keeping their position.
func (c Contact) MergedWith(other Contact) Contact {
	merged := c
	merged.formattedName = firstNonEmpty(c.formattedName, other.formattedName)
	merged.givenName = firstNonEmpty(c.givenName, other.givenName)
	merged.familyName = firstNonEmpty(c.familyName, other.familyName)
	merged.organization = firstNonEmpty(c.organization, other.organization)
	merged.title = firstNonEmpty(c.title, other.title)
	merged.note = firstNonEmpty(c.note, other.note)
	merged.birthday = firstNonEmpty(c.birthday, other.birthday)
	merged.emails = mergeEmails(c.emails, other.emails)
	merged.phones = mergePhones(c.phones, other.phones)
	merged.addresses = mergeAddresses(c.addresses, other.addresses)
	return merged
}

// firstNonEmpty returns a when it holds something, otherwise b.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// mergeEmails appends the entries of extra whose address is not already present, comparing addresses
// case-insensitively since email domains and most mailboxes are not case-sensitive in practice.
func mergeEmails(base, extra []ContactEmail) []ContactEmail {
	seen := make(map[string]bool, len(base))
	for _, e := range base {
		seen[strings.ToLower(e.address.Address())] = true
	}
	out := cloneEmails(base)
	for _, e := range extra {
		key := strings.ToLower(e.address.Address())
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out
}

// mergePhones appends the entries of extra whose number is not already present. Numbers are compared
// as stored rather than normalised, since phone formats vary too widely to normalise reliably; the
// label is not part of the key, so the same number under a different label is not added twice.
func mergePhones(base, extra []ContactPhone) []ContactPhone {
	seen := make(map[string]bool, len(base))
	for _, p := range base {
		seen[p.number] = true
	}
	out := clonePhones(base)
	for _, p := range extra {
		if seen[p.number] {
			continue
		}
		seen[p.number] = true
		out = append(out, p)
	}
	return out
}

// mergeAddresses appends the entries of extra that are not already present, an address being the same
// when all five of its structured components match case-insensitively. The label is not part of the
// key, so the same place under a different label is not added twice.
func mergeAddresses(base, extra []ContactAddress) []ContactAddress {
	seen := make(map[string]bool, len(base))
	for _, a := range base {
		seen[a.mergeKey()] = true
	}
	out := cloneAddresses(base)
	for _, a := range extra {
		key := a.mergeKey()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
	}
	return out
}

// mergeKey builds the comparison key for an address from its structured components, lower-cased and
// separated by a NUL so no component boundary can be forged by the values themselves.
func (a ContactAddress) mergeKey() string {
	return strings.ToLower(strings.Join(
		[]string{a.street, a.locality, a.region, a.postalCode, a.country}, "\x00"))
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

// cloneAddresses returns an independent copy of the addresses, or nil when there are none.
func cloneAddresses(in []ContactAddress) []ContactAddress {
	if len(in) == 0 {
		return nil
	}
	out := make([]ContactAddress, len(in))
	copy(out, in)
	return out
}
