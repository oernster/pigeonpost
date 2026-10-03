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

// dataScheme is the URL scheme of an image whose bytes are written into the HTML itself.
const dataScheme = "data:"

// embeddedSchemes are the only schemes whose content travels with the message, so the only sources shown
// without the reader asking. Everything else is parked.
var embeddedSchemes = []string{dataScheme, cidScheme}

// browserURLValue reads a URL attribute or CSS target the way a browser does before parsing it: tab, line
// feed and carriage return removed anywhere, then leading and trailing C0 controls and spaces trimmed. The
// trim is deliberately the URL standard's and not strings.TrimSpace, which also strips Unicode spaces a
// browser keeps; trimming those would let a value that a browser reads as something else pass as embedded.
func browserURLValue(src string) string {
	return strings.TrimFunc(urlControlWhitespace.Replace(src), func(r rune) bool { return r <= ' ' })
}

// isEmbeddedURL reports whether a source is carried inside the message: a data: URI or a cid: reference,
// in any case. It is an allow-list on purpose. A browser fetches far more spellings than the http://,
// https:// and // prefixes an earlier deny-list tested for (http:\\host, http:/host, https:host, a leading
// backslash pair), so the only safe question is whether a source is one of the two that cannot reach the
// network.
func isEmbeddedURL(src string) bool {
	value := strings.ToLower(browserURLValue(src))
	for _, scheme := range embeddedSchemes {
		if strings.HasPrefix(value, scheme) {
			return true
		}
	}
	return false
}

// needsParking reports whether a source must be held back until the reader asks: anything that is not
// embedded, except an empty value, which no browser fetches and which would otherwise raise a Load images
// offer for nothing.
func needsParking(src string) bool {
	return browserURLValue(src) != "" && !isEmbeddedURL(src)
}
