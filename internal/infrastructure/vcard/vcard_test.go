package vcard

import (
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The codec must satisfy the application ContactCodec port.
var _ application.ContactCodec = Codec{}

func card(lines ...string) []byte {
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func TestRoundTrip(t *testing.T) {
	e1, _ := domain.NewContactEmail("work", "jo@work.example.com")
	e2, _ := domain.NewContactEmail("home", "jo@home.example.com")
	ph, _ := domain.NewContactPhone("cell", "+441234567890")
	original, err := domain.NewContact(domain.ContactInput{
		ID: "uid-123", UID: "uid-123", FormattedName: "Jo Bloggs", GivenName: "Jo", FamilyName: "Bloggs",
		Organization: "Acme", Title: "Engineer", Note: "a note",
		Emails: []domain.ContactEmail{e1, e2}, Phones: []domain.ContactPhone{ph},
	})
	if err != nil {
		t.Fatalf("build contact: %v", err)
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
		t.Fatalf("decoded %d contacts, want 1", len(got))
	}
	c := got[0]
	if c.ID() != "uid-123" || c.UID() != "uid-123" || c.FormattedName() != "Jo Bloggs" {
		t.Errorf("identity not preserved: %+v", c)
	}
	if c.GivenName() != "Jo" || c.FamilyName() != "Bloggs" || c.Organization() != "Acme" ||
		c.Title() != "Engineer" || c.Note() != "a note" {
		t.Errorf("fields not preserved: %+v", c)
	}
	if len(c.Emails()) != 2 || c.Emails()[0].Address().Address() != "jo@work.example.com" ||
		c.Emails()[0].Label() != "work" || c.Emails()[1].Label() != "home" {
		t.Errorf("emails not preserved: %+v", c.Emails())
	}
	if len(c.Phones()) != 1 || c.Phones()[0].Number() != "+441234567890" || c.Phones()[0].Label() != "cell" {
		t.Errorf("phones not preserved: %+v", c.Phones())
	}
}

func TestDecodeThunderbirdStyleCard(t *testing.T) {
	data := card(
		"BEGIN:VCARD", "VERSION:4.0", "UID:urn:uuid:abc-123", "FN:Amy Pond", "N:Pond;Amy;;;",
		"EMAIL;TYPE=work:amy@example.com", "TEL;TYPE=cell:555-0100", "ORG:Tardis", "TITLE:Companion",
		"NOTE:hello", "END:VCARD",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	c := got[0]
	if c.ID() != "urn:uuid:abc-123" || c.FormattedName() != "Amy Pond" ||
		c.FamilyName() != "Pond" || c.GivenName() != "Amy" {
		t.Errorf("card not mapped: %+v", c)
	}
	if len(c.Emails()) != 1 || c.Emails()[0].Address().Address() != "amy@example.com" || c.Emails()[0].Label() != "work" {
		t.Errorf("email = %+v", c.Emails())
	}
	if len(c.Phones()) != 1 || c.Phones()[0].Number() != "555-0100" {
		t.Errorf("phone = %+v", c.Phones())
	}
	if c.Organization() != "Tardis" || c.Title() != "Companion" || c.Note() != "hello" {
		t.Errorf("org/title/note = %q/%q/%q", c.Organization(), c.Title(), c.Note())
	}
}

func TestDecodeNoUIDGeneratesID(t *testing.T) {
	data := card("BEGIN:VCARD", "VERSION:4.0", "FN:No Uid", "N:Uid;No;;;", "END:VCARD")
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].ID() == "" {
		t.Fatalf("expected one contact with a generated id, got %+v", got)
	}
	if got[0].UID() != got[0].ID() {
		t.Errorf("expected uid to equal the generated id, got id=%q uid=%q", got[0].ID(), got[0].UID())
	}
}

func TestDecodeSkipsInvalidEmail(t *testing.T) {
	data := card(
		"BEGIN:VCARD", "VERSION:4.0", "UID:x1", "FN:Bad Email",
		"EMAIL:not-an-email", "EMAIL;TYPE=work:good@example.com", "END:VCARD",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || len(got[0].Emails()) != 1 || got[0].Emails()[0].Address().Address() != "good@example.com" {
		t.Errorf("expected only the valid email kept, got %+v", got[0].Emails())
	}
}

func TestDecodeMalformedReturnsError(t *testing.T) {
	if _, err := New().Decode([]byte("BEGIN:VCARD\r\nnocolonhere\r\n")); err == nil {
		t.Errorf("expected a decode error for a malformed card")
	}
}

func TestDecodeEmptyIsNoContacts(t *testing.T) {
	got, err := New().Decode(nil)
	if err != nil || len(got) != 0 {
		t.Errorf("Decode(nil) = %v, %v; want no contacts and no error", got, err)
	}
}

func TestEncodeMinimalContactRoundTrips(t *testing.T) {
	// No organization, title or note, and an email with no label: exercises the "omit empty field"
	// and "no TYPE parameter" paths.
	email, _ := domain.NewContactEmail("", "min@example.com")
	c, err := domain.NewContact(domain.ContactInput{
		ID: "m1", UID: "m1", FormattedName: "Min Imal", Emails: []domain.ContactEmail{email},
	})
	if err != nil {
		t.Fatalf("contact: %v", err)
	}
	data, err := New().Encode([]domain.Contact{c})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := New().Decode(data)
	if err != nil || len(got) != 1 {
		t.Fatalf("Decode = %v, %v", got, err)
	}
	if got[0].Organization() != "" || got[0].Title() != "" || got[0].Note() != "" {
		t.Errorf("expected empty org/title/note, got %+v", got[0])
	}
	if len(got[0].Emails()) != 1 || got[0].Emails()[0].Label() != "" ||
		got[0].Emails()[0].Address().Address() != "min@example.com" {
		t.Errorf("email = %+v", got[0].Emails())
	}
}

func TestDecodeNamelessCardErrors(t *testing.T) {
	// A card with neither FN nor N cannot form a valid contact, so the decode fails rather than
	// silently dropping it.
	data := card("BEGIN:VCARD", "VERSION:4.0", "UID:x9", "END:VCARD")
	if _, err := New().Decode(data); err == nil {
		t.Errorf("expected an error decoding a nameless card")
	}
}

func TestDecodeSkipsEmptyPhone(t *testing.T) {
	data := card("BEGIN:VCARD", "VERSION:4.0", "UID:p1", "FN:Has Phone", "TEL:", "TEL;TYPE=cell:555", "END:VCARD")
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || len(got[0].Phones()) != 1 || got[0].Phones()[0].Number() != "555" {
		t.Errorf("expected only the non-empty phone, got %+v", got[0].Phones())
	}
}
