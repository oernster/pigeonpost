package csv

import (
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// streetLineSeparator joins an address's separate street-line columns into the single street component
// the domain holds.
const streetLineSeparator = ", "

// emailHeaders are the column names, lower-cased, read as email addresses on import, in preference
// order: Outlook numbers its three slots, Thunderbird names two.
var emailHeaders = []string{
	"e-mail address", "email address", "primary email", "e-mail 2 address", "email 2 address",
	"secondary email", "e-mail 3 address", "email 3 address", "e-mail", "email",
}

// phoneHeaders map a lower-cased column name to the label a matching value is imported under. Both
// exporters spread phone numbers across many columns; the ones with no natural home (car, radio,
// callback) are labelled "other" rather than dropped.
var phoneHeaders = []struct{ header, label string }{
	{"mobile phone", "mobile"}, {"mobile number", "mobile"}, {"cell phone", "mobile"}, {"mobile", "mobile"},
	{"home phone", "home"}, {"home phone 2", "home"},
	{"business phone", "work"}, {"business phone 2", "work"}, {"work phone", "work"},
	{"office phone", "work"}, {"company main phone", "work"},
	{"primary phone", "main"},
	{"other phone", "other"}, {"car phone", "other"}, {"radio phone", "other"}, {"callback", "other"},
	{"home fax", "fax"}, {"business fax", "fax"}, {"work fax", "fax"}, {"fax number", "fax"},
	{"pager", "pager"}, {"pager number", "pager"},
}

// addressSpec describes one postal-address block across both export conventions. Thunderbird prefixes
// its columns Home and Work and words the region and postal columns by locale (County and Post Code on
// a UK build, State and ZipCode on a US one); Outlook prefixes them Home, Business and Other.
type addressSpec struct {
	label string
	// streets are the discrete street-line columns, joined in order when present.
	streets []string
	// combined are the single-field address columns, used only when no street line carries anything.
	// Outlook exports both forms, its combined field repeating the city and region that have their own
	// columns, so preferring the street lines is what stops those being duplicated into the street.
	combined []string
	locality []string
	region   []string
	postal   []string
	country  []string
}

// addressSpecs are the address blocks read on import, in the order they are attached to a contact.
var addressSpecs = []addressSpec{
	{
		label:    "home",
		streets:  []string{"home street", "home street 2", "home street 3"},
		combined: []string{"home address", "home address 2"},
		locality: []string{"home city"},
		region:   []string{"home county", "home state", "home province", "home state/province"},
		postal:   []string{"home post code", "home postcode", "home postal code", "home zipcode", "home zip"},
		country:  []string{"home country", "home country/region"},
	},
	{
		label:    "work",
		streets:  []string{"business street", "business street 2", "business street 3", "work street"},
		combined: []string{"work address", "work address 2", "business address"},
		locality: []string{"work city", "business city"},
		region: []string{"work county", "work state", "work province", "business county", "business state",
			"business state/province"},
		postal: []string{"work post code", "work postcode", "work postal code", "work zipcode", "work zip",
			"business postal code", "business post code", "business zip"},
		country: []string{"work country", "work country/region", "business country", "business country/region"},
	},
	{
		label:    "other",
		streets:  []string{"other street", "other street 2", "other street 3"},
		combined: []string{"other address", "other address 2"},
		locality: []string{"other city"},
		region:   []string{"other county", "other state", "other province"},
		postal:   []string{"other post code", "other postcode", "other postal code", "other zipcode", "other zip"},
		country:  []string{"other country", "other country/region"},
	},
}

// headerIndex maps each lower-cased, trimmed header to its column index, keeping the first occurrence.
func headerIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key == "" {
			continue
		}
		if _, exists := m[key]; !exists {
			m[key] = i
		}
	}
	return m
}

// get returns the first non-empty value among the given header aliases.
func get(headers map[string]int, row []string, aliases ...string) string {
	for _, a := range aliases {
		if i, ok := headers[a]; ok && i < len(row) {
			if v := strings.TrimSpace(row[i]); v != "" {
				return v
			}
		}
	}
	return ""
}

// joinValues concatenates the non-empty values of the named columns, in the order given.
func joinValues(headers map[string]int, row []string, sep string, aliases []string) string {
	var present []string
	for _, a := range aliases {
		if i, ok := headers[a]; ok && i < len(row) {
			if v := strings.TrimSpace(row[i]); v != "" {
				present = append(present, v)
			}
		}
	}
	return strings.Join(present, sep)
}

