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
