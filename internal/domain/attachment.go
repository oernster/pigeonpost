package domain

import "strings"

// defaultAttachmentContentType is used when a caller does not know an attachment's media type.
const defaultAttachmentContentType = "application/octet-stream"

const (
	// BytesPerMebibyte converts the limit below between the unit a person reads and the unit a byte
	// count is measured in.
	BytesPerMebibyte = 1 << 20
	// MaxAttachmentMebibytes is the largest total a single outgoing message may carry: its attachments
	// plus the images embedded in its HTML body, which ship as message parts just as attachments do.
	MaxAttachmentMebibytes = 25
	// MaxTotalAttachmentBytes is that limit in bytes. It lives in the domain because two surfaces now
	// hold to it: the send path, which refuses an oversized message; and a template, which refuses to
	// store more than a message could ever carry rather than waiting to fail at the send.
	MaxTotalAttachmentBytes = MaxAttachmentMebibytes * BytesPerMebibyte
)

// Attachment is a file carried by an outgoing message: its display filename, MIME content type and raw
// bytes. It is immutable once constructed.
type Attachment struct {
	filename    string
	contentType string
	content     []byte
}

// NewAttachment builds an attachment. A filename is required; an empty content type defaults to a
// generic binary type. The content bytes are copied so the attachment cannot be mutated afterwards.
func NewAttachment(filename, contentType string, content []byte) (Attachment, error) {
	if strings.TrimSpace(filename) == "" {
		return Attachment{}, ErrEmptyAttachmentName
	}
	resolved := strings.TrimSpace(contentType)
	if resolved == "" {
		resolved = defaultAttachmentContentType
	}
	return Attachment{
		filename:    filename,
		contentType: resolved,
		content:     append([]byte(nil), content...),
	}, nil
}

// Filename returns the attachment's display filename.
func (a Attachment) Filename() string { return a.filename }

// ContentType returns the attachment's MIME content type.
func (a Attachment) ContentType() string { return a.contentType }

// Content returns a copy of the attachment's raw bytes.
func (a Attachment) Content() []byte { return append([]byte(nil), a.content...) }

// Size returns the attachment's size in bytes without copying its content.
func (a Attachment) Size() int { return len(a.content) }
