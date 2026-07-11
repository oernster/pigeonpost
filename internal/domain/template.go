package domain

import "strings"

// Template is a reusable message skeleton the user inserts while composing: a name to pick it by, a
// subject and an HTML body. It is immutable once constructed.
type Template struct {
	id      string
	name    string
	subject string
	body    string
}

// NewTemplate validates and constructs a template. The id and name must be non-empty; the subject and
// body may be empty (a template can carry a body without a subject, or the reverse). All fields are
// trimmed.
func NewTemplate(id, name, subject, body string) (Template, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Template{}, ErrEmptyTemplateID
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, ErrEmptyTemplateName
	}
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	return Template{id: id, name: name, subject: subject, body: body}, nil
}

// ID returns the template identifier.
func (t Template) ID() string { return t.id }

// Name returns the template name.
func (t Template) Name() string { return t.name }

// Subject returns the template subject.
func (t Template) Subject() string { return t.subject }

// Body returns the template HTML body.
func (t Template) Body() string { return t.body }
