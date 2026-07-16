package mailparse

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// LinkifyHTML wraps bare web addresses found in an HTML fragment's text with
// anchors, the way mainstream mail clients do, so a URL pasted as plain text
// arrives clickable. It recognises http and https URLs, mailto addresses and
// www.-prefixed hosts (linked with an http scheme added). Text already inside
// an anchor is left alone, as is script, style and form-control text. It is
// used on both sides of the wire: over a received message's HTML before
// sanitising (the sanitiser then applies its usual scheme policy to the new
// anchors) and over an outgoing message's HTML alternative when it is built.
// On a parse or render failure the fragment is returned unchanged.
func LinkifyHTML(source string) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	for _, text := range linkifiableTextNodes(doc) {
		anchorBareLinks(text)
	}
	var b strings.Builder
	if err := html.Render(&b, doc); err != nil {
		return source
	}
	return b.String()
}

// linkTargetRe matches a bare link target in running text: an http or https
// URL, a mailto address or a www.-prefixed host. Angle brackets, quotes and
// whitespace end a match; trailing punctuation is trimmed separately.
var linkTargetRe = regexp.MustCompile(`(?i)(?:https?://|mailto:|www\.)[^\s<>"']+`)

// trailingPunctuation holds the characters trimmed from the end of a match:
// sentence punctuation after a URL belongs to the sentence, not the link.
const trailingPunctuation = ".,;:!?)]}"

// skippedElements are the elements whose text is never linkified: an existing
// anchor must not gain a nested one and the rest hold non-prose text.
var skippedElements = map[string]bool{
	"a": true, "script": true, "style": true, "textarea": true, "title": true,
}

// linkifiableTextNodes collects the text nodes eligible for linkifying before
// any mutation, so the tree walk never iterates nodes it is rewriting.
func linkifiableTextNodes(doc *html.Node) []*html.Node {
	var nodes []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skippedElements[n.Data] {
			return
		}
		if n.Type == html.TextNode && linkTargetRe.MatchString(n.Data) {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return nodes
}

// anchorBareLinks replaces one text node with an alternation of text and
// anchor nodes covering each bare link target it holds.
func anchorBareLinks(text *html.Node) {
	parent := text.Parent
	if parent == nil {
		return
	}
	rest := text.Data
	insert := func(n *html.Node) { parent.InsertBefore(n, text) }
	for rest != "" {
		loc := linkTargetRe.FindStringIndex(rest)
		if loc == nil {
			break
		}
		match := strings.TrimRight(rest[loc[0]:loc[1]], trailingPunctuation)
		if before := rest[:loc[0]]; before != "" {
			insert(&html.Node{Type: html.TextNode, Data: before})
		}
		insert(anchorNode(match))
		rest = rest[loc[0]+len(match):]
	}
	if rest != "" {
		insert(&html.Node{Type: html.TextNode, Data: rest})
	}
	parent.RemoveChild(text)
}

// anchorNode builds the anchor for one bare link target. A www.-prefixed
// target gets an http scheme in the href while keeping its bare text.
func anchorNode(target string) *html.Node {
	href := target
	if strings.HasPrefix(strings.ToLower(target), "www.") {
		href = "http://" + target
	}
	anchor := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}
	anchor.AppendChild(&html.Node{Type: html.TextNode, Data: target})
	return anchor
}
