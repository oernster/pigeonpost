package csv

import (
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The codec must satisfy the application ContactCodec port.
var _ application.ContactCodec = Codec{}

func lines(rows ...string) []byte { return []byte(strings.Join(rows, "\r\n") + "\r\n") }

func TestCSVRoundTrip(t *testing.T) {
	e1, _ := domain.NewContactEmail("", "jo@work.example.com")
	e2, _ := domain.NewContactEmail("", "jo@home.example.com")
	mobile, _ := domain.NewContactPhone("mobile", "111")
	home, _ := domain.NewContactPhone("home", "222")
	work, _ := domain.NewContactPhone("work", "333")
	original, err := domain.NewContact(domain.ContactInput{
		ID: "c1", UID: "c1", FormattedName: "Jo Bloggs", GivenName: "Jo", FamilyName: "Bloggs",
		Organization: "Acme", Title: "Engineer", Note: "hello",
		Emails: []domain.ContactEmail{e1, e2}, Phones: []domain.ContactPhone{mobile, home, work},
	})
	if err != nil {
		t.Fatalf("contact: %v", err)
	}

	data, err := New().Encode([]domain.Contact{original})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	c := got[0]
	// CSV carries no id, so the id is regenerated; the rest must survive.
	if c.FormattedName() != "Jo Bloggs" || c.GivenName() != "Jo" || c.FamilyName() != "Bloggs" ||
		c.Organization() != "Acme" || c.Title() != "Engineer" || c.Note() != "hello" {
		t.Errorf("fields not preserved: %+v", c)
	}
	if len(c.Emails()) != 2 || c.Emails()[0].Address().Address() != "jo@work.example.com" {
		t.Errorf("emails = %+v", c.Emails())
	}
	labels := map[string]string{}
	for _, p := range c.Phones() {
		labels[p.Label()] = p.Number()
	}
	if labels["mobile"] != "111" || labels["home"] != "222" || labels["work"] != "333" {
		t.Errorf("phones by label = %v", labels)
	}
}

func TestDecodeOutlookHeaders(t *testing.T) {
	data := lines(
		"First Name,Last Name,Company,Job Title,E-mail Address,Mobile Phone,Notes",
		"Amy,Pond,Tardis,Companion,amy@example.com,555-0100,a note",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	c := got[0]
	if c.FormattedName() != "Amy Pond" || c.Organization() != "Tardis" || c.Title() != "Companion" {
		t.Errorf("mapping = %+v", c)
	}
	if len(c.Emails()) != 1 || c.Emails()[0].Address().Address() != "amy@example.com" {
		t.Errorf("email = %+v", c.Emails())
	}
	if len(c.Phones()) != 1 || c.Phones()[0].Label() != "mobile" || c.Phones()[0].Number() != "555-0100" {
		t.Errorf("phone = %+v", c.Phones())
	}
}

func TestDecodeThunderbirdHeaders(t *testing.T) {
	data := lines(
		"Display Name,Primary Email,Secondary Email",
		"Rory Williams,rory@example.com,rory2@example.com",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].FormattedName() != "Rory Williams" || len(got[0].Emails()) != 2 {
		t.Fatalf("thunderbird mapping failed: %+v", got)
	}
}

func TestDecodeNoNameUsesEmail(t *testing.T) {
	data := lines("E-mail Address", "solo@example.com")
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].FormattedName() != "solo@example.com" {
		t.Errorf("expected the email used as the name, got %+v", got)
	}
}

func TestDecodeSkipsBlankRows(t *testing.T) {
	data := lines(
		"First Name,Last Name,E-mail Address",
		"Jo,Bloggs,jo@example.com",
		",,",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected the blank row skipped, got %d contacts", len(got))
	}
}

func TestDecodeEmptyIsNoContacts(t *testing.T) {
	got, err := New().Decode(nil)
	if err != nil || len(got) != 0 {
		t.Errorf("Decode(nil) = %v, %v; want none and no error", got, err)
	}
}

func TestDecodeMalformedReturnsError(t *testing.T) {
	if _, err := New().Decode([]byte("First Name\r\n\"unterminated")); err == nil {
		t.Errorf("expected a read error for malformed CSV")
	}
}

func TestEncodePhoneOverflowUsesFreeColumn(t *testing.T) {
	// Two mobiles: the second has no free mobile slot, so it falls back to the next free column.
	m1, _ := domain.NewContactPhone("mobile", "111")
	m2, _ := domain.NewContactPhone("mobile", "222")
	c, _ := domain.NewContact(domain.ContactInput{
		ID: "c1", UID: "c1", FormattedName: "Two Mobiles", Phones: []domain.ContactPhone{m1, m2},
	})
	data, err := New().Encode([]domain.Contact{c})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The second mobile lands in the Home Phone column; on decode it is read back as a home phone.
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	numbers := map[string]string{}
	for _, p := range got[0].Phones() {
		numbers[p.Label()] = p.Number()
	}
	if numbers["mobile"] != "111" || numbers["home"] != "222" {
		t.Errorf("overflow phone not placed in a free column: %v", numbers)
	}
}

func TestEncodeDecodeAddressesRoundTrip(t *testing.T) {
	home, err := domain.NewContactAddress("home", "12 Leadworth Lane", "Leadworth", "Gloucestershire", "GL1 2AB", "UK")
	if err != nil {
		t.Fatalf("home address: %v", err)
	}
	work, err := domain.NewContactAddress("work", "1 Tardis Way", "London", "Greater London", "SW1A 1AA", "UK")
	if err != nil {
		t.Fatalf("work address: %v", err)
	}
	// Deliberately work-first, so a pass proves the blocks are chosen by label rather than by position.
	original, err := domain.NewContact(domain.ContactInput{
		ID: "c1", UID: "c1", FormattedName: "Jo Bloggs", Birthday: "1980-01-15",
		Addresses: []domain.ContactAddress{work, home},
	})
	if err != nil {
		t.Fatalf("contact: %v", err)
	}

	data, err := New().Encode([]domain.Contact{original})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	if got[0].Birthday() != "1980-01-15" {
		t.Errorf("birthday = %q, want it carried through the export", got[0].Birthday())
	}
	byLabel := addressByLabel(got[0])
	if a := byLabel["home"]; a.Street() != "12 Leadworth Lane" || a.PostalCode() != "GL1 2AB" {
		t.Errorf("home address = %+v", a)
	}
	if a := byLabel["work"]; a.Street() != "1 Tardis Way" || a.PostalCode() != "SW1A 1AA" {
		t.Errorf("work address = %+v", a)
	}
}

func TestEncodeUnlabelledEntriesUseTheFirstFreeSlot(t *testing.T) {
	// A label the slot rules do not recognise must still be exported, in whichever column is free.
	phone, err := domain.NewContactPhone("carrier pigeon", "555-9000")
	if err != nil {
		t.Fatalf("phone: %v", err)
	}
	address, err := domain.NewContactAddress("", "1 Nowhere Road", "", "", "", "")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	c, err := domain.NewContact(domain.ContactInput{
		ID: "c1", UID: "c1", FormattedName: "Odd Labels",
		Phones: []domain.ContactPhone{phone}, Addresses: []domain.ContactAddress{address},
	})
	if err != nil {
		t.Fatalf("contact: %v", err)
	}

	data, err := New().Encode([]domain.Contact{c})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// The first free phone column is Mobile Phone and the first free address block is Home.
	if len(got[0].Phones()) != 1 || got[0].Phones()[0].Number() != "555-9000" {
		t.Errorf("phones = %+v, want the oddly labelled number kept", got[0].Phones())
	}
	if a := addressByLabel(got[0])["home"]; a.Street() != "1 Nowhere Road" {
		t.Errorf("addresses = %+v, want the unlabelled address kept", got[0].Addresses())
	}
}

func TestDecodeIgnoresBlankAndInvalidColumns(t *testing.T) {
	// A blank header (a stray trailing comma on the header row) must not claim a column, and a value
	// that is not a usable email address must not sink the row.
	data := lines(
		"Display Name,,Primary Email,Secondary Email",
		"Jo Bloggs,ignored,not-an-email-address,jo@example.com",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	if len(got[0].Emails()) != 1 || got[0].Emails()[0].Address().Address() != "jo@example.com" {
		t.Errorf("emails = %+v, want only the valid address kept", got[0].Emails())
	}
}
