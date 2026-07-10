package mailparse

import (
	"encoding/base64"
	"strings"

	"github.com/emersion/go-message/mail"
)

// cidScheme is the URL scheme an HTML body uses to reference an image carried inside the same message
// (RFC 2392), as in <img src="cid:logo">.
const cidScheme = "cid:"

// inlineImage is an image part carried inside the message and referenced from the HTML by a cid: URL.
// Its bytes travel with the message, so it is resolved to a data: URI and shown at once, unlike a remote
// image which is parked until the reader asks for it.
type inlineImage struct {
	mediaType string
	content   []byte
}

// contentID returns a part's Content-ID with the surrounding angle brackets and any whitespace removed,
// lowercased so a cid: reference matches it regardless of case. It is empty when the part carries no
// Content-ID.
func contentID(header mail.PartHeader) string {
	raw := strings.TrimSpace(header.Get("Content-Id"))
	raw = strings.TrimPrefix(raw, "<")
	raw = strings.TrimSuffix(raw, ">")
	return strings.ToLower(strings.TrimSpace(raw))
}

// imageDataURI encodes an embedded image as a base64 data: URI so the webview renders it inline with no
// network fetch.
func imageDataURI(img inlineImage) string {
	return "data:" + img.mediaType + ";base64," + base64.StdEncoding.EncodeToString(img.content)
}

// resolveInlineImage turns a cid: image reference into the embedded image's data: URI when the message
// carried that part. It reports false for anything that is not a resolvable cid: reference (a remote
// URL, an already-inline data: URI or a cid: with no matching part), leaving the caller to decide how
// to treat the source.
func resolveInlineImage(src string, inline map[string]inlineImage) (string, bool) {
	trimmed := strings.TrimSpace(src)
	if !strings.HasPrefix(strings.ToLower(trimmed), cidScheme) {
		return "", false
	}
	id := strings.ToLower(strings.TrimSpace(trimmed[len(cidScheme):]))
	img, ok := inline[id]
	if !ok {
		return "", false
	}
	return imageDataURI(img), true
}

// isRemoteURL reports whether a src would trigger a network fetch: an absolute http(s) URL or a
// protocol-relative one. An embedded data: URI and an unresolved cid: reference are not remote, so the
// caller leaves them in place rather than parking them.
func isRemoteURL(src string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(src))
	return strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		strings.HasPrefix(trimmed, "//")
}
