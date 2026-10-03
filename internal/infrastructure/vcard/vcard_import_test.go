package vcard

import (
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// decodeOne runs DecodeImport and requires exactly one contact and no skipped record.
func decodeOne(t *testing.T, data []byte) []domain.Contact {
	t.Helper()
	got, skipped, err := New().DecodeImport(data)
	if err != nil {
		t.Fatalf("DecodeImport: %v", err)
	}
	if len(got) != 1 || skipped != 0 {
		t.Fatalf("decoded %d contacts with %d skipped, want 1 and 0", len(got), skipped)
	}
	return got
}

// TestDecodeOutlookQuotedPrintableCard is the shape Outlook writes for a single-contact vCard 2.1
// export: Windows-1252 text in quoted-printable, a soft line break and bare type parameters.
func TestDecodeOutlookQuotedPrintableCard(t *testing.T) {
	data := card(
		"BEGIN:VCARD", "VERSION:2.1",
		"N;LANGUAGE=de;CHARSET=Windows-1252;ENCODING=QUOTED-PRINTABLE:M=FCller;J=FCrgen",
		"FN;CHARSET=Windows-1252;ENCODING=QUOTED-PRINTABLE:J=FCrgen M=FCller",
		"NOTE;ENCODING=QUOTED-PRINTABLE:First line=0D=0ASecond line that Outlook =",
		"breaks softly",
		"TEL;WORK;VOICE:+44 20 7946 0000",
		"EMAIL;PREF;INTERNET:jurgen@example.com",
		"END:VCARD",
	)
	got := decodeOne(t, data)
	c := got[0]
	if c.FormattedName() != "Jürgen Müller" || c.GivenName() != "Jürgen" || c.FamilyName() != "Müller" {
		t.Errorf("name left encoded: fn=%q given=%q family=%q", c.FormattedName(), c.GivenName(), c.FamilyName())
	}
	if c.Note() != "First line\nSecond line that Outlook breaks softly" {
		t.Errorf("note = %q", c.Note())
	}
	if len(c.Phones()) != 1 || c.Phones()[0].Number() != "+44 20 7946 0000" || c.Phones()[0].Label() != "WORK" {
		t.Errorf("TEL;WORK;VOICE not kept: %+v", c.Phones())
	}
	if len(c.Emails()) != 1 || c.Emails()[0].Address().Address() != "jurgen@example.com" ||
		c.Emails()[0].Label() != "" {
		t.Errorf("EMAIL;PREF;INTERNET not kept: %+v", c.Emails())
	}
}

// TestDecodeQuotedPrintableUTF8AndBareEncoding covers a UTF-8 charset and the 2.1 form in which the
// encoding itself is a bare parameter rather than ENCODING=.
func TestDecodeQuotedPrintableUTF8AndBareEncoding(t *testing.T) {
	data := card(
		"BEGIN:VCARD", "VERSION:2.1",
		"FN;CHARSET=UTF-8;QUOTED-PRINTABLE:Ren=C3=A9e Dupont",
		// A writer that folds its soft break with a leading space as well as the trailing equals sign.
		"ORG;ENCODING=QUOTED-PRINTABLE:Soci=C3=A9t=C3=A9=",
		" G=C3=A9n=C3=A9rale",
		"END:VCARD",
	)
	got := decodeOne(t, data)
	if got[0].FormattedName() != "Renée Dupont" {
		t.Errorf("fn = %q", got[0].FormattedName())
	}
	if got[0].Organization() != "SociétéGénérale" {
		t.Errorf("org = %q", got[0].Organization())
	}
}

// TestDecodeUnknownCharsetKeepsBytes pins the fallback: a charset the decoder does not know leaves the
// decoded bytes as they are rather than failing the whole card.
func TestDecodeUnknownCharsetKeepsBytes(t *testing.T) {
	data := card("BEGIN:VCARD", "VERSION:2.1", "FN;CHARSET=x-made-up;ENCODING=QUOTED-PRINTABLE:Plain=20Name", "END:VCARD")
	got := decodeOne(t, data)
	if got[0].FormattedName() != "Plain Name" {
		t.Errorf("fn = %q", got[0].FormattedName())
	}
}

// TestDecodeModernQuotedParamUntouched guards the legacy rewrite against damaging a vCard 4.0 line
// whose parameter value is quoted and holds a comma.
func TestDecodeModernQuotedParamUntouched(t *testing.T) {
	data := card(
		"BEGIN:VCARD", "VERSION:4.0", "UID:m4", "FN:Modern Card",
		"TEL;VALUE=uri;TYPE=\"home,voice\":tel:+1-555-0100",
		"END:VCARD",
	)
	got := decodeOne(t, data)
	if len(got[0].Phones()) != 1 || got[0].Phones()[0].Label() != "home" || got[0].Phones()[0].Number() != "tel:+1-555-0100" {
		t.Errorf("phones = %+v", got[0].Phones())
	}
}

// TestDecodeImportSkipsAndCountsNamelessCard: one card with neither FN nor N must not abort the import
// of the cards around it; it is skipped and counted so the user can be told.
func TestDecodeImportSkipsAndCountsNamelessCard(t *testing.T) {
	data := card(
		"BEGIN:VCARD", "VERSION:4.0", "UID:a1", "FN:Before", "END:VCARD",
		"BEGIN:VCARD", "VERSION:4.0", "UID:x9", "EMAIL:nobody@example.com", "END:VCARD",
		"BEGIN:VCARD", "VERSION:4.0", "UID:a2", "FN:After", "END:VCARD",
	)
	got, skipped, err := New().DecodeImport(data)
	if err != nil {
		t.Fatalf("DecodeImport aborted: %v", err)
	}
	if len(got) != 2 || skipped != 1 || got[0].FormattedName() != "Before" || got[1].FormattedName() != "After" {
		t.Errorf("got %d contacts, %d skipped; want 2 and 1", len(got), skipped)
	}
}

// TestDecodeImportMalformedReturnsError: a structurally broken file is still an error, not a skip.
func TestDecodeImportMalformedReturnsError(t *testing.T) {
	if _, _, err := New().DecodeImport([]byte("BEGIN:VCARD\r\nnocolonhere\r\n")); err == nil {
		t.Errorf("expected a decode error for a malformed card")
	}
}