// rowToContact builds a contact from one data row. The bool is false for a blank or unusable row,
// which the caller skips.
func rowToContact(headers map[string]int, row []string) (domain.Contact, bool, error) {
	first := get(headers, row, "first name", "given name")
	last := get(headers, row, "last name", "family name", "surname")
	display := displayName(headers, row, first, last)
	emails := collectEmails(headers, row)
	if display == "" && len(emails) == 0 {
		return domain.Contact{}, false, nil
	}
	if display == "" {
		display = emails[0].Address().Address()
	}
	id := generatedID()
	contact, err := domain.NewContact(domain.ContactInput{
		ID:            id,
		UID:           id,
		FormattedName: display,
		GivenName:     first,
		FamilyName:    last,
		Organization:  get(headers, row, "company", "organisation", "organization"),
		Title:         jobTitle(headers, row),
		Note:          get(headers, row, "notes", "note"),
		Birthday:      birthday(headers, row),
		Emails:        emails,
		Phones:        collectPhones(headers, row),
		Addresses:     collectAddresses(headers, row),
	})
	if err != nil {
		return domain.Contact{}, false, fmt.Errorf("csv: build contact: %w", err)
	}
	return contact, true, nil
}

// displayName returns the row's display name, falling back to the name parts when the file has no such
// column. Outlook has no display-name column at all, so the fallback is its normal path; the middle
// name is included there because Outlook exports one and dropping it would silently shorten names.
func displayName(headers map[string]int, row []string, first, last string) string {
	if name := get(headers, row, "display name", "name", "full name", "formatted name"); name != "" {
		return name
	}
	parts := []string{first, get(headers, row, "middle name"), last}
	var present []string
	for _, p := range parts {
		if p != "" {
			present = append(present, p)
		}
	}
	return strings.Join(present, " ")
}

// jobTitle reads the contact's role. Outlook's bare "Title" column is an honorific (Mr, Dr), not a job
// title, and Outlook exports both columns, so the bare one is read only when the file carries no
// job-title column at all: without that check an Outlook contact with no role recorded imports with a
// job title of "Mr".
func jobTitle(headers map[string]int, row []string) string {
	if _, ok := headers["job title"]; ok {
		return get(headers, row, "job title")
	}
	return get(headers, row, "title")
}

// birthday reads a birthday from either exporter's shape: Outlook's single date column, or
// Thunderbird's separate year, month and day columns.
func birthday(headers map[string]int, row []string) string {
	if value := get(headers, row, "birthday", "birth date", "date of birth"); value != "" {
		if iso := parseBirthday(value); iso != "" {
			return iso
		}
	}
	return birthdayFromParts(
		get(headers, row, "birth year"),
		get(headers, row, "birth month"),
		get(headers, row, "birth day"),
	)
}

// collectEmails reads every recognised email column, de-duplicating by address.
func collectEmails(headers map[string]int, row []string) []domain.ContactEmail {
	var emails []domain.ContactEmail
	seen := make(map[string]bool)
	for _, h := range emailHeaders {
		i, ok := headers[h]
		if !ok || i >= len(row) {
			continue
		}
		addr := strings.TrimSpace(row[i])
		if addr == "" || seen[strings.ToLower(addr)] {
			continue
		}
		email, err := domain.NewContactEmail("", addr)
		if err != nil {
			continue
		}
		seen[strings.ToLower(addr)] = true
		emails = append(emails, email)
	}
	return emails
}

// collectPhones reads every recognised phone column, labelling each by the column it came from and
// dropping repeats of a number already taken, which both exporters produce by writing the same number
// into more than one column.
func collectPhones(headers map[string]int, row []string) []domain.ContactPhone {
	var phones []domain.ContactPhone
	seen := make(map[string]bool)
	for _, ph := range phoneHeaders {
		i, ok := headers[ph.header]
		if !ok || i >= len(row) {
			continue
		}
		num := strings.TrimSpace(row[i])
		if num == "" || seen[num] {
			continue
		}
		phone, err := domain.NewContactPhone(ph.label, num)
		if err != nil {
			continue
		}
		seen[num] = true
		phones = append(phones, phone)
	}
	return phones
}

// collectAddresses reads each recognised address block. A block whose every component is empty is
// rejected by the domain and skipped.
func collectAddresses(headers map[string]int, row []string) []domain.ContactAddress {
	var addresses []domain.ContactAddress
	for _, spec := range addressSpecs {
		street := joinValues(headers, row, streetLineSeparator, spec.streets)
		if street == "" {
			street = joinValues(headers, row, streetLineSeparator, spec.combined)
		}
		address, err := domain.NewContactAddress(
			spec.label,
			street,
			get(headers, row, spec.locality...),
			get(headers, row, spec.region...),
			get(headers, row, spec.postal...),
			get(headers, row, spec.country...),
		)
		if err != nil {
			continue
		}
		addresses = append(addresses, address)
	}
	return addresses
}
