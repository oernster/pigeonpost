package domain

import (
	"errors"
	"testing"
)

func TestNewTemplate(t *testing.T) {
	tpl, err := NewTemplate("  t1  ", "  Welcome  ", "  Hello there  ", "  <p>Hi</p>  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.ID() != "t1" || tpl.Name() != "Welcome" {
		t.Errorf("id/name not trimmed: %+v", tpl)
	}
	if tpl.Subject() != "Hello there" || tpl.Body() != "<p>Hi</p>" {
		t.Errorf("subject/body not trimmed: %+v", tpl)
	}
}

func TestNewTemplateEmptySubjectAndBodyAllowed(t *testing.T) {
	tpl, err := NewTemplate("t1", "Blank", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.Subject() != "" || tpl.Body() != "" {
		t.Errorf("expected empty subject and body, got %+v", tpl)
	}
}

func TestNewTemplateInvalid(t *testing.T) {
	cases := map[string]struct {
		id, name string
		want     error
	}{
		"empty id":   {"", "n", ErrEmptyTemplateID},
		"blank id":   {"   ", "n", ErrEmptyTemplateID},
		"empty name": {"t", "", ErrEmptyTemplateName},
		"blank name": {"t", "   ", ErrEmptyTemplateName},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewTemplate(tc.id, tc.name, "s", "b"); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewTemplateAttachment(t *testing.T) {
	t.Parallel()
	described, err := NewTemplateAttachment("terms.pdf", "application/pdf", 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if described.Filename() != "terms.pdf" || described.ContentType() != "application/pdf" || described.Size() != 9 {
		t.Errorf("fields lost: %+v", described)
	}

	// An unknown media type falls back to the generic binary one, as it does for an Attachment, so a
	// description never reaches a message header empty.
	defaulted, err := NewTemplateAttachment("notes", "  ", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if defaulted.ContentType() != defaultAttachmentContentType {
		t.Errorf("content type = %q, want the generic default", defaulted.ContentType())
	}

	if _, err := NewTemplateAttachment("   ", "application/pdf", 1); !errors.Is(err, ErrEmptyAttachmentName) {
		t.Errorf("error = %v, want ErrEmptyAttachmentName", err)
	}
}

// The descriptions a template carries and the bytes stored beside them come from one slice, so they
// cannot disagree about what a template holds.
func TestNewTemplateFromFilesDescribesEachFile(t *testing.T) {
	t.Parallel()
	first, err := NewAttachment("terms.pdf", "application/pdf", []byte("the terms"))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	second, err := NewAttachment("logo.png", "image/png", []byte("png"))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}

	template, err := NewTemplateFromFiles("t1", "Welcome", "Hello", "<p>Hi</p>", []Attachment{first, second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	described := template.Attachments()
	if len(described) != 2 || described[0].Filename() != "terms.pdf" || described[1].Filename() != "logo.png" {
		t.Fatalf("descriptions lost or reordered: %+v", described)
	}
	if described[0].Size() != len("the terms") || described[0].ContentType() != "application/pdf" {
		t.Errorf("description wrong: %+v", described[0])
	}
	if template.AttachmentBytes() != len("the terms")+len("png") {
		t.Errorf("AttachmentBytes = %d, want %d", template.AttachmentBytes(), len("the terms")+len("png"))
	}
}

// The returned slice is a copy, so a caller cannot reach back into the template and change what it
// says it carries.
func TestTemplateAttachmentsAreCopied(t *testing.T) {
	t.Parallel()
	described, err := NewTemplateAttachment("terms.pdf", "application/pdf", 9)
	if err != nil {
		t.Fatalf("description: %v", err)
	}
	template, err := NewTemplateWithAttachments("t1", "Welcome", "", "", []TemplateAttachment{described})
	if err != nil {
		t.Fatalf("template: %v", err)
	}

	got := template.Attachments()
	got[0] = TemplateAttachment{}
	if template.Attachments()[0].Filename() != "terms.pdf" {
		t.Error("mutating the returned slice changed the template")
	}
}

// A template that could never be sent is refused at the save rather than at every send, which is the
// same stance the rules editor takes on a rule that could never act.
func TestNewTemplateRefusesFilesLargerThanAMessageMayCarry(t *testing.T) {
	t.Parallel()
	oversized, err := NewTemplateAttachment("huge.bin", "", MaxTotalAttachmentBytes+1)
	if err != nil {
		t.Fatalf("description: %v", err)
	}

	if _, err := NewTemplateWithAttachments("t1", "Welcome", "", "", []TemplateAttachment{oversized}); !errors.Is(err, ErrTemplateAttachmentsTooLarge) {
		t.Errorf("error = %v, want ErrTemplateAttachmentsTooLarge", err)
	}

	// Exactly at the limit is allowed: the limit is what a message may carry, not one byte less.
	atLimit, err := NewTemplateAttachment("big.bin", "", MaxTotalAttachmentBytes)
	if err != nil {
		t.Fatalf("description: %v", err)
	}
	if _, err := NewTemplateWithAttachments("t1", "Welcome", "", "", []TemplateAttachment{atLimit}); err != nil {
		t.Errorf("a template exactly at the limit was refused: %v", err)
	}
}
