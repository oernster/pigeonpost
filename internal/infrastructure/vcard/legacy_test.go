package vcard

import (
	"strings"
	"testing"
)

// TestDecodeQuotedPrintableKeepsWhatItCan: a dangling soft break at the very end of a value is an error
// to the standard reader; the text decoded before it is kept; a value with nothing decodable is
// kept raw rather than emptied.
func TestDecodeQuotedPrintableKeepsWhatItCan(t *testing.T) {
	if got := decodeQuotedPrintable("ab="); got != "ab" {
		t.Errorf("partial decode = %q, want %q", got, "ab")
	}
	if got := decodeQuotedPrintable("="); got != "=" {
		t.Errorf("undecodable value = %q, want it kept raw", got)
	}
}

// TestNormaliseLegacyBareBase64AndUnsplittableLine: a bare BASE64 token becomes an ENCODING parameter
// rather than a type; a line with no value separator passes through for go-vcard to judge.
func TestNormaliseLegacyBareBase64AndUnsplittableLine(t *testing.T) {
	out := string(normaliseLegacy([]byte("PHOTO;BASE64;JPEG:AAAA\r\nnocolon\r\n")))
	if !strings.Contains(out, "PHOTO;TYPE=JPEG;ENCODING=BASE64:AAAA") || !strings.Contains(out, "nocolon") {
		t.Errorf("normalised = %q", out)
	}
}

// TestNormaliseLegacyFoldedLines: a folded legacy line is unfolded before it is rewritten; a folded
// modern line is passed through exactly as it arrived.
func TestNormaliseLegacyFoldedLines(t *testing.T) {
	out := string(normaliseLegacy([]byte("TEL;WORK:+44 20\r\n 7946\r\nNOTE:long\r\n  text")))
	if out != "TEL;TYPE=WORK:+44 207946\r\nNOTE:long\r\n  text" {
		t.Errorf("normalised = %q", out)
	}
}

// TestNormaliseLegacySoftBreakAtEndOfInput: a quoted-printable value whose soft break is the last line
// of the file ends there instead of reading past the input.
func TestNormaliseLegacySoftBreakAtEndOfInput(t *testing.T) {
	out := string(normaliseLegacy([]byte("NOTE;QUOTED-PRINTABLE:tail=\r\nmore")))
	if out != "NOTE:tailmore" {
		t.Errorf("normalised = %q", out)
	}
}
