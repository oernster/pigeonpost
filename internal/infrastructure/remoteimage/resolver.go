// Package remoteimage fetches a message's blocked remote images server-side and inlines them as data: URIs, so
// the reader can show images a browser cannot load cross-origin. Many senders serve their images with a
// Cross-Origin-Resource-Policy of same-origin (or with CORS or hotlink protection), which stops the sandboxed
// reader iframe from embedding them by URL however hard it tries; the bytes are reachable only from a plain
// server-side GET. Fetching here (a server ignores CORP where a browser cannot) and rewriting each <img> to
// carry the bytes inline means the iframe only ever renders data: images, so it also makes no remote request
// that could leak the message was opened. The fetch itself is hardened against server-side request forgery in
// fetch.go.
package remoteimage

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// blockedImageAttr is the data attribute a parked remote image's original source lives in. It must match
// mailparse's blockedImageAttr: mailparse parks the source there when it sanitises the body (so nothing loads
// until the reader asks) and this package reads it back to fetch. The value is part of the body's wire shape,
// shared with the frontend that also keys off "data-pp-src", so the three stay in step by this constant's value
// rather than a shared symbol (an infrastructure adapter does not import another).
const blockedImageAttr = "data-pp-src"

// resolvedImageAttr is the attribute a fetched image is written to: a successful fetch renames the parked
// attribute to the ordinary src the browser renders.
const resolvedImageAttr = "src"

// maxConcurrentFetches caps how many images are fetched at once, so a message carrying many remote images does
// not open a burst of outbound connections.
const maxConcurrentFetches = 6

// fetchFunc fetches one image URL and returns its bytes and response Content-Type, or an error. It is the seam
// the SSRF-safe HTTP client is injected through, so Resolve can be tested without real network access.
type fetchFunc func(ctx context.Context, url string) (data []byte, contentType string, err error)

// Resolver rewrites a sanitised HTML fragment, replacing each parked remote image with the image bytes inlined
// as a data: URI. It holds only the fetch seam, so it is immutable and safe for concurrent use.
type Resolver struct {
	fetch fetchFunc
}

// NewResolver builds a Resolver wired to the real SSRF-hardened fetch.
func NewResolver() *Resolver {
	return &Resolver{fetch: newSafeFetch()}
}

// blockedImage pairs a parked <img> node with the remote URL to fetch for it.
type blockedImage struct {
	node *html.Node
	url  string
}

// Resolve fetches the fragment's parked remote images and inlines them as data: URIs. It parses the fragment,
// collects every <img> that carries a http/https blocked-image attribute, fetches them concurrently and, for
// each that succeeds, renames the parked attribute to src with a data: URI of the bytes. An image whose fetch
// fails is left parked, so it simply does not show. The result is rendered back as a fragment in the same
// shape as the input. On a parse or render failure the source is returned unchanged, so a malformed body is
// never left worse off than before.
func (r *Resolver) Resolve(ctx context.Context, fragment string) (string, error) {
	// A body context makes ParseFragment keep an inline <style> (which a sanitised email body can carry) as a
	// sibling of the content rather than hoisting it into a synthesised <head> that a fragment render drops.
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), body)
	if err != nil {
		return fragment, nil
	}
	var blocked []blockedImage
	for _, n := range nodes {
		collectBlockedImages(n, &blocked)
	}
	if len(blocked) == 0 {
		return fragment, nil
	}
	r.inlineImages(ctx, blocked)
	var b strings.Builder
	for _, n := range nodes {
		if err := html.Render(&b, n); err != nil {
			return fragment, nil
		}
	}
	return b.String(), nil
}

// collectBlockedImages walks a node subtree and appends every <img> carrying a http/https blocked-image
// attribute. A non-remote value (anything not http/https) is left alone, so only a genuine remote source is
// ever fetched.
func collectBlockedImages(n *html.Node, out *[]blockedImage) {
	if n.Type == html.ElementNode && n.Data == "img" {
		for _, attr := range n.Attr {
			if attr.Key == blockedImageAttr && isHTTPURL(attr.Val) {
				*out = append(*out, blockedImage{node: n, url: attr.Val})
				break
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectBlockedImages(c, out)
	}
}

// inlineImages fetches every blocked image concurrently (capped by maxConcurrentFetches) and, for each fetch
// that succeeds, rewrites its node's parked attribute to a src data: URI. The fetches run concurrently but
// every node is rewritten here on the single calling goroutine after they finish, so the parse tree is never
// mutated concurrently.
func (r *Resolver) inlineImages(ctx context.Context, blocked []blockedImage) {
	dataURIs := make([]string, len(blocked))
	sem := make(chan struct{}, maxConcurrentFetches)
	var wg sync.WaitGroup
	for i, img := range blocked {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data, contentType, err := r.fetch(ctx, url)
			if err != nil {
				return
			}
			dataURIs[index] = dataURI(contentType, data)
		}(i, img.url)
	}
	wg.Wait()
	for i, img := range blocked {
		if dataURIs[i] != "" {
			setImageSource(img.node, dataURIs[i])
		}
	}
}

// setImageSource renames a node's parked blocked-image attribute to src and gives it the resolved data: URI, so
// the browser now renders the inlined bytes. Every other attribute is left untouched.
func setImageSource(n *html.Node, uri string) {
	for i := range n.Attr {
		if n.Attr[i].Key == blockedImageAttr {
			n.Attr[i].Key = resolvedImageAttr
			n.Attr[i].Val = uri
			return
		}
	}
}

// dataURI builds a base64 data: URI from an image's Content-Type and bytes.
func dataURI(contentType string, data []byte) string {
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
}
