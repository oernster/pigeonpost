// Package mailparse turns a raw RFC 5322 message into the plain-text and sanitised HTML bodies the UI
// renders. It is shared by the mail source adapters (IMAP and POP3) so the message-body handling,
// including HTML sanitising and remote-image blocking, lives in one place rather than being duplicated
// per protocol.
package mailparse

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"regexp"
	"strings"

	// Registers a CharsetReader on go-message so bodies in legacy charsets (iso-8859-1, windows-1252,
	// and the rest) decode instead of failing with "unhandled charset".
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

// htmlSanitizer strips anything unsafe (scripts, event handlers, javascript: URLs, style/iframe/form
// elements) from message HTML while keeping common formatting and links. It also allows the
// data-pp-src attribute on images, which is where parkElementSource parks a remote image's original
// source so it does not load until the reader asks for it. It is built once and safe for concurrent use.
var htmlSanitizer = buildSanitizer()

// blockedImageAttr holds a remote image's original src while it is prevented from loading.
const blockedImageAttr = "data-pp-src"

func buildSanitizer() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs(blockedImageAttr).OnElements("img")
	return policy
}

// blankLines collapses three or more consecutive newlines down to two.
var blankLines = regexp.MustCompile(`\n{3,}`)

// ParsedBody is the result of parsing a raw message: its plain-text and HTML bodies, and the raw
// text/calendar payload when the message carried one (an iMIP scheduling object such as a meeting
// invite). Invite is nil when the message carried no calendar part.
type ParsedBody struct {
	Plain  string
	HTML   string
	Invite []byte
}

// ParseBody parses a raw RFC 5322 message into its plain-text and HTML bodies plus any text/calendar
// scheduling payload. When the message has only an HTML body, a plain-text rendering is derived from it
// so the message is always readable. A text/calendar part (whether sent inline or as an attachment) is
// captured as the invite rather than folded into the readable body; the first such part wins.
func ParseBody(raw []byte) (ParsedBody, error) {
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return ParsedBody{}, fmt.Errorf("mailparse: parse message: %w", err)
	}
	var plainBuf, htmlBuf strings.Builder
	var invite []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParsedBody{}, fmt.Errorf("mailparse: read part: %w", err)
		}
		mediaType := partMediaType(part.Header)
		if mediaType == "text/calendar" {
			content, err := io.ReadAll(part.Body)
			if err != nil {
				return ParsedBody{}, fmt.Errorf("mailparse: read part body: %w", err)
			}
			if invite == nil {
				invite = content
			}
			continue
		}
		if _, ok := part.Header.(*mail.InlineHeader); !ok {
			continue // an attachment; not part of the readable body
		}
		content, err := io.ReadAll(part.Body)
		if err != nil {
			return ParsedBody{}, fmt.Errorf("mailparse: read part body: %w", err)
		}
		if mediaType == "text/html" {
			htmlBuf.Write(content)
		} else {
			plainBuf.Write(content)
		}
	}
	plain := plainBuf.String()
	htmlBody := htmlBuf.String()
	if htmlBody != "" {
		htmlBody = htmlSanitizer.Sanitize(prepareHTML(htmlBody))
	}
	if strings.TrimSpace(plain) == "" && htmlBody != "" {
		plain = htmlToText(htmlBody)
	}
	return ParsedBody{Plain: plain, HTML: htmlBody, Invite: invite}, nil
}

// partMediaType returns a part's media type (lowercased, without parameters), or an empty string when
// the part has no readable Content-Type. It reads the raw header so it works for an attachment part as
// well as an inline one, since the PartHeader interface exposes no typed ContentType accessor.
func partMediaType(header mail.PartHeader) string {
	mediaType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return mediaType
}

// prepareHTML walks the parsed message HTML before sanitising to do two things the sanitiser cannot.
// First it removes nodes the sender deliberately hid with inline CSS (a preheader / preview-text block,
// the snippet a mail client shows in the message list). Those nodes are meant to stay invisible, but
// the sanitiser strips the style attribute that hides them, so left in place they would surface and
// duplicate the visible content; they are dropped here while their hiding style is still readable.
// Second it stops the message from auto-loading any remote resource, which would leak that the reader
// opened it (and their IP) to the sender. It parks every <img> and <picture> <source> src into a data
// attribute and drops srcset, and it neutralises remote url(...) references in inline style attributes
// and <style> elements, so a CSS background cannot be used as a tracking pixel either. Embedded data:
// and cid: references are left intact. On a parse or render failure the original HTML is returned
// unchanged; the sanitizer still runs over it afterwards.
func prepareHTML(source string) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img", "source":
				parkElementSource(n)
			case "style":
				stripStyleElementURLs(n)
			}
			stripStyleAttrURLs(n)
		}
		var next *html.Node
		for c := n.FirstChild; c != nil; c = next {
			next = c.NextSibling
			if c.Type == html.ElementNode && isHiddenBySender(c) {
				n.RemoveChild(c)
				continue
			}
			walk(c)
		}
	}
	walk(doc)
	var b strings.Builder
	if err := html.Render(&b, doc); err != nil {
		return source
	}
	return b.String()
}

