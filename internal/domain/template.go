package domain

import "strings"

// TemplateAttachment describes a file a template carries, without its bytes: the display filename, the
// MIME content type and the size. It is deliberately separate from Attachment, which holds content.
//
// Listing the templates must stay cheap, since the compose window loads them all to fill its picker.
// A template may carry up to a whole message's worth of files, so a list that carried content would
// read every byte of every template to show a menu of names. The bytes are fetched for one template at
// the moment it is used or edited (see the TemplateStore port); this is what the rest of the
// application passes around.
type TemplateAttachment struct {
	filename    string
	contentType string
	size        int
}

// NewTemplateAttachment validates and constructs the description of a template's file. A filename is
// required; an empty content type defaults to a generic binary type, as it does for an Attachment.
func NewTemplateAttachment(filename, contentType string, size int) (TemplateAttachment, error) {
	if strings.TrimSpace(filename) == "" {
		return TemplateAttachment{}, ErrEmptyAttachmentName
	}
	resolved := strings.TrimSpace(contentType)
	if resolved == "" {
		resolved = defaultAttachmentContentType
	}
	return TemplateAttachment{filename: filename, contentType: resolved, size: size}, nil
}

// describeAttachment reduces a file to what a template stores about it.
func describeAttachment(a Attachment) TemplateAttachment {
	return TemplateAttachment{filename: a.Filename(), contentType: a.ContentType(), size: a.Size()}
}

// Filename returns the attachment's display filename.
func (a TemplateAttachment) Filename() string { return a.filename }

// ContentType returns the attachment's MIME content type.
func (a TemplateAttachment) ContentType() string { return a.contentType }

// Size returns the attachment's size in bytes.
func (a TemplateAttachment) Size() int { return a.size }

// Template is a reusable message skeleton the user inserts while composing: a name to pick it by, a
// subject, an HTML body and the files that go with it. It is immutable once constructed.
type Template struct {
	id          string
	name        string
	subject     string
	body        string
	attachments []TemplateAttachment
}

// NewTemplate validates and constructs a template that carries no files. The id and name must be
// non-empty; either the subject or the body may be empty, so a template can carry a body with no
// subject or a subject with no body. All string fields are trimmed.
func NewTemplate(id, name, subject, body string) (Template, error) {
	return NewTemplateWithAttachments(id, name, subject, body, nil)
}

// NewTemplateWithAttachments is NewTemplate for a template that carries files, described rather than
// held. The descriptions are copied, so the caller's slice cannot mutate the template afterwards.
//
// Their total is held to the same limit a single message is: a template carrying more than that could
// be written and then fail at every send, which is the shape of failure this application refuses
// elsewhere (a rule that can never act is stored switched off rather than left looking live). Refusing
// the save states the problem while the user is still looking at it.
func NewTemplateWithAttachments(
	id, name, subject, body string, attachments []TemplateAttachment,
) (Template, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Template{}, ErrEmptyTemplateID
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, ErrEmptyTemplateName
	}
	total := 0
	for _, a := range attachments {
		total += a.Size()
	}
	if total > MaxTotalAttachmentBytes {
		return Template{}, ErrTemplateAttachmentsTooLarge
	}
	return Template{
		id:          id,
		name:        name,
		subject:     strings.TrimSpace(subject),
		body:        strings.TrimSpace(body),
		attachments: append([]TemplateAttachment(nil), attachments...),
	}, nil
}

// NewTemplateFromFiles builds a template from the files themselves, describing each one. It is what the
// save path uses, so the descriptions a template stores can never disagree with the bytes stored beside
// them: both come from the same slice in the same order.
func NewTemplateFromFiles(id, name, subject, body string, files []Attachment) (Template, error) {
	described := make([]TemplateAttachment, 0, len(files))
	for _, f := range files {
		described = append(described, describeAttachment(f))
	}
	return NewTemplateWithAttachments(id, name, subject, body, described)
}

// ID returns the template identifier.
func (t Template) ID() string { return t.id }

// Name returns the template name.
func (t Template) Name() string { return t.name }

// Subject returns the template subject.
func (t Template) Subject() string { return t.subject }

// Body returns the template HTML body.
func (t Template) Body() string { return t.body }

// Attachments returns a copy of the descriptions of the files the template carries, in stored order.
func (t Template) Attachments() []TemplateAttachment {
	return append([]TemplateAttachment(nil), t.attachments...)
}

// AttachmentBytes reports the total size of the template's attachments.
func (t Template) AttachmentBytes() int {
	total := 0
	for _, a := range t.attachments {
		total += a.Size()
	}
	return total
}
