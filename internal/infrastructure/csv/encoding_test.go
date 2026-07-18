package csv

import (
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// bomRune is the byte-order mark, written as an escape because Go rejects a literal one in source.
const bomRune = "\uFEFF"

// encodeUTF16 builds a BOM-prefixed UTF-16 document by hand, so the test does not lean on the same
// encoding package the codec uses to decode it.
func encodeUTF16(s string, littleEndian bool) []byte {
	units := utf16.Encode([]rune(bomRune + s))
	out := make([]byte, 0, len(units)*2)
	for _, u := range units {
		if littleEndian {
			out = append(out, byte(u), byte(u>>8))
		} else {
			out = append(out, byte(u>>8), byte(u))
		}
	}
	return out
}

func TestDecodeWindows1252(t *testing.T) {
	// What Thunderbird actually wrote on a Western Windows build: the accented character is one high
	// byte, not a UTF-8 sequence. Left undecoded it reaches the store as invalid UTF-8.
	data := []byte("Display Name,Primary Email\r\nB\xe1rbara Barbizan,barbara@example.com\r\n")
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	if name := got[0].FormattedName(); name != "Bárbara Barbizan" {
		t.Errorf("name = %q, want the decoded accented form", name)
	}
	if !utf8.ValidString(got[0].FormattedName()) {
		t.Errorf("name is not valid UTF-8, so it would be stored and rendered as replacement characters")
	}
}

func TestDecodeUTF8BOMStillMatchesFirstColumn(t *testing.T) {
	// A BOM binds to the first header, so without stripping it "First Name" matches no alias and every
	// first name in the file is silently dropped. With no name and no email the row vanishes entirely.
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("First Name,Last Name\r\nAmy,Pond\r\n")...)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1: the BOM swallowed the first column", len(got))
	}
	if got[0].FormattedName() != "Amy Pond" || got[0].GivenName() != "Amy" {
		t.Errorf("BOM not stripped from the first header: %+v", got[0])
	}
}

func TestDecodeUTF16(t *testing.T) {
	for _, tc := range []struct {
		name         string
		littleEndian bool
	}{
		{"little endian", true},
		{"big endian", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := encodeUTF16("Display Name,Primary Email\r\nRory Williams,rory@example.com\r\n", tc.littleEndian)
			got, err := New().Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if len(got) != 1 || got[0].FormattedName() != "Rory Williams" {
				t.Errorf("utf-16 not decoded: %+v", got)
			}
		})
	}
}

func TestDecodeValidUTF8IsUnchanged(t *testing.T) {
	data := []byte("Display Name,Primary Email\r\nZoë Washburne,zoe@example.com\r\n")
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].FormattedName() != "Zoë Washburne" {
		t.Errorf("valid UTF-8 was mangled: %+v", got)
	}
}
