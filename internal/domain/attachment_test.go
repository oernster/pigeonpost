package domain

import (
	"errors"
	"testing"
)

func TestNewAttachment(t *testing.T) {
	content := []byte("hello")
	a, err := NewAttachment("note.txt", "text/plain", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Filename() != "note.txt" {
		t.Errorf("Filename = %q", a.Filename())
	}
	if a.ContentType() != "text/plain" {
		t.Errorf("ContentType = %q", a.ContentType())
	}
	if string(a.Content()) != "hello" {
		t.Errorf("Content = %q", a.Content())
	}

	// The stored content is a copy: mutating the caller's slice does not change the attachment, and
	// the getter itself returns a copy.
	content[0] = 'H'
	if string(a.Content()) != "hello" {
		t.Errorf("attachment content shares the caller's slice: %q", a.Content())
	}
	got := a.Content()
	got[0] = 'X'
	if string(a.Content()) != "hello" {
		t.Errorf("Content getter shares state: %q", a.Content())
	}
}

func TestNewAttachmentDefaultsContentType(t *testing.T) {
	a, err := NewAttachment("blob", "  ", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ContentType() != defaultAttachmentContentType {
		t.Errorf("ContentType = %q, want the default %q", a.ContentType(), defaultAttachmentContentType)
	}
}

func TestNewAttachmentEmptyName(t *testing.T) {
	if _, err := NewAttachment("   ", "text/plain", []byte("x")); !errors.Is(err, ErrEmptyAttachmentName) {
		t.Errorf("error = %v, want ErrEmptyAttachmentName", err)
	}
}
