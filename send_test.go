package main

import (
	"encoding/base64"
	"strings"
	"testing"
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
