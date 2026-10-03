package csv

import "bytes"

// candidateDelimiters are the field separators an address-book CSV is known to use, in tie-break order.
// Comma is the default. Semicolon is what Excel and Outlook write on a locale whose decimal separator is
// a comma (most of continental Europe), where a comma-only reader finds one giant column and imports
// nothing. Tab appears when a tab-separated export is saved under a .csv name.
var candidateDelimiters = []rune{',', ';', '\t'}

// quoteChar opens and closes a quoted CSV field, inside which a delimiter is literal text.
const quoteChar = '"'

// detectDelimiter picks the separator from the header line: the candidate that occurs there most often
// outside quotes. The header is the line to judge because every exporter writes one and its column
// names hold no separators of their own unless quoted; a tie (including a single-column header) keeps
// the comma.
func detectDelimiter(text []byte) rune {
	header := text
	if end := bytes.IndexByte(text, '\n'); end >= 0 {
		header = text[:end]
	}
	best, bestCount := candidateDelimiters[0], 0
	for _, d := range candidateDelimiters {
		if n := countUnquoted(header, d); n > bestCount {
			best, bestCount = d, n
		}
	}
	return best
}

// countUnquoted counts the occurrences of an ASCII delimiter that sit outside double-quoted text.
func countUnquoted(line []byte, delimiter rune) int {
	count, inQuotes := 0, false
	for _, c := range line {
		switch {
		case c == quoteChar:
			inQuotes = !inQuotes
		case !inQuotes && rune(c) == delimiter:
			count++
		}
	}
	return count
}
