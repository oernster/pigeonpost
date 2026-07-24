package message

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// inlinePNG is a tiny stand-in for a pasted image; the builder treats content as opaque bytes.
var inlinePNG = []byte{0x89, 'P', 'N', 'G', 1, 2, 3, 4}

func htmlWithEmbeddedImage() string {
	return `<p>shot: <img src="data:image/png;base64,` +
		base64.StdEncoding.EncodeToString(inlinePNG) + `"></p>`
}

func buildInlineMessage(t *testing.T, attachments []domain.Attachment) string {
	t.Helper()
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:        addr(t, "", "me@example.com"),
		To:          []domain.EmailAddress{addr(t, "", "a@example.com")},
		Subject:     "With image",
		Body:        "shot:",
		HTMLBody:    htmlWithEmbeddedImage(),
		Attachments: attachments,
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	return string(BuildMIME(msg, time.Unix(0, 0).UTC(), "mid55"))
}

func TestBuildMIMELiftsEmbeddedImageIntoRelatedPart(t *testing.T) {
	out := buildInlineMessage(t, nil)

	wants := []string{
		`Content-Type: multipart/alternative; boundary="=_pigeonpost_mid55"` + "\r\n",
		`Content-Type: multipart/related; boundary="=_pigeonpost_related_mid55"; type="text/html"` + "\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"Content-Type: text/html; charset=utf-8\r\n",
		`Content-Type: image/png; name="image-1.png"` + "\r\n",
		"Content-Transfer-Encoding: base64\r\n",
		`Content-Disposition: inline; filename="image-1.png"` + "\r\n",
		base64.StdEncoding.EncodeToString(inlinePNG),
		"--=_pigeonpost_related_mid55--\r\n",
		"--=_pigeonpost_mid55--\r\n",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n---\n%s", w, out)
		}
	}
	// The HTML must reference the image by cid and carry no data: URI on the wire.
	decoded := decodeQuotedPrintableParts(t, out)
	if !strings.Contains(decoded, `src="cid:`) {
		t.Errorf("HTML lacks a cid reference\n---\n%s", decoded)
	}
	if strings.Contains(decoded, "data:image/") {
		t.Errorf("HTML still carries a data URI\n---\n%s", decoded)
	}
	// The Content-ID header must match the cid the HTML references.
	cidRef := decoded[strings.Index(decoded, `src="cid:`)+len(`src="cid:`):]
	cidRef = cidRef[:strings.Index(cidRef, `"`)]
	if !strings.Contains(out, "Content-ID: <"+cidRef+">\r\n") {
		t.Errorf("Content-ID header does not match cid reference %q\n---\n%s", cidRef, out)
	}
	// The plain variant must precede the related variant (least-to-most rich per RFC 2046).
	if strings.Index(out, "text/plain") > strings.Index(out, "multipart/related") {
		t.Error("plain variant must precede the related variant")
	}
}

func TestBuildMIMEEmbeddedImageAlongsideAttachment(t *testing.T) {
	attachment, err := domain.NewAttachment("report.txt", "text/plain", []byte("hello"))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	out := buildInlineMessage(t, []domain.Attachment{attachment})

	wants := []string{
		`Content-Type: multipart/mixed; boundary="=_pigeonpost_mixed_mid55"` + "\r\n",
		`Content-Type: multipart/alternative; boundary="=_pigeonpost_mid55"` + "\r\n",
		`Content-Type: multipart/related; boundary="=_pigeonpost_related_mid55"; type="text/html"` + "\r\n",
		`Content-Disposition: inline; filename="image-1.png"` + "\r\n",
		`Content-Disposition: attachment; filename="report.txt"` + "\r\n",
		"--=_pigeonpost_mixed_mid55--\r\n",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n---\n%s", w, out)
		}
	}
	// The inline image lives inside the related body, before the ordinary attachment part.
	if strings.Index(out, "image-1.png") > strings.Index(out, "report.txt") {
		t.Error("inline image part must precede the attachment part")
	}
}

func TestBuildMIMEWithoutEmbeddedImagesKeepsPlainAlternative(t *testing.T) {
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:     addr(t, "", "me@example.com"),
		To:       []domain.EmailAddress{addr(t, "", "a@example.com")},
		Subject:  "No image",
		Body:     "hi",
		HTMLBody: "<p>hi</p>",
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	out := string(BuildMIME(msg, time.Unix(0, 0).UTC(), "mid56"))
	if strings.Contains(out, "multipart/related") {
		t.Errorf("unexpected related body for image-free HTML\n---\n%s", out)
	}
}
