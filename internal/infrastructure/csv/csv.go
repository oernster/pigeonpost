// Package csv converts contacts to and from CSV, the format Outlook uses for bulk address-book export
// and import (Thunderbird reads and writes CSV too). It implements the application ContactCodec port
// and depends only on the domain and the standard library. CSV carries no stable per-contact id, so a
// decoded contact is given a generated id; a re-import therefore adds rather than reconciles.
package csv

import (
	"bytes"
	"crypto/rand"
	stdcsv "encoding/csv"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// generatedIDBytes is the length of the random id assigned to each imported CSV row.
const generatedIDBytes = 16

// exportHeader is the column order written on export. The headers are the widely-recognised Outlook
// names, which Outlook maps directly and Thunderbird's import wizard can map by hand.
var exportHeader = []string{
	"First Name", "Last Name", "Display Name", "Company", "Job Title",
	"E-mail Address", "E-mail 2 Address", "E-mail 3 Address",
	"Mobile Phone", "Home Phone", "Business Phone", "Notes",
}

// emailHeaders are the column names, lower-cased, read as email addresses on import (Outlook and
// Thunderbird conventions), in preference order.
var emailHeaders = []string{
	"e-mail address", "email address", "primary email", "e-mail 2 address", "email 2 address",
	"secondary email", "e-mail 3 address", "email 3 address", "e-mail", "email",
}

// phoneHeaders map a lower-cased column name to the label a matching value is imported under.
var phoneHeaders = []struct{ header, label string }{
	{"mobile phone", "mobile"}, {"mobile number", "mobile"}, {"cell phone", "mobile"},
	{"home phone", "home"}, {"home phone 2", "home"},
	{"business phone", "work"}, {"work phone", "work"}, {"business phone 2", "work"}, {"office phone", "work"},
}

// Codec is the CSV implementation of the application ContactCodec port.
type Codec struct{}

// New constructs a CSV codec.
func New() Codec { return Codec{} }

// Decode parses a CSV address book into contacts. The first row is the header; columns are matched by
// name against the known Outlook and Thunderbird conventions. Blank rows are skipped.
func (Codec) Decode(data []byte) ([]domain.Contact, error) {
	reader := stdcsv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: read: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	headers := headerIndex(records[0])
	var contacts []domain.Contact
	for _, row := range records[1:] {
		contact, ok, err := rowToContact(headers, row)
		if err != nil {
			return nil, err
		}
		if ok {
			contacts = append(contacts, contact)
		}
	}
	return contacts, nil
}

// rowToContact builds a contact from one data row. The bool is false for a blank/unusable row, which
// the caller skips.
func rowToContact(headers map[string]int, row []string) (domain.Contact, bool, error) {
	first := get(headers, row, "first name", "given name")
	last := get(headers, row, "last name", "family name", "surname")
	display := get(headers, row, "display name", "name", "formatted name")
	if display == "" {
		display = strings.TrimSpace(first + " " + last)
	}
	emails := collectEmails(headers, row)
	phones := collectPhones(headers, row)
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
		Organization:  get(headers, row, "company", "organization", "organisation"),
		Title:         get(headers, row, "job title", "title"),
		Note:          get(headers, row, "notes", "note"),
		Emails:        emails,
		Phones:        phones,
	})
	if err != nil {
		return domain.Contact{}, false, fmt.Errorf("csv: build contact: %w", err)
	}
	return contact, true, nil
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

// collectPhones reads every recognised phone column, labelling each by the column it came from.
func collectPhones(headers map[string]int, row []string) []domain.ContactPhone {
	var phones []domain.ContactPhone
	for _, ph := range phoneHeaders {
		i, ok := headers[ph.header]
		if !ok || i >= len(row) {
			continue
		}
		num := strings.TrimSpace(row[i])
		if num == "" {
			continue
		}
		phone, err := domain.NewContactPhone(ph.label, num)
		if err != nil {
			continue
		}
		phones = append(phones, phone)
	}
	return phones
}

// Encode writes the contacts as a CSV address book with the Outlook column headers.
func (Codec) Encode(contacts []domain.Contact) ([]byte, error) {
	var buf bytes.Buffer
	writer := stdcsv.NewWriter(&buf)
	if err := writer.Write(exportHeader); err != nil {
		return nil, fmt.Errorf("csv: write header: %w", err)
	}
	for _, c := range contacts {
		if err := writer.Write(contactRow(c)); err != nil {
			return nil, fmt.Errorf("csv: write %q: %w", c.ID(), err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("csv: flush: %w", err)
	}
	return buf.Bytes(), nil
}

// contactRow flattens a contact into the export columns. Emails beyond three and phones beyond the
// three slots are dropped, since a flat CSV has nowhere to put them.
func contactRow(c domain.Contact) []string {
	emails := c.Emails()
	mobile, home, work := slotPhones(c.Phones())
	return []string{
		c.GivenName(), c.FamilyName(), c.FormattedName(), c.Organization(), c.Title(),
		emailAt(emails, 0), emailAt(emails, 1), emailAt(emails, 2),
		mobile, home, work, c.Note(),
	}
}

// emailAt returns the address at index i, or an empty string when there is none.
func emailAt(emails []domain.ContactEmail, i int) string {
	if i < len(emails) {
		return emails[i].Address().Address()
	}
	return ""
}

// slotPhones places phones into the mobile, home and business columns by their label, falling back to
// the first free column for an unlabelled or overflowing phone.
func slotPhones(phones []domain.ContactPhone) (mobile, home, work string) {
	var slots [3]string // 0 mobile, 1 home, 2 work
	preferred := func(label string) int {
		label = strings.ToLower(label)
		switch {
		case strings.Contains(label, "mobile"), strings.Contains(label, "cell"):
			return 0
		case strings.Contains(label, "home"):
			return 1
		case strings.Contains(label, "work"), strings.Contains(label, "business"), strings.Contains(label, "office"):
			return 2
		default:
			return -1
		}
	}
	place := func(num string, pref int) {
		if pref >= 0 && slots[pref] == "" {
			slots[pref] = num
			return
		}
		for i := range slots {
			if slots[i] == "" {
				slots[i] = num
				return
			}
		}
	}
	for _, p := range phones {
		place(p.Number(), preferred(p.Label()))
	}
	return slots[0], slots[1], slots[2]
}

// generatedID returns a random hex id for an imported row, since CSV carries no contact id.
func generatedID() string {
	var b [generatedIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "contact"
	}
	return hex.EncodeToString(b[:])
}
