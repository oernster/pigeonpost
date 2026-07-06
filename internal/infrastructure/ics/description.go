package ics

import (
	"html"
	"regexp"
	"strings"
)

// Real-world invites (Microsoft Teams, Outlook) put HTML into the iCalendar DESCRIPTION even though
// RFC 5545 defines it as plain text, so imported verbatim the tags show literally in the event. These
// expressions convert such a description back to readable text on import.
var (
	htmlTagRe    = regexp.MustCompile(`(?i)<[a-z/][^>]*>`)
	anchorRe     = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	brRe         = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockCloseRe = regexp.MustCompile(`(?i)</(?:p|div|li|tr|h[1-6])>`)
	blankLinesRe = regexp.MustCompile(`\n{3,}`)
)

// descriptionText converts an ICS text field that carries raw HTML into plain text. A value with no tags
// is returned unchanged, so an ordinary description is untouched. A link becomes "label (url)" so the
// target survives rather than being dropped with the tag, which keeps join-link detection working; a
// line-breaking tag becomes a newline and every other tag is stripped, with HTML entities decoded.
func descriptionText(s string) string {
	if !htmlTagRe.MatchString(s) {
		return s
	}
	out := anchorRe.ReplaceAllStringFunc(s, func(match string) string {
		groups := anchorRe.FindStringSubmatch(match)
		url := strings.TrimSpace(groups[1])
		label := strings.TrimSpace(stripTags(groups[2]))
		if label == "" || label == url {
			return url
		}
		return label + " (" + url + ")"
	})
	out = brRe.ReplaceAllString(out, "\n")
	out = blockCloseRe.ReplaceAllString(out, "\n")
	out = stripTags(out)
	out = html.UnescapeString(out)
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	out = blankLinesRe.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	return strings.TrimSpace(out)
}

// stripTags removes any remaining HTML tags from a fragment.
func stripTags(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}
