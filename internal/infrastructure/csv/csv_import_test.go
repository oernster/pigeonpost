package csv

import (
	"errors"
	"testing"
)

// TestDecodeSemicolonDelimited is the shape Excel and Outlook write on a locale whose decimal separator
// is a comma (most of continental Europe): the same headers, separated by semicolons.
func TestDecodeSemicolonDelimited(t *testing.T) {
	data := lines(
		"First Name;Last Name;E-mail Address;Company",
		"Jo;Bloggs;jo@example.com;\"Acme, Ltd\"",
	)
	got, skipped, err := New().DecodeImport(data)
	if err != nil {
		t.Fatalf("DecodeImport: %v", err)
	}
	if len(got) != 1 || skipped != 0 {
		t.Fatalf("decoded %d with %d skipped, want 1 and 0", len(got), skipped)
	}
	if got[0].FormattedName() != "Jo Bloggs" || got[0].Organization() != "Acme, Ltd" ||
		len(got[0].Emails()) != 1 {
		t.Errorf("contact = %+v", got[0])
	}
}

// TestDecodeTabDelimited covers a tab-separated export saved with a .csv name.
func TestDecodeTabDelimited(t *testing.T) {
	data := lines("Display Name\tE-mail Address", "Amy Pond\tamy@example.com")
	got, _, err := New().DecodeImport(data)
	if err != nil || len(got) != 1 || got[0].FormattedName() != "Amy Pond" {
		t.Fatalf("DecodeImport = %+v, %v", got, err)
	}
}

// TestDecodeQuotedDelimiterDoesNotDecide: a semicolon inside a quoted comma-separated header is part of
// the header's text, not a vote for the semicolon.
func TestDecodeQuotedDelimiterDoesNotDecide(t *testing.T) {
	// Four semicolons inside quotes against three bare commas: counting the quoted ones would pick the
	// semicolon and lose every column.
	data := lines("\"Display Name\",Notes,\"Department; Team; Unit; Floor; Desk\",E-mail Address",
		"Jo,a;b;c,,jo@example.com")
	got, _, err := New().DecodeImport(data)
	if err != nil || len(got) != 1 || got[0].Note() != "a;b;c" {
		t.Fatalf("DecodeImport = %+v, %v", got, err)
	}
}

// TestDecodeImportCountsRowsWithNoNameOrEmail: a row that carries data but neither a name nor an email
// cannot form a contact; it is skipped and counted. A wholly blank row is not a record and is not.
func TestDecodeImportCountsRowsWithNoNameOrEmail(t *testing.T) {
	data := lines(
		"First Name,Last Name,E-mail Address,Company",
		"Jo,Bloggs,jo@example.com,",
		",,,Orphan Company",
		",,,",
	)
	got, skipped, err := New().DecodeImport(data)
	if err != nil {
		t.Fatalf("DecodeImport: %v", err)
	}
	if len(got) != 1 || skipped != 1 {
		t.Errorf("decoded %d with %d skipped, want 1 and 1", len(got), skipped)
	}
}

// TestDecodeUnrecognisedHeadersIsAnError: a file with data rows whose header names no column this codec
// knows (a localised export, here German) must say so instead of reporting zero contacts as success.
func TestDecodeUnrecognisedHeadersIsAnError(t *testing.T) {
	data := lines("Vorname,Nachname,E-Mail-Adresse", "Jürgen,Müller,jurgen@example.com")
	got, _, err := New().DecodeImport(data)
	if !errors.Is(err, ErrNoRecognisedColumns) || len(got) != 0 {
		t.Fatalf("DecodeImport = %+v, %v; want ErrNoRecognisedColumns", got, err)
	}
}

// TestDecodeHeaderOnlyIsNoContacts: an export of an empty address book is a header and nothing else,
// which is a genuine empty result rather than an unreadable file.
func TestDecodeHeaderOnlyIsNoContacts(t *testing.T) {
	got, skipped, err := New().DecodeImport(lines("Vorname,Nachname"))
	if err != nil || len(got) != 0 || skipped != 0 {
		t.Errorf("DecodeImport = %+v, %d, %v; want nothing and no error", got, skipped, err)
	}
}
