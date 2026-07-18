package csv

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Byte-order marks the two exporters are known to emit. Outlook writes a UTF-8 BOM from its newer
// export paths and a UTF-16 one when asked for Unicode text; Thunderbird writes none.
var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// toUTF8 normalises an exported address book to BOM-less UTF-8, which is what the rest of the codec,
// the store and the UI all assume.
//
// This matters because neither exporter reliably produces UTF-8. Outlook's classic CSV export uses the
// system ANSI code page (Windows-1252 on a Western install) and Thunderbird follows the platform
// encoding, so an accented name arrives as a single high byte rather than a UTF-8 sequence. Left alone
// those bytes are stored as invalid UTF-8 and render as replacement characters. A UTF-8 BOM is just as
// damaging in a different way: it is not whitespace, so it binds to the first header and turns
// "First Name" into a column no alias can match, silently dropping every first name in the file.
func toUTF8(data []byte) ([]byte, error) {
	switch {
	case bytes.HasPrefix(data, bomUTF8):
		return data[len(bomUTF8):], nil
	case bytes.HasPrefix(data, bomUTF16LE):
		return decodeUTF16(data, unicode.LittleEndian)
	case bytes.HasPrefix(data, bomUTF16BE):
		return decodeUTF16(data, unicode.BigEndian)
	case utf8.Valid(data):
		return data, nil
	default:
		// Not valid UTF-8 and no BOM to say what it is. Windows-1252 is the only realistic answer for a
		// Windows address-book export, and it is a superset of Latin-1, so it also covers the Linux and
		// macOS cases where a legacy file carries Latin-1.
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(data)
		if err != nil {
			return nil, fmt.Errorf("csv: decode windows-1252: %w", err)
		}
		return decoded, nil
	}
}

// decodeUTF16 converts UTF-16 input, BOM included, to UTF-8.
func decodeUTF16(data []byte, order unicode.Endianness) ([]byte, error) {
	decoded, err := unicode.UTF16(order, unicode.ExpectBOM).NewDecoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("csv: decode utf-16: %w", err)
	}
	return decoded, nil
}
