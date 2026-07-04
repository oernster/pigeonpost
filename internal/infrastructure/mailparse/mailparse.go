// Package mailparse turns a raw RFC 5322 message into the plain-text and sanitised HTML bodies the UI
// renders. It is shared by the mail source adapters (IMAP and POP3) so the message-body handling,
// including HTML sanitising and remote-image blocking, lives in one place rather than being duplicated
// per protocol.
package mailparse

import (
	"bytes"
	"fmt"
	"io"
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
// data-pp-src attribute on images, which is where blockRemoteImages parks a remote image's original
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

// ParseBody parses a raw RFC 5322 message into its plain-text and HTML bodies. When the message has
// only an HTML body, a plain-text rendering is derived from it so the message is always readable.
func ParseBody(raw []byte) (plain, htmlBody string, err error) {
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return "", "", fmt.Errorf("mailparse: parse message: %w", err)
	}
	var plainBuf, htmlBuf strings.Builder
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", fmt.Errorf("mailparse: read part: %w", err)
		}
		inline, ok := part.Header.(*mail.InlineHeader)
		if !ok {
			continue // an attachment; not part of the readable body
		}
		content, err := io.ReadAll(part.Body)
		if err != nil {
			return "", "", fmt.Errorf("mailparse: read part body: %w", err)
		}
		contentType, _, _ := inline.ContentType()
		if contentType == "text/html" {
			htmlBuf.Write(content)
		} else {
			plainBuf.Write(content)
		}
	}
	plain = plainBuf.String()
	htmlBody = htmlBuf.String()
	if htmlBody != "" {
		htmlBody = htmlSanitizer.Sanitize(prepareHTML(htmlBody))
	}
	if strings.TrimSpace(plain) == "" && htmlBody != "" {
		plain = htmlToText(htmlBody)
	}
	return plain, htmlBody, nil
}

// prepareHTML walks the parsed message HTML before sanitising to do two things the sanitiser cannot.
// First it removes nodes the sender deliberately hid with inline CSS (a preheader / preview-text block,
// the snippet a mail client shows in the message list). Those nodes are meant to stay invisible, but
// the sanitiser strips the style attribute that hides them, so left in place they would surface and
// duplicate the visible content; they are dropped here while their hiding style is still readable.
// Second it rewrites every <img src> to a data attribute so remote images do not load automatically:
// auto-loading a remote image leaks that the reader opened the message (and their IP) to the sender, so
// the source is parked until the reader explicitly asks to load images, and srcset is dropped for the
// same reason. On a parse or render failure the original HTML is returned unchanged; the sanitizer
// still runs over it afterwards.
func prepareHTML(source string) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			parkImageSource(n)
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

// parkImageSource renames an image's src attribute to the blocked-image data attribute and drops
// srcset, so the browser has nothing to fetch until the source is restored.
func parkImageSource(n *html.Node) {
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
