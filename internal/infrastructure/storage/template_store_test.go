package storage

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func buildTemplate(t *testing.T, id, name, subject, body string) domain.Template {
	t.Helper()
	template, err := domain.NewTemplate(id, name, subject, body)
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	return template
}

func TestTemplateRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveTemplate(ctx, buildTemplate(t, "t1", "Welcome", "Hello", "<p>Hi</p>")); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := store.SaveTemplate(ctx, buildTemplate(t, "t2", "Follow up", "Checking in", "<p>Any news?</p>")); err != nil {
		t.Fatalf("save template: %v", err)
	}

	templates, err := store.ListTemplates(ctx)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	// Ordered by name, so "Follow up" precedes "Welcome".
	if len(templates) != 2 || templates[0].Name() != "Follow up" {
		t.Fatalf("expected 2 templates ordered by name, got %+v", templates)
	}
	if templates[1].Subject() != "Hello" || templates[1].Body() != "<p>Hi</p>" {
		t.Errorf("template fields lost in round trip: %+v", templates[1])
	}

	// Saving the same id replaces rather than accumulates.
	if err := store.SaveTemplate(ctx, buildTemplate(t, "t1", "Welcome again", "Hi", "<p>Hey</p>")); err != nil {
		t.Fatalf("re-save template: %v", err)
	}
	templates, _ = store.ListTemplates(ctx)
	if len(templates) != 2 {
		t.Fatalf("expected replace to keep 2 templates, got %d", len(templates))
	}

	if err := store.DeleteTemplate(ctx, "t1"); err != nil {
		t.Fatalf("delete template: %v", err)
	}
	templates, _ = store.ListTemplates(ctx)
	if len(templates) != 1 || templates[0].ID() != "t2" {
		t.Fatalf("expected only t2 left, got %+v", templates)
	}

	// Deleting an absent template is not an error.
	if err := store.DeleteTemplate(ctx, "missing"); err != nil {
		t.Errorf("delete missing template: %v", err)
	}
}
