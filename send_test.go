package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func b64(content []byte) string { return base64.StdEncoding.EncodeToString(content) }

func TestDataAttachmentsDecodesBytes(t *testing.T) {
	t.Parallel()
	out, err := dataAttachments([]AttachmentDataEntry{
		{Name: "notes.pdf", ContentType: "application/pdf", Content: b64([]byte{1, 2, 3})},
		{Name: "plain.bin", Content: b64([]byte{4})},
	})
	if err != nil {
		t.Fatalf("dataAttachments: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("attachments = %d, want 2", len(out))
	}
	if out[0].Filename() != "notes.pdf" || out[0].ContentType() != "application/pdf" || out[0].Size() != 3 {
		t.Errorf("first attachment mismatch: %q %q %d", out[0].Filename(), out[0].ContentType(), out[0].Size())
	}
	// The domain defaults an empty content type to a generic binary type.
	if out[1].ContentType() != "application/octet-stream" {
		t.Errorf("defaulted content type = %q", out[1].ContentType())
	}
}

// A reopened draft's attachments go out to the compose window as data entries and come back on send
// through dataAttachments, so the pair must round-trip every byte, the name and the type exactly.
func TestAttachmentDataEntriesRoundTripThroughTheSendPath(t *testing.T) {
	t.Parallel()
	original, err := domain.NewAttachment("photo.jpg", "image/jpeg", []byte{0, 255, 7, 42})
	if err != nil {
		t.Fatalf("NewAttachment: %v", err)
	}
	back, err := dataAttachments(attachmentDataEntries([]domain.Attachment{original}))
	if err != nil {
		t.Fatalf("dataAttachments: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("attachments = %d, want 1", len(back))
	}
	got := back[0]
	if got.Filename() != "photo.jpg" || got.ContentType() != "image/jpeg" ||
		!bytes.Equal(got.Content(), original.Content()) {
		t.Errorf("round trip = %q %q %v, want photo.jpg image/jpeg %v",
			got.Filename(), got.ContentType(), got.Content(), original.Content())
	}
}

func TestAttachmentDataEntriesOfNoneIsAnEmptyList(t *testing.T) {
	t.Parallel()
	// Empty rather than nil, so the front end receives [] and never has to coalesce a null.
	if out := attachmentDataEntries(nil); out == nil || len(out) != 0 {
		t.Errorf("attachmentDataEntries(nil) = %#v, want an empty list", out)
	}
}

func TestDataAttachmentsRejectsBadBase64(t *testing.T) {
	t.Parallel()
	_, err := dataAttachments([]AttachmentDataEntry{{Name: "x.bin", Content: "@@not-base64@@"}})
	if err == nil || !strings.Contains(err.Error(), "decode pasted attachment") {
		t.Fatalf("err = %v, want a decode error naming the attachment", err)
	}
}

func TestDataAttachmentsRejectsAnEmptyName(t *testing.T) {
	t.Parallel()
	_, err := dataAttachments([]AttachmentDataEntry{{Name: "  ", Content: b64([]byte{1})}})
	if err == nil || !strings.Contains(err.Error(), "pasted attachment") {
		t.Fatalf("err = %v, want it to name the pasted attachment", err)
	}
}
