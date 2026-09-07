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

	if err := store.SaveTemplate(ctx, buildTemplate(t, "t1", "Welcome", "Hello", "<p>Hi</p>"), nil); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := store.SaveTemplate(ctx, buildTemplate(t, "t2", "Follow up", "Checking in", "<p>Any news?</p>"), nil); err != nil {
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
	if err := store.SaveTemplate(ctx, buildTemplate(t, "t1", "Welcome again", "Hi", "<p>Hey</p>"), nil); err != nil {
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

// buildTemplateFile is one of a template's files, with content chosen so a byte-for-byte comparison
// after the round trip means something.
func buildTemplateFile(t *testing.T, name, contentType, content string) domain.Attachment {
	t.Helper()
	file, err := domain.NewAttachment(name, contentType, []byte(content))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	return file
}

func TestTemplateAttachmentRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	files := []domain.Attachment{
		buildTemplateFile(t, "terms.pdf", "application/pdf", "the terms"),
		buildTemplateFile(t, "logo.png", "image/png", "\x89PNG not really"),
	}

	if err := store.SaveTemplate(ctx, buildTemplate(t, "t1", "Welcome", "Hello", "<p>Hi</p>"), files); err != nil {
		t.Fatalf("save template: %v", err)
	}

	// The listing describes the files without reading them, which is what keeps the compose picker cheap.
	templates, err := store.ListTemplates(ctx)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	described := templates[0].Attachments()
	if len(described) != 2 || described[0].Filename() != "terms.pdf" || described[1].Filename() != "logo.png" {
		t.Fatalf("descriptions lost or reordered: %+v", described)
	}
	if described[0].ContentType() != "application/pdf" || described[0].Size() != len("the terms") {
		t.Errorf("description wrong: %+v", described[0])
	}

	// The bytes come back only when asked for, in the same order.
	got, err := store.TemplateFiles(ctx, "t1")
	if err != nil {
		t.Fatalf("template files: %v", err)
	}
	if len(got) != 2 || string(got[0].Content()) != "the terms" || string(got[1].Content()) != "\x89PNG not really" {
		t.Fatalf("file bytes lost in round trip: %+v", got)
	}
}

// Saving replaces the files outright rather than merging, since their positions define the stored order.
func TestTemplateSaveReplacesItsFiles(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	template := buildTemplate(t, "t1", "Welcome", "Hello", "<p>Hi</p>")

	if err := store.SaveTemplate(ctx, template, []domain.Attachment{
		buildTemplateFile(t, "old.pdf", "application/pdf", "old"),
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := store.SaveTemplate(ctx, template, []domain.Attachment{
		buildTemplateFile(t, "new.pdf", "application/pdf", "new"),
	}); err != nil {
		t.Fatalf("re-save template: %v", err)
	}

	got, err := store.TemplateFiles(ctx, "t1")
	if err != nil {
		t.Fatalf("template files: %v", err)
	}
	if len(got) != 1 || got[0].Filename() != "new.pdf" {
		t.Fatalf("expected the second save to replace the first, got %+v", got)
	}
}

// A deleted template must take its files with it: an orphaned blob is invisible and never freed.
func TestDeleteTemplateRemovesItsFiles(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveTemplate(ctx, buildTemplate(t, "t1", "Welcome", "Hello", "<p>Hi</p>"),
		[]domain.Attachment{buildTemplateFile(t, "terms.pdf", "application/pdf", "the terms")}); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := store.DeleteTemplate(ctx, "t1"); err != nil {
		t.Fatalf("delete template: %v", err)
	}

	var rows int
	if err := store.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM template_attachment WHERE template_id = ?;", "t1").Scan(&rows); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if rows != 0 {
		t.Errorf("expected the files to go with the template, %d rows left", rows)
	}
}
