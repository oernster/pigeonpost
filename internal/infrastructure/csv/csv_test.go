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
