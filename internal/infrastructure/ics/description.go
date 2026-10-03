package ics

import (
	"html"
	"regexp"
	"strings"

	goical "github.com/emersion/go-ical"
)

const (
	// propTeamsMeetingURL is the Microsoft Teams join link, shipped as a non-standard property Outlook
	// adds to a Teams meeting invite.
	propTeamsMeetingURL = "X-MICROSOFT-SKYPETEAMSMEETINGURL"
	// propAltDesc is the HTML alternative description Microsoft ships alongside (or instead of) the plain
	// DESCRIPTION, carried as X-ALT-DESC with an FMTTYPE=text/html parameter.
	propAltDesc = "X-ALT-DESC"
)

// Real-world invites (Microsoft Teams, Outlook) put HTML into the iCalendar DESCRIPTION even though
// RFC 5545 defines it as plain text, so imported verbatim the tags show literally in the event. These
// expressions convert such a description back to readable text on import.
//
// Deciding that a value IS HTML is deliberately stricter than stripping it. Plain text routinely holds
// angle brackets that are not markup: an address written as John <john@x.com>, an arrow, a comparison.
// A value only counts as HTML when it holds a tag naming a real HTML element (a, b, br, div, p, span, td
// and the like) followed by whitespace, a slash or the closing bracket; <john@x.com> names no element
// and is followed by "@", so it is left alone. Once a value is known to be HTML every tag is stripped.
var (
	htmlMarkupRe = regexp.MustCompile(`(?i)</?(?:a|abbr|b|blockquote|body|br|center|code|div|em|font|h[1-6]|` +
		`head|hr|html|i|img|li|meta|ol|p|pre|s|small|span|strong|style|sub|sup|table|tbody|td|th|thead|title|` +
		`tr|u|ul)(?:\s[^>]*)?/?>`)
	htmlTagRe    = regexp.MustCompile(`(?i)<[a-z/][^>]*>`)
	anchorRe     = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	brRe         = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockCloseRe = regexp.MustCompile(`(?i)</(?:p|div|li|tr|h[1-6])>`)
	blankLinesRe = regexp.MustCompile(`\n{3,}`)
)

// eventDescription builds the readable description for an imported event, hardened for real-world Teams
// and Outlook invites. It starts from the plain DESCRIPTION, falls back to the HTML X-ALT-DESC (which
// Microsoft ships when DESCRIPTION is empty) converted to text; it then appends the Teams join URL from
// X-MICROSOFT-SKYPETEAMSMEETINGURL when the description does not already contain it, so the join link
// survives import rather than being dropped with the properties PigeonPost did not model before.
func eventDescription(props goical.Props) string {
	desc := descriptionText(text(props, goical.PropDescription))
	if strings.TrimSpace(desc) == "" {
		desc = descriptionText(text(props, propAltDesc))
	}
	joinURL := strings.TrimSpace(text(props, propTeamsMeetingURL))
	if joinURL != "" && !strings.Contains(desc, joinURL) {
		if strings.TrimSpace(desc) == "" {
			desc = joinURL
		} else {
			desc = desc + "\n\n" + joinURL
		}
	}
	return desc
}

// descriptionText converts an ICS text field that carries raw HTML into plain text. A value with no real
// HTML element tag is returned unchanged, so an ordinary description (angle-bracketed addresses included)
// is untouched. A link becomes "label (url)" so the target survives rather than being dropped with the
// tag, which keeps join-link detection working; a line-breaking tag becomes a newline and every other tag
// is stripped, with HTML entities decoded.
func descriptionText(s string) string {
	if !htmlMarkupRe.MatchString(s) {
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
