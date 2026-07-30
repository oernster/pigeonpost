package remoteimage

import (
	"context"
	"encoding/base64"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// parkedCSSURLScheme is the scheme a remote CSS url(...) target is parked behind by the body parser, with the
// target base64url-encoded after it. It must match mailparse's copy: that package writes the parked form when
// it prepares a body and this one reads it back to fetch. The two are kept in step by this value rather than a
// shared symbol, since one infrastructure adapter does not import another.
const parkedCSSURLScheme = "pp-blocked:"

// parkedCSSURLRe matches one parked reference and captures its encoded target. The encoding is base64url, so
// the capture is restricted to that alphabet and cannot run past the closing bracket.
var parkedCSSURLRe = regexp.MustCompile(`url\(` + regexp.QuoteMeta(parkedCSSURLScheme) + `([A-Za-z0-9_-]*)\)`)

// parkedCSS is one piece of CSS carrying parked references, together with the way to write the rewritten CSS
// back where it came from: an element's style attribute or the text of a <style> element.
type parkedCSS struct {
	css   string
	apply func(string)
}

// collectParkedCSS walks a node subtree and appends every style attribute and <style> text node that carries a
// parked reference. CSS with no parked reference is skipped, so a message that uses none costs nothing.
func collectParkedCSS(n *html.Node, out *[]parkedCSS) {
	if n.Type == html.ElementNode {
		for i, attr := range n.Attr {
			if strings.EqualFold(attr.Key, "style") && strings.Contains(attr.Val, parkedCSSURLScheme) {
				node, index := n, i
				*out = append(*out, parkedCSS{css: attr.Val, apply: func(css string) { node.Attr[index].Val = css }})
			}
		}
	}
	if n.Type == html.ElementNode && n.Data == "style" {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode && strings.Contains(c.Data, parkedCSSURLScheme) {
				text := c
				*out = append(*out, parkedCSS{css: c.Data, apply: func(css string) { text.Data = css }})
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectParkedCSS(c, out)
	}
}

// parkedCSSTargets returns the distinct remote URLs a set of parked CSS refers to, so each is fetched once
// however many rules or elements share it (an email routinely repeats one hero image across breakpoints).
// An entry that does not decode, or decodes to something that is not an http/https URL, is ignored.
func parkedCSSTargets(parked []parkedCSS) []string {
	seen := make(map[string]struct{})
	var targets []string
	for _, entry := range parked {
		for _, match := range parkedCSSURLRe.FindAllStringSubmatch(entry.css, -1) {
			target, ok := decodeParkedTarget(match[1])
			if !ok {
				continue
			}
			if _, done := seen[target]; done {
				continue
			}
			seen[target] = struct{}{}
			targets = append(targets, target)
		}
	}
	return targets
}

// decodeParkedTarget decodes one parked target back to its URL, reporting whether it is a remote URL this
// package will fetch. Anything else is left parked rather than guessed at.
func decodeParkedTarget(encoded string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	target := string(raw)
	if !isHTTPURL(target) {
		return "", false
	}
	return target, true
}

// inlineCSSBackgrounds fetches every remote background a message's parked CSS refers to and rewrites each
// reference to a data: URI, so a background image renders once the reader has asked for images. A reference
// whose fetch failed is left parked: it still paints nothing, and the reader can ask again.
func (r *Resolver) inlineCSSBackgrounds(ctx context.Context, parked []parkedCSS) {
	targets := parkedCSSTargets(parked)
	if len(targets) == 0 {
		return
	}
	resolved := r.fetchAll(ctx, targets)
	for _, entry := range parked {
		rewritten := parkedCSSURLRe.ReplaceAllStringFunc(entry.css, func(match string) string {
			target, ok := decodeParkedTarget(parkedCSSURLRe.FindStringSubmatch(match)[1])
			if !ok {
				return match
			}
			uri, fetched := resolved[target]
			if !fetched {
				return match
			}
			return "url(" + uri + ")"
		})
		if rewritten != entry.css {
			entry.apply(rewritten)
		}
	}
}
