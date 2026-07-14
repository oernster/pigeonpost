package mailparse

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// prepareHTML walks the parsed message HTML before sanitising to do two things the sanitiser cannot.
// First it removes nodes the sender deliberately hid with inline CSS (a preheader / preview-text block,
// the snippet a mail client shows in the message list). Those nodes are meant to stay invisible, but
// the sanitiser strips the style attribute that hides them, so left in place they would surface and
// duplicate the visible content; they are dropped here while their hiding style is still readable.
// Second it stops the message from auto-loading any remote resource, which would leak that the reader
// opened it (and their IP) to the sender. It parks a remote <img> or <picture> <source> src into a
// data attribute and drops srcset; it also neutralises remote url(...) references in inline style
// attributes and <style> elements, so a CSS background cannot be used as a tracking pixel either. An
// embedded image is shown at once: a cid: reference is resolved to the message's own image part as a
// data: URI, while an inline data: URI is kept. On a parse or render failure the original HTML is returned
// unchanged; the sanitizer still runs over it afterwards.
func prepareHTML(source string, inline map[string]inlineImage) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img", "source":
				parkElementSource(n, inline)
			case "style":
				stripStyleElementURLs(n)
			case "a":
				normaliseAnchorHref(n)
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

// hiddenStyleRe matches the inline CSS senders use to hide preheader / preview text from every client: an
// off display, an invisible or zero-opacity box or a collapsed height. Numeric values are anchored to a
// declaration terminator (optionally through !important) so a visible opacity:0.9 is not caught. Two
// declarations are handled elsewhere. A zero font size (tinyFontStyleRe) hides only an element's own text,
// not element children that re-set their size. mso-hide:all is deliberately not matched: it hides content
// in Outlook only, so every other client (this one included) is meant to show it; treating it as hidden
// would drop content the sender intended to be visible.
var hiddenStyleRe = regexp.MustCompile(`(?i)(?:display\s*:\s*none|visibility\s*:\s*hidden|opacity\s*:\s*0(?:\.0+)?(?:\s*!important)?\s*(?:;|$)|(?:^|[;{\s])(?:max-)?height\s*:\s*0(?:px)?(?:\s*!important)?\s*(?:;|$))`)

// tinyFontStyleRe matches a zero or 1px font size. This hides an element's own text, so it marks a leaf
// preheader (a "<span style=font-size:0>preview</span>"); it is NOT a whole-subtree hide: email
// frameworks (MJML and the like) put font-size:0 on layout wrapper cells to collapse the whitespace
// between inline-block columns, with the real text inside re-setting its size. So it is treated as hidden
// only for a leaf element; a font-size:0 element that has element children is a visible wrapper to keep,
// not a preheader to drop (dropping it deletes the whole visible body of those messages).
var tinyFontStyleRe = regexp.MustCompile(`(?i)font-size\s*:\s*(?:0(?:px|pt|em|rem)?|1px)(?:\s*!important)?\s*(?:;|$)`)

// isHiddenBySender reports whether an element is one the sender hid from view, via the HTML hidden
// attribute or an inline style that makes it invisible. Such elements are preheader / preview text that
// must not surface once the sanitiser removes the style that hides them.
func isHiddenBySender(n *html.Node) bool {
	for _, attr := range n.Attr {
		switch {
		case strings.EqualFold(attr.Key, "hidden"):
			return true
		case strings.EqualFold(attr.Key, "style"):
			if hiddenStyleRe.MatchString(attr.Val) {
				return true
			}
			if tinyFontStyleRe.MatchString(attr.Val) && !hasElementChild(n) {
				return true
			}
		}
	}
	return false
}

// hasElementChild reports whether n has at least one element child. A zero font size hides an element's
// own text but not element children (they re-set their size), so a font-size:0 element with element
// children is a layout wrapper to keep rather than a hidden preheader to drop.
func hasElementChild(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return true
		}
	}
	return false
}

// urlControlWhitespace removes the ASCII tab, line feed and carriage return characters that the URL
// standard strips from every URL before parsing. Bulk-mail senders wrap long href attribute values
// across source lines, so these characters routinely appear inside real hrefs; browsers remove them
// and follow the link, but Go's url.Parse rejects them, which made the sanitiser silently delete the
// whole anchor and leave the email's button styled but dead.
var urlControlWhitespace = strings.NewReplacer("\t", "", "\n", "", "\r", "")

// normaliseAnchorHref rewrites an anchor's href the way a browser would read it: control whitespace
// removed, the edges trimmed and any interior space percent-encoded, so a link a browser would follow
// survives sanitising with the same target. It runs before the sanitiser, so scheme policy is
// unaffected: a javascript: href that this normalisation makes legible is exactly what the sanitiser
// then rejects, which is safer than passing the obfuscated form through.
func normaliseAnchorHref(n *html.Node) {
	for i, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "href") {
			href := strings.TrimSpace(urlControlWhitespace.Replace(attr.Val))
			n.Attr[i].Val = strings.ReplaceAll(href, " ", "%20")
		}
	}
}

// parkElementSource rewrites an image element's src for safe display and drops srcset. An embedded
// image is shown at once: a cid: reference is swapped for the matching part's data: URI, while an
// inline data: URI is left as is. A remote src is parked into the blocked-image data attribute instead, so the
// browser fetches nothing until the reader asks. srcset is always dropped, being a second way to
// trigger a remote fetch. It covers <img> and the <source> children of a <picture>.
func parkElementSource(n *html.Node, inline map[string]inlineImage) {
	kept := n.Attr[:0]
	for _, attr := range n.Attr {
		switch strings.ToLower(attr.Key) {
		case "src":
			if resolved, ok := resolveInlineImage(attr.Val, inline); ok {
				attr.Val = resolved
			} else if isRemoteURL(attr.Val) {
				attr.Key = blockedImageAttr
			}
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
// block elements into line breaks and collapses runs of blank lines.
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
