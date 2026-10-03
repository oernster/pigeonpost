package vcard

import (
	"io"
	"mime/quotedprintable"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
)

// vCard 2.1 parameter vocabulary. Version 2.1 (what Outlook still writes for a single-contact export)
// allows a parameter to be a bare token: TEL;WORK;VOICE means TYPE=WORK,VOICE; a bare
// QUOTED-PRINTABLE means ENCODING=QUOTED-PRINTABLE. go-vcard only understands the KEY=VALUE form; given
// a bare token it consumes the property value as that token's value, which is why such a TEL or EMAIL
// used to arrive empty and be dropped.
const (
	paramEncoding   = "ENCODING"
	paramCharset    = "CHARSET"
	paramType       = "TYPE"
	encodingQP      = "QUOTED-PRINTABLE"
	charsetUTF8     = "UTF-8"
	softLineBreak   = "="
	paramSeparator  = ";"
	valueSeparator  = ":"
	paramAssignment = "="
	typeJoiner      = ","
	escapedNewline  = `\n`
	lineEnding      = "\r\n"
)

// bareEncodings are the bare 2.1 tokens that name an encoding rather than a type.
var bareEncodings = map[string]bool{encodingQP: true, "BASE64": true, "8BIT": true, "7BIT": true}

// newlineEscaper turns the real line breaks a quoted-printable value decodes to into the escaped form a
// vCard text value carries, so the decoded value stays on one logical line.
var newlineEscaper = strings.NewReplacer("\r\n", escapedNewline, "\r", escapedNewline, "\n", escapedNewline)

// normaliseLegacy rewrites vCard 2.1 constructs into the form go-vcard parses: bare parameters become
// TYPE= or ENCODING= ones, quoted-printable values are decoded (joining their soft line breaks) and a
// CHARSET other than UTF-8 is converted. A line that needs none of this is passed through byte for
// byte, so a 3.0 or 4.0 card is not touched.
func normaliseLegacy(data []byte) []byte {
	lines := strings.Split(strings.ReplaceAll(string(data), lineEnding, "\n"), "\n")
	out := make([]string, 0, len(lines))
	for start := 0; start < len(lines); {
		logical, end := joinLogicalLine(lines, start)
		if rewritten, ok := rewriteLegacyLine(logical); ok {
			out = append(out, rewritten)
		} else {
			out = append(out, lines[start:end]...)
		}
		start = end
	}
	return []byte(strings.Join(out, lineEnding))
}

// joinLogicalLine unfolds the content line starting at lines[start] and returns it with the index just
// past its last physical line. A line continues on the next when that one is folded (starts with
// whitespace). A quoted-printable value also continues when it ends in a soft line break.
func joinLogicalLine(lines []string, start int) (string, int) {
	logical := lines[start]
	qp := isQuotedPrintable(logical)
	end := start + 1
	for ; end < len(lines); end++ {
		next := lines[end]
		switch {
		case qp && strings.HasSuffix(logical, softLineBreak):
			// Some writers also fold a soft break with leading whitespace, which is no part of the value.
			logical = strings.TrimSuffix(logical, softLineBreak) + trimFoldIndent(next)
		case isFolded(next):
			logical += next[1:]
		default:
			return logical, end
		}
	}
	return logical, end
}

// isFolded reports whether a physical line continues the previous one (RFC 6350 section 3.2).
func isFolded(s string) bool {
	return s != "" && (s[0] == ' ' || s[0] == '\t')
}

// trimFoldIndent removes the single whitespace character a folded line starts with, if any.
func trimFoldIndent(s string) string {
	if isFolded(s) {
		return s[1:]
	}
	return s
}

// splitProperty separates a content line into its name, its raw parameters and its value, honouring
// double-quoted parameter values (which may hold a colon or semicolon). ok is false for a line with no
// value separator, which is left for go-vcard to judge.
func splitProperty(line string) (name string, params []string, value string, ok bool) {
	inQuotes := false
	fieldStart := 0
	var parts []string
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '"':
			inQuotes = !inQuotes
		case inQuotes:
		case c == paramSeparator[0]:
			parts = append(parts, line[fieldStart:i])
			fieldStart = i + 1
		case c == valueSeparator[0]:
			parts = append(parts, line[fieldStart:i])
			return parts[0], parts[1:], line[i+1:], true
		}
	}
	return "", nil, "", false
}

// isQuotedPrintable reports whether a line's parameters declare a quoted-printable value.
func isQuotedPrintable(line string) bool {
	_, params, _, ok := splitProperty(line)
	if !ok {
		return false
	}
	for _, p := range params {
		if isQuotedPrintableParam(p) {
			return true
		}
	}
	return false
}

// isQuotedPrintableParam reports whether one raw parameter declares quoted-printable, in either the
// bare 2.1 form or the ENCODING= form.
func isQuotedPrintableParam(p string) bool {
	key, val, assigned := strings.Cut(p, paramAssignment)
	if !assigned {
		return strings.EqualFold(key, encodingQP)
	}
	return strings.EqualFold(key, paramEncoding) && strings.EqualFold(val, encodingQP)
}

// rewriteLegacyLine rebuilds one unfolded line in modern form. ok is false when the line carries no
// bare parameter, no quoted-printable value and no CHARSET, so the caller keeps the original bytes.
func rewriteLegacyLine(line string) (string, bool) {
	name, params, value, ok := splitProperty(line)
	if !ok {
		return "", false
	}
	var types, kept []string
	qp, charset, legacy := false, "", false
	for _, p := range params {
		key, val, assigned := strings.Cut(p, paramAssignment)
		upper := strings.ToUpper(key)
		switch {
		case isQuotedPrintableParam(p):
			qp, legacy = true, true
		case !assigned && bareEncodings[upper]:
			kept, legacy = append(kept, paramEncoding+paramAssignment+upper), true
		case !assigned:
			types, legacy = append(types, key), true
		case upper == paramCharset:
			charset, legacy = val, true
		case upper == paramType:
			types = append(types, val)
		default:
			kept = append(kept, p)
		}
	}
	if !legacy {
		return "", false
	}
	if qp {
		value = decodeQuotedPrintable(value)
	}
	value = newlineEscaper.Replace(toUTF8(value, charset))
	head := []string{name}
	if len(types) > 0 {
		head = append(head, paramType+paramAssignment+strings.Join(types, typeJoiner))
	}
	head = append(head, kept...)
	return strings.Join(head, paramSeparator) + valueSeparator + value, true
}

// decodeQuotedPrintable decodes a quoted-printable value whose soft breaks are already joined. The
// standard reader is lenient about stray equals signs, so a malformed escape is kept literally rather
// than losing the value; what it has decoded up to an error is kept for the same reason.
func decodeQuotedPrintable(value string) string {
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(value)))
	if err != nil && len(decoded) == 0 {
		return value
	}
	return string(decoded)
}

// toUTF8 converts a value from the named charset to UTF-8. An empty, UTF-8 or unrecognised charset
// leaves the bytes as they are: an unknown label cannot be decoded; keeping the text readable where
// it is ASCII is better than dropping the contact.
func toUTF8(value, charset string) string {
	if charset == "" || strings.EqualFold(charset, charsetUTF8) {
		return value
	}
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return value
	}
	converted, err := enc.NewDecoder().String(value)
	if err != nil {
		return value
	}
	return converted
}
