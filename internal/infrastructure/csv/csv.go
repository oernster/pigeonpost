// Package csv converts contacts to and from CSV, the format both Thunderbird and Outlook use for bulk
// address-book export and import. It implements the application ContactCodec port and depends only on
// the domain, the standard library and the text-encoding package.
//
// The two exporters differ in almost every detail: Outlook numbers its email slots and prefixes work
// columns "Business", Thunderbird names two emails and prefixes them "Work"; Thunderbird words its
// region and postal columns by locale (County and Post Code on a UK build, State and ZipCode on a US
// one); Outlook writes a birthday as one locale-formatted date, Thunderbird as three numeric columns.
// Columns are therefore matched by name against the aliases both conventions are known to use, in
// mapping.go, rather than by position. Input is normalised to UTF-8 first (see encoding.go), since
// neither exporter reliably writes it.
//
// CSV carries no stable per-contact id, so a decoded contact is given a generated one; reconciling a
// re-import against the existing address book is the application layer's job, which matches on email
// address rather than id for exactly this reason.
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

// exportHeader is the column order written on export. The names are the widely-recognised Outlook
// ones, which Outlook maps directly and Thunderbird's import wizard can map by hand, and which this
// codec's own aliases read back, so an export round-trips through any of the three.
var exportHeader = []string{
	"First Name", "Last Name", "Display Name", "Company", "Job Title", "Birthday",
	"E-mail Address", "E-mail 2 Address", "E-mail 3 Address",
	"Mobile Phone", "Home Phone", "Business Phone",
	"Home Street", "Home City", "Home State", "Home Postal Code", "Home Country",
	"Business Street", "Business City", "Business State", "Business Postal Code", "Business Country",
	"Notes",
}

// Codec is the CSV implementation of the application ContactCodec port.
type Codec struct{}

// New constructs a CSV codec.
func New() Codec { return Codec{} }

// Decode parses a CSV address book into contacts. The first row is the header; columns are matched by
// name against the known Outlook and Thunderbird conventions. Blank rows are skipped.
func (Codec) Decode(data []byte) ([]domain.Contact, error) {
	text, err := toUTF8(data)
	if err != nil {
		return nil, err
	}
	reader := stdcsv.NewReader(bytes.NewReader(text))
	// Both exporters emit ragged rows: Outlook omits trailing empty columns, and a hand-edited file
	// often gains or loses one, so the row length is not held to the header's.
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

// contactRow flattens a contact into the export columns. Emails beyond three, phones beyond the three
// slots and addresses beyond the two written here are dropped, since a flat CSV has nowhere to put
// them.
func contactRow(c domain.Contact) []string {
	emails := c.Emails()
	mobile, home, work := slotPhones(c.Phones())
	homeAddr, workAddr := slotAddresses(c.Addresses())
	return []string{
		c.GivenName(), c.FamilyName(), c.FormattedName(), c.Organization(), c.Title(), c.Birthday(),
		emailAt(emails, 0), emailAt(emails, 1), emailAt(emails, 2),
		mobile, home, work,
		homeAddr.Street(), homeAddr.Locality(), homeAddr.Region(), homeAddr.PostalCode(), homeAddr.Country(),
		workAddr.Street(), workAddr.Locality(), workAddr.Region(), workAddr.PostalCode(), workAddr.Country(),
		c.Note(),
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
	var used [3]bool
	for _, p := range phones {
		placeInSlot(slots[:], used[:], p.Number(), preferredSlot(p.Label()))
	}
	return slots[0], slots[1], slots[2]
}

// slotAddresses places addresses into the home and business column blocks by their label, falling back
// to the first free block for an unlabelled or overflowing address.
func slotAddresses(addresses []domain.ContactAddress) (home, work domain.ContactAddress) {
	var slots [2]domain.ContactAddress // 0 home, 1 business
	var used [2]bool
	for _, a := range addresses {
		placeInSlot(slots[:], used[:], a, addressSlot(a.Label()))
	}
	return slots[0], slots[1]
}

// preferredSlot returns the phone column a label belongs in, or -1 when the label says nothing.
func preferredSlot(label string) int {
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

// addressSlot returns the address block a label belongs in, or -1 when the label says nothing.
func addressSlot(label string) int {
	label = strings.ToLower(label)
	switch {
	case strings.Contains(label, "home"):
		return 0
	case strings.Contains(label, "work"), strings.Contains(label, "business"), strings.Contains(label, "office"):
		return 1
	default:
		return -1
	}
}

// placeInSlot writes a value into its preferred slot when that slot is free, and otherwise into the
// first free slot. A value with nowhere to go is dropped. Occupancy is tracked in a parallel slice
// rather than inferred from the slot's value, so the same placement rule serves both the phone columns
// and the address blocks.
func placeInSlot[T any](slots []T, used []bool, value T, preferred int) {
	if preferred >= 0 && !used[preferred] {
		slots[preferred], used[preferred] = value, true
		return
	}
	for i := range slots {
		if !used[i] {
			slots[i], used[i] = value, true
			return
		}
	}
}

// generatedID returns a random hex id for an imported row, since CSV carries no contact id.
func generatedID() string {
	var b [generatedIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "contact"
	}
	return hex.EncodeToString(b[:])
}
