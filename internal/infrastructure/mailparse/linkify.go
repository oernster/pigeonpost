package mailparse

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// LinkifyHTML wraps bare web addresses found in an HTML fragment's text with
// anchors, the way mainstream mail clients do, so a URL pasted as plain text
// arrives clickable. It recognises http and https URLs, mailto addresses,
// www.-prefixed hosts (linked with an http scheme added) and markdown-style
// labelled links ("[label](url)"), which become anchors showing their label.
// Text already inside an anchor is left alone, as is script, style and
// form-control text. It is used on both sides of the wire: over a received
// message's HTML before sanitising (the sanitiser then applies its usual
// scheme policy to the new anchors) and over an outgoing message's HTML
// alternative when it is built. On a parse or render failure the fragment is
// returned unchanged.
func LinkifyHTML(source string) string {
	return linkify(source, false)
}

// linkifyForDisplay is the received-message variant: besides anchoring, it
// marks each anchor that stands alone on its own line with soloLinkClass, so
// the reader can present it as a button. Outgoing mail never gets the class:
// it would carry no stylesheet and only add noise for other clients.
func linkifyForDisplay(source string) string {
	return linkify(source, true)
}

func linkify(source string, markSolo bool) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	for _, text := range linkifiableTextNodes(doc) {
		anchorBareLinks(text)
	}
	if markSolo {
		markSoloLinks(doc)
	}
	var b strings.Builder
	if err := html.Render(&b, doc); err != nil {
		return source
	}
	return b.String()
}

// soloLinkClass marks an anchor that is the only content on its line of the
// message, so the reader styles it as a button. The sanitiser keeps class
// attributes (AllowStyling), so the marker survives sanitising.
const soloLinkClass = "pp-solo-link"

// linkTargetRe matches a bare link target in running text: an http or https
// URL, a mailto address or a www.-prefixed host. Angle brackets, quotes and
// whitespace end a match; trailing punctuation is trimmed separately.
var linkTargetRe = regexp.MustCompile(`(?i)(?:https?://|mailto:|www\.)[^\s<>"']+`)

// markdownLinkRe matches a markdown-style labelled link: [label](target)
// where the target is a bare link target. The label shows; the target is the
// href. Parentheses and whitespace end the target so the closing bracket is
// found reliably.
var markdownLinkRe = regexp.MustCompile(`(?i)\[([^\][]+)\]\(((?:https?://|mailto:|www\.)[^\s()<>]+)\)`)

// trailingPunctuation holds the characters trimmed from the end of a bare
// match: sentence punctuation after a URL belongs to the sentence, not the
// link.
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
// anchor nodes covering each labelled or bare link target it holds. When a
// markdown label and a bare target could both start a match, the earlier one
// in the text wins (the bare pattern would otherwise eat a labelled link's
// target).
func anchorBareLinks(text *html.Node) {
	parent := text.Parent
	if parent == nil {
		return
	}
	rest := text.Data
	insert := func(n *html.Node) { parent.InsertBefore(n, text) }
	emitBefore := func(end int) {
		if before := rest[:end]; before != "" {
			insert(&html.Node{Type: html.TextNode, Data: before})
		}
	}
	for rest != "" {
		labelled := markdownLinkRe.FindStringSubmatchIndex(rest)
		bare := linkTargetRe.FindStringIndex(rest)
		if labelled != nil && (bare == nil || labelled[0] <= bare[0]) {
			emitBefore(labelled[0])
			insert(anchorNode(rest[labelled[2]:labelled[3]], rest[labelled[4]:labelled[5]]))
			rest = rest[labelled[1]:]
			continue
		}
		if bare == nil {
			break
		}
		match := strings.TrimRight(rest[bare[0]:bare[1]], trailingPunctuation)
		emitBefore(bare[0])
		insert(anchorNode(match, match))
		rest = rest[bare[0]+len(match):]
	}
	if rest != "" {
		insert(&html.Node{Type: html.TextNode, Data: rest})
	}
	parent.RemoveChild(text)
}

// anchorNode builds the anchor for one link: the label is the visible text
// and the target the href. A www.-prefixed target gets an http scheme in the
// href while the visible text keeps its bare form.
func anchorNode(label, target string) *html.Node {
	href := target
	if strings.HasPrefix(strings.ToLower(target), "www.") {
		href = "http://" + target
	}
	anchor := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}
	anchor.AppendChild(&html.Node{Type: html.TextNode, Data: label})
	return anchor
}

// markSoloLinks tags every anchor that is the only content on its line with
// soloLinkClass. A line runs between <br> elements or block edges, so both a
// paragraph holding just a link and a link on its own br-separated line
// qualify; any sender-supplied class is replaced (the sanitiser strips
// arbitrary classes anyway).
func markSoloLinks(doc *html.Node) {
	var anchors []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			anchors = append(anchors, n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	for _, anchor := range anchors {
		if soloOnLine(anchor) {
			setClass(anchor, soloLinkClass)
		}
	}
}

// soloOnLine reports whether every sibling between the anchor and the nearest
// line boundary on each side is whitespace-only text.
func soloOnLine(anchor *html.Node) bool {
	for s := anchor.PrevSibling; s != nil; s = s.PrevSibling {
		if isLineBoundary(s) {
			break
		}
		if !isWhitespaceText(s) {
			return false
		}
	}
	for s := anchor.NextSibling; s != nil; s = s.NextSibling {
		if isLineBoundary(s) {
			break
		}
		if !isWhitespaceText(s) {
			return false
		}
	}
	return true
}

func isLineBoundary(n *html.Node) bool {
	return n.Type == html.ElementNode && (n.Data == "br" || isBlockElement(n.Data))
}

func isWhitespaceText(n *html.Node) bool {
	return n.Type == html.TextNode && strings.TrimSpace(n.Data) == ""
}

func setClass(n *html.Node, value string) {
	for i, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "class") {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: "class", Val: value})
}
