package mailparse

import (
	"encoding/base64"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// neutraliseRemoteCSS stops a fragment of message CSS from fetching anything until the reader asks. A
// tracker can pull a remote file through CSS just as through an <img>; CSS offers more ways to spell the
// fetch than a plain url(...): an escaped function name (u\72l), an @import given a bare string and the bare
// string candidates of image-set(). The passes run in this order so each sees the plain form of what the one
// before exposed: escapes are decoded first, so an escaped @import or url( is caught by the passes after it;
// @import rules are removed; image-set strings are rewritten as url(...); and every url(...) is then parked.
func neutraliseRemoteCSS(css string) string {
	css = decodeFetchSpellingEscapes(css)
	css = cssImportRe.ReplaceAllString(css, cssRemovedRule)
	css = wrapImageSetStrings(css)
	return parkRemoteCSSURLs(css)
}

// cssEscapeRe matches one CSS escape: a backslash followed either by one to six hex digits and an optional
// single whitespace (which the escape consumes) or by any one character that is not a newline or hex digit.
// An escaped backslash matches as a whole, so the backslash after it never starts a second escape.
var cssEscapeRe = regexp.MustCompile(`\\(?:([0-9a-fA-F]{1,6})(?:\r\n|[ \t\n\r\f])?|([^\n\r\f0-9a-fA-F]))`)

// cssHexBase is the base of a CSS hex escape's digits.
const cssHexBase = 16

// decodeFetchSpellingEscapes decodes the CSS escapes that can spell part of a fetch (url(, @import,
// image-set() into their plain characters and leaves every other escape exactly as written. Only ASCII
// letters, the open parenthesis, the at sign and the hyphen are decoded: that is every character of those
// names. None of them changes the meaning where an escape is used honestly (a class name or a quote
// glyph in generated content, a CJK font name), which keeps the rewrite from disturbing real styling.
func decodeFetchSpellingEscapes(css string) string {
	if !strings.Contains(css, `\`) {
		return css
	}
	return cssEscapeRe.ReplaceAllStringFunc(css, func(escape string) string {
		parts := cssEscapeRe.FindStringSubmatch(escape)
		decoded := parts[2]
		if parts[1] != "" {
			// The regexp admits only one to six hex digits, which always parse within 32 bits, so there is
			// no error to report. A code point outside Unicode becomes U+FFFD, which is not decoded.
			code, _ := strconv.ParseUint(parts[1], cssHexBase, 32)
			decoded = string(rune(code))
		}
		if len(decoded) == 1 && isFetchSpellingChar(decoded[0]) {
			return decoded
		}
		return escape
	})
}

// isFetchSpellingChar reports whether an escaped character is one decodeFetchSpellingEscapes decodes.
func isFetchSpellingChar(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '(' || c == '@' || c == '-'
}

// cssImportRe matches an @import rule up to its terminating semicolon. An imported stylesheet is dropped
// outright rather than parked: Load images brings back images; a stylesheet is never one.
var cssImportRe = regexp.MustCompile(`(?i)@import\b[^;]*;?`)

// cssRemovedRule replaces a removed rule. It is a space rather than nothing, so the text either side of the
// removal cannot join into a new token (an "@imp" before it and an "ort" after it, say).
const cssRemovedRule = " "

// imageSetOpenRe matches the opening of an image-set() function, prefixed or not.
var imageSetOpenRe = regexp.MustCompile(`(?i)image-set\(`)

// wrapImageSetStrings rewrites every bare string candidate inside image-set() as a url(...), which the url
// parker then handles like any other: the remote ones are parked and come back when the reader loads images.
// A browser treats such a string as a URL to fetch, so left bare it would bypass the parker entirely.
func wrapImageSetStrings(css string) string {
	var b strings.Builder
	rest := css
	for {
		loc := imageSetOpenRe.FindStringIndex(rest)
		if loc == nil {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:loc[1]])
		rest = rest[loc[1]:]
		rest = rest[wrapImageSetArguments(&b, rest):]
	}
}

// wrapImageSetArguments copies one image-set()'s arguments into b, wrapping each string at the top level of
// the function in url(...). A string nested deeper (inside type("image/avif") or an existing url(...)) is not
// a candidate URL and is copied as is. It returns how many bytes of args it consumed, up to and including the
// closing parenthesis.
func wrapImageSetArguments(b *strings.Builder, args string) int {
	depth := 1
	for i := 0; i < len(args); i++ {
		switch c := args[i]; c {
		case '"', '\'':
			end := cssStringEnd(args, i)
			if depth == 1 {
				b.WriteString("url(" + args[i:end] + ")")
			} else {
				b.WriteString(args[i:end])
			}
			i = end - 1
		case '(':
			depth++
			b.WriteByte(c)
		case ')':
			depth--
			b.WriteByte(c)
			if depth == 0 {
				return i + 1
			}
		default:
			b.WriteByte(c)
		}
	}
	return len(args)
}

// cssStringEnd returns the index just past the quoted string starting at start, honouring backslash escapes.
// An unterminated string runs to the end, as it does for a browser.
func cssStringEnd(s string, start int) int {
	quote := s[start]
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(s)
}

// remoteCSSURLRe matches a CSS url(...) reference and captures its target in one of three groups: a double
// quoted string, a single quoted string or an unquoted value. Each form honours backslash escapes, so a
// quote of the other kind, an escaped quote or an escaped space inside the target cannot end the match early
// and leave the real target behind it unparked.
var remoteCSSURLRe = regexp.MustCompile(`(?is)url\(\s*(?:"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)'|((?:[^)'"\s\\]|\\.)*))\s*\)`)

// parkRemoteCSSURLs parks every non-embedded url(...) in a CSS fragment behind the unfetchable parked scheme,
// leaving embedded data: and cid: references intact. Parking rather than discarding is what lets the reader
// ask for the image at all. It is the CSS counterpart of parkElementSource.
//
// Discarding the target used to be the behaviour; it stranded any text the sender coloured for a
// background image: with the image gone for good, a heading set white to sit on a dark photo fell back to
// the sender's pale background-colour and became invisible, which the reader's dark treatment then rendered
// as black on black. A parked target that is not a well-formed http(s) URL is never fetched by the image
// proxy, so it simply stays parked.
func parkRemoteCSSURLs(css string) string {
	matches := remoteCSSURLRe.FindAllStringSubmatchIndex(css, -1)
	if len(matches) == 0 {
		return css
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(css[last:m[0]])
		b.WriteString(parkOneCSSURL(css, m))
		last = m[1]
	}
	b.WriteString(css[last:])
	return b.String()
}

// parkOneCSSURL decides what one url(...) becomes. m is the match's index pairs: the whole match followed by
// the three alternative target groups, of which exactly one took part.
func parkOneCSSURL(css string, m []int) string {
	match := css[m[0]:m[1]]
	target := ""
	for group := 2; group+1 < len(m); group += 2 {
		if m[group] >= 0 {
			target = css[m[group]:m[group+1]]
		}
	}
	if isEmbeddedURL(target) {
		return match
	}
	// A font source is emptied rather than parked. Parking exists so the reader can ask for the resource
	// later; a font never comes back: the image proxy rejects it on content type. Parking one would only
	// buy an outbound request to the sender's CDN, on a press of Load images, that cannot succeed.
	if !needsParking(target) || isFontSourceDeclaration(css[:m[0]]) {
		return "url()"
	}
	return "url(" + parkedCSSURLScheme + base64.RawURLEncoding.EncodeToString([]byte(browserURLValue(target))) + ")"
}

// isFontSourceDeclaration reports whether the CSS ending at a url(...) is inside a src declaration, which in
// email CSS means an @font-face source. It reads back to the start of the current declaration and compares
// the property name, so a url() later in a multi-value src (the usual "local(...), url(...)" pair) is caught
// as readily as one on its own.
func isFontSourceDeclaration(before string) bool {
	declaration := before[strings.LastIndexAny(before, ";{}")+1:]
	colon := strings.Index(declaration, ":")
	if colon < 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(declaration[:colon]), "src")
}

// parkStyleAttrURLs neutralises remote references in an element's inline style attribute.
func parkStyleAttrURLs(n *html.Node) {
	for i, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "style") {
			n.Attr[i].Val = neutraliseRemoteCSS(attr.Val)
		}
	}
}

// parkStyleElementURLs neutralises remote references inside a <style> element's CSS text.
func parkStyleElementURLs(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			c.Data = neutraliseRemoteCSS(c.Data)
		}
	}
}
