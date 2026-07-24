package mailparse

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/net/html"
)

// dataImageCIDBytes is how many bytes of the content hash form the Content-ID: 8 bytes (16 hex
// characters) keeps the id short while making a collision between two different images in one
// message vanishingly unlikely.
const dataImageCIDBytes = 8

// dataImageCIDDomain is the domain part of a generated Content-ID, giving it the msg-id shape
// RFC 2392 expects.
const dataImageCIDDomain = "@pigeonpost"

// dataURIBase64Marker separates a data: URI's media type from its base64 payload.
const dataURIBase64Marker = ";base64,"

// DataImage is one embedded image lifted out of an outgoing HTML body: its decoded bytes, its media
// type and the Content-ID the rewritten HTML now references it by (bare, without angle brackets).
type DataImage struct {
	CID       string
	MediaType string
	Content   []byte
}

// ExtractDataImages lifts every embedded <img src="data:image/...;base64,..."> out of an outgoing HTML
// fragment, rewriting each src to a cid: reference and returning the images in document order. The id is
// a hash of the image bytes, so the same image pasted twice becomes one cid and one returned part. A src
// that is not a well-formed base64 image data: URI is left untouched, as is everything else in the
// fragment. On a parse or render failure the original HTML is returned with no images, which degrades to
// the message being sent with its data: URIs inline.
func ExtractDataImages(source string) (string, []DataImage) {
	if !strings.Contains(source, "data:image/") {
		return source, nil
	}
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source, nil
	}
	seen := make(map[string]bool)
	var images []DataImage
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			if img, ok := liftDataImage(n); ok && !seen[img.CID] {
				seen[img.CID] = true
				images = append(images, img)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(images) == 0 {
		return source, nil
	}
	var b strings.Builder
	if err := html.Render(&b, doc); err != nil {
		return source, nil
	}
	return b.String(), images
}

// liftDataImage decodes an img element's base64 image data: URI, rewrites the src to the image's cid:
// reference and returns the lifted image. It reports false when the src is not such a URI or its
// payload does not decode, leaving the element untouched.
func liftDataImage(n *html.Node) (DataImage, bool) {
	for i, attr := range n.Attr {
		if !strings.EqualFold(attr.Key, "src") {
			continue
		}
		img, ok := decodeImageDataURI(attr.Val)
		if !ok {
			return DataImage{}, false
		}
		n.Attr[i].Val = cidScheme + img.CID
		return img, true
	}
	return DataImage{}, false
}

// decodeImageDataURI parses a base64 image data: URI into its bytes and media type, stamping the
// content-derived Content-ID. It reports false for any other src.
func decodeImageDataURI(src string) (DataImage, bool) {
	trimmed := strings.TrimSpace(src)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "data:image/") {
		return DataImage{}, false
	}
	marker := strings.Index(lower, dataURIBase64Marker)
	if marker < 0 {
		return DataImage{}, false
	}
	// Any parameters between the media type and the base64 marker (a charset, say) are dropped: the
	// part's Content-Type wants the bare type.
	mediaType, _, _ := strings.Cut(lower[len("data:"):marker], ";")
	content, err := base64.StdEncoding.DecodeString(trimmed[marker+len(dataURIBase64Marker):])
	if err != nil || len(content) == 0 {
		return DataImage{}, false
	}
	return DataImage{CID: contentCID(content), MediaType: mediaType, Content: content}, true
}

// contentCID derives an image part's Content-ID from its bytes, so identical images share one part and
// the id is stable across rebuilds of the same message.
func contentCID(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:dataImageCIDBytes]) + dataImageCIDDomain
}

// DataImageBytes returns the total decoded size of the embedded images an HTML fragment carries, so a
// size cap can count pasted inline images alongside ordinary attachments without lifting them out.
func DataImageBytes(source string) int {
	_, images := ExtractDataImages(source)
	total := 0
	for _, img := range images {
		total += len(img.Content)
	}
	return total
}
