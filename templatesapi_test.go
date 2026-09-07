package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TestTemplateDTOSendsAnArrayNotNull is the rules DTO's lesson applied here: a nil Go slice encodes as
// JSON null while the front-end type declares an array, so reading a length off it throws during
// render. A template with no files is the ordinary case, so this is the ordinary path rather than an
// edge. The check is on the encoded bytes because that is what the front end actually receives.
func TestTemplateDTOSendsAnArrayNotNull(t *testing.T) {
	template, err := domain.NewTemplate("t1", "Welcome", "Hello", "<p>Hi</p>")
	if err != nil {
		t.Fatalf("template: %v", err)
	}

	data, err := json.Marshal(toTemplateDTO(template))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"attachments":[]`) {
		t.Errorf("a template with no files encoded as %s", data)
	}
}

// The description is what draws the chip in the editor, so it must carry the name, the type and the
// size and must NOT carry content: a template may hold a whole message's worth of bytes and the list
// is loaded to fill a menu.
func TestTemplateDTODescribesItsFilesWithoutTheirBytes(t *testing.T) {
	described, err := domain.NewTemplateAttachment("terms.pdf", "application/pdf", 9)
	if err != nil {
		t.Fatalf("description: %v", err)
	}
	template, err := domain.NewTemplateWithAttachments("t1", "Welcome", "Hello", "<p>Hi</p>",
		[]domain.TemplateAttachment{described})
	if err != nil {
		t.Fatalf("template: %v", err)
	}

	data, err := json.Marshal(toTemplateDTO(template))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(data)
	for _, want := range []string{`"filename":"terms.pdf"`, `"contentType":"application/pdf"`, `"size":9`} {
		if !strings.Contains(text, want) {
			t.Errorf("encoded %s, missing %s", text, want)
		}
	}
	if strings.Contains(text, `"content"`) {
		t.Errorf("the description carried content: %s", text)
	}
}

// A template's files reach the compose window in the shape it already attaches pasted files in: a
// name, a content type and base64 content. Stating it on the encoded bytes is what holds the two
// sides together, since nothing type-checks the Go struct against the front end's interface.
func TestTemplateFileDTOWireShape(t *testing.T) {
	file, err := domain.NewAttachment("terms.pdf", "application/pdf", []byte("the terms"))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}

	data, err := json.Marshal(toTemplateFileDTOs([]domain.Attachment{file}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `[{"name":"terms.pdf","contentType":"application/pdf","content":"` +
		base64.StdEncoding.EncodeToString([]byte("the terms")) + `"}]`
	if string(data) != want {
		t.Errorf("encoded %s, want %s", data, want)
	}
}

func TestTemplateFileDTOsAreAnArrayWhenThereAreNone(t *testing.T) {
	data, err := json.Marshal(toTemplateFileDTOs(nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("encoded %s, want []", data)
	}
}