// hiddenStyleRe matches the inline CSS senders use to hide preheader / preview text: an off display, an
// invisible or zero-opacity box, a collapsed height, or a zero or 1px font. Numeric values are anchored
// to a declaration terminator (optionally through !important) so a visible opacity:0.9 or
// font-size:0.5em is not caught.
var hiddenStyleRe = regexp.MustCompile(`(?i)(?:display\s*:\s*none|visibility\s*:\s*hidden|mso-hide\s*:\s*all|opacity\s*:\s*0(?:\.0+)?(?:\s*!important)?\s*(?:;|$)|(?:^|[;{\s])(?:max-)?height\s*:\s*0(?:px)?(?:\s*!important)?\s*(?:;|$)|font-size\s*:\s*(?:0(?:px|pt|em|rem)?|1px)(?:\s*!important)?\s*(?:;|$))`)

// isHiddenBySender reports whether an element is one the sender hid from view, via the HTML hidden
// attribute or an inline style that makes it invisible. Such elements are preheader / preview text that
// must not surface once the sanitiser removes the style that hides them.
func isHiddenBySender(n *html.Node) bool {
	for _, attr := range n.Attr {
		switch {
		case strings.EqualFold(attr.Key, "hidden"):
			return true
		case strings.EqualFold(attr.Key, "style") && hiddenStyleRe.MatchString(attr.Val):
			return true
		}
	}
	return false
}

// parkElementSource renames an element's src attribute to the blocked-image data attribute and drops
// srcset, so the browser has nothing to fetch until the source is restored. It covers <img> and the
// <source> children of a <picture>, both of which trigger a remote fetch through src or srcset.
func parkElementSource(n *html.Node) {
	kept := n.Attr[:0]
	for _, attr := range n.Attr {
		switch strings.ToLower(attr.Key) {
		case "src":
			attr.Key = blockedImageAttr
			kept = append(kept, attr)
		case "srcset":
			// Dropped: srcset is another way to trigger a remote fetch.
		default:
			kept = append(kept, attr)
		}
	}
	n.Attr = kept
}

// remoteCSSURLRe matches a CSS url(...) reference and captures its target, so a remote target can be
// told apart from an embedded one.
var remoteCSSURLRe = regexp.MustCompile(`(?i)url\(\s*['"]?([^)'"]*)['"]?\s*\)`)

// stripRemoteCSSURLs replaces every remote url(...) in a CSS fragment with an empty url(), leaving
// embedded data: and cid: references intact. A tracker can pull a remote file through a CSS background
// just as through an <img>, so this closes that vector in both inline styles and <style> elements.
func stripRemoteCSSURLs(css string) string {
	return remoteCSSURLRe.ReplaceAllStringFunc(css, func(match string) string {
		target := strings.ToLower(strings.TrimSpace(remoteCSSURLRe.FindStringSubmatch(match)[1]))
		if strings.HasPrefix(target, "data:") || strings.HasPrefix(target, "cid:") {
			return match
		}
		return "url()"
	})
}

// stripStyleAttrURLs neutralises remote url(...) references in an element's inline style attribute.
func stripStyleAttrURLs(n *html.Node) {
	for i, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "style") {
			n.Attr[i].Val = stripRemoteCSSURLs(attr.Val)
		}
	}
}

// stripStyleElementURLs neutralises remote url(...) references inside a <style> element's CSS text.
func stripStyleElementURLs(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			c.Data = stripRemoteCSSURLs(c.Data)
		}
	}
}

// htmlToText renders HTML into readable plain text: it drops script/style, turns <br> and the close of
// block elements into line breaks, and collapses runs of blank lines.
func htmlToText(source string) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && n.Data == "br" {
			b.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && isBlockElement(n.Data) {
			b.WriteByte('\n')
		}
	}
	walk(doc)
	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(strings.Join(strings.Fields(line), " "), " ")
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "section", "article":
		return true
	default:
		return false
	}
}
