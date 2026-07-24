package mailparse

import (
	"encoding/base64"
	"strings"
	"testing"
)

// pngBytes is a tiny stand-in image payload; the extractor treats content as opaque bytes.
var pngBytes = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 1, 2, 3}

func pngDataURI() string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}

func TestExtractDataImagesLiftsAnEmbeddedImage(t *testing.T) {
	out, images := ExtractDataImages(`<p>before <img src="` + pngDataURI() + `"> after</p>`)
	if len(images) != 1 {
		t.Fatalf("images = %d, want 1", len(images))
	}
	img := images[0]
	if img.MediaType != "image/png" {
		t.Errorf("media type = %q, want image/png", img.MediaType)
	}
	if string(img.Content) != string(pngBytes) {
		t.Errorf("content does not round-trip")
	}
	if !strings.HasSuffix(img.CID, "@pigeonpost") {
		t.Errorf("cid = %q, want @pigeonpost suffix", img.CID)
	}
	if !strings.Contains(out, `src="cid:`+img.CID+`"`) {
		t.Errorf("rewritten html lacks cid reference: %s", out)
	}
	if strings.Contains(out, "data:image/") {
		t.Errorf("rewritten html still holds a data URI: %s", out)
	}
}

func TestExtractDataImagesDedupesIdenticalImages(t *testing.T) {
	src := pngDataURI()
	out, images := ExtractDataImages(`<p><img src="` + src + `"><img src="` + src + `"></p>`)
	if len(images) != 1 {
		t.Fatalf("images = %d, want 1 (identical bytes share a part)", len(images))
	}
	if got := strings.Count(out, `src="cid:`+images[0].CID+`"`); got != 2 {
		t.Errorf("cid references = %d, want 2", got)
	}
}

func TestExtractDataImagesKeepsDocumentOrder(t *testing.T) {
	second := []byte{9, 9, 9, 9}
	uriB := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(second)
	_, images := ExtractDataImages(`<p><img src="` + pngDataURI() + `"><img src="` + uriB + `"></p>`)
	if len(images) != 2 {
		t.Fatalf("images = %d, want 2", len(images))
	}
	if images[0].MediaType != "image/png" || images[1].MediaType != "image/jpeg" {
		t.Errorf("order = %q, %q; want png then jpeg", images[0].MediaType, images[1].MediaType)
	}
}

func TestExtractDataImagesDropsDataURIParameters(t *testing.T) {
	uri := "data:image/png;charset=utf-8;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	_, images := ExtractDataImages(`<img src="` + uri + `">`)
	if len(images) != 1 {
		t.Fatalf("images = %d, want 1", len(images))
	}
	if images[0].MediaType != "image/png" {
		t.Errorf("media type = %q, want bare image/png", images[0].MediaType)
	}
}

func TestExtractDataImagesLeavesOtherSourcesAlone(t *testing.T) {
	cases := []string{
		`<img src="https://example.org/pic.png">`,
		`<img src="cid:already@elsewhere">`,
		`<img src="data:text/plain;base64,aGk=">`,
		`<img src="data:image/png;base64,@@not-base64@@">`,
		`<img alt="no source">`,
		`<p>data:image/png;base64, mentioned in prose only</p>`,
	}
	for _, source := range cases {
		out, images := ExtractDataImages(source)
		if len(images) != 0 {
			t.Errorf("%s: lifted %d images, want 0", source, len(images))
		}
		if out != source {
			t.Errorf("%s: html changed to %s", source, out)
		}
	}
}

func TestExtractDataImagesEmptyPayloadIsLeft(t *testing.T) {
	source := `<img src="data:image/png;base64,">`
	out, images := ExtractDataImages(source)
	if len(images) != 0 || out != source {
		t.Errorf("empty payload was lifted: images=%d out=%s", len(images), out)
	}
}

func TestDataImageBytesSumsDecodedSizes(t *testing.T) {
	second := []byte{1, 2, 3, 4, 5}
	html := `<p><img src="` + pngDataURI() + `"><img src="data:image/gif;base64,` +
		base64.StdEncoding.EncodeToString(second) + `"></p>`
	if got, want := DataImageBytes(html), len(pngBytes)+len(second); got != want {
		t.Errorf("DataImageBytes = %d, want %d", got, want)
	}
	if got := DataImageBytes("<p>no images</p>"); got != 0 {
		t.Errorf("DataImageBytes with no images = %d, want 0", got)
	}
}
