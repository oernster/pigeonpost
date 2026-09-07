package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newTemplateService() (*TemplateService, *fakeTemplateStore) {
	templates := &fakeTemplateStore{}
	return NewTemplateService(templates, func() string { return "generated-id" }), templates
}

func validTemplateInput() TemplateInput {
	return TemplateInput{Name: "Welcome", Subject: "Hello", Body: "<p>Hi</p>"}
}

func TestTemplateList(t *testing.T) {
	svc, store := newTemplateService()
	tpl, _ := domain.NewTemplate("t1", "Welcome", "Hello", "<p>Hi</p>")
	store.templates = []domain.Template{tpl}

	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID() != "t1" {
		t.Errorf("expected t1, got %+v", got)
	}

	store.listErr = errBoom
	if _, err := svc.List(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestTemplateSaveNew(t *testing.T) {
	svc, store := newTemplateService()
	if err := svc.Save(context.Background(), validTemplateInput()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.saved) != 1 || store.saved[0].ID() != "generated-id" {
		t.Errorf("expected a generated id, got %+v", store.saved)
	}
}

func TestTemplateSaveExisting(t *testing.T) {
	svc, store := newTemplateService()
	in := validTemplateInput()
	in.ID = "t7"
	if err := svc.Save(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.saved) != 1 || store.saved[0].ID() != "t7" {
		t.Errorf("expected id t7 kept, got %+v", store.saved)
	}
}

func TestTemplateSaveInvalid(t *testing.T) {
	svc, _ := newTemplateService()
	in := validTemplateInput()
	in.Name = "  "
	if err := svc.Save(context.Background(), in); !errors.Is(err, domain.ErrEmptyTemplateName) {
		t.Errorf("error = %v, want ErrEmptyTemplateName", err)
	}
}

func TestTemplateSaveStoreError(t *testing.T) {
	svc, store := newTemplateService()
	store.saveErr = errBoom
	if err := svc.Save(context.Background(), validTemplateInput()); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestTemplateDelete(t *testing.T) {
	svc, store := newTemplateService()
	if err := svc.Delete(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "t1" {
		t.Errorf("expected delete of t1, got %v", store.deleted)
	}

	store.deleteErr = errBoom
	if err := svc.Delete(context.Background(), "t2"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

// buildFile is a file a template carries, named so a test can tell two of them apart by content.
func buildFile(t *testing.T, name, content string) domain.Attachment {
	t.Helper()
	file, err := domain.NewAttachment(name, "", []byte(content))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	return file
}

func TestTemplateSaveStoresNewFiles(t *testing.T) {
	svc, store := newTemplateService()
	in := validTemplateInput()
	in.AddFiles = []domain.Attachment{buildFile(t, "terms.pdf", "one"), buildFile(t, "logo.png", "two")}

	if err := svc.Save(context.Background(), in); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(store.savedFiles) != 1 || len(store.savedFiles[0]) != 2 {
		t.Fatalf("expected two files saved, got %+v", store.savedFiles)
	}
	// The stored template describes what was written, in the same order, so a listing needs no bytes.
	described := store.saved[0].Attachments()
	if len(described) != 2 || described[0].Filename() != "terms.pdf" || described[1].Filename() != "logo.png" {
		t.Errorf("descriptions do not match the files: %+v", described)
	}
	if described[0].Size() != len("one") {
		t.Errorf("size = %d, want %d", described[0].Size(), len("one"))
	}
}

// An edit names the files it keeps by position rather than sending them back, so this is the path that
// decides whether editing a subject silently drops a template's attachments.
func TestTemplateSaveKeepsStoredFilesByPosition(t *testing.T) {
	svc, store := newTemplateService()
	stored := []domain.Attachment{
		buildFile(t, "first.pdf", "one"),
		buildFile(t, "second.pdf", "two"),
		buildFile(t, "third.pdf", "three"),
	}
	store.files = map[string][]domain.Attachment{"t1": stored}

	in := validTemplateInput()
	in.ID = "t1"
	// Keep the third and the first, in that order, drop the second and add one.
	in.KeepFiles = []int{2, 0}
	in.AddFiles = []domain.Attachment{buildFile(t, "new.pdf", "four")}

	if err := svc.Save(context.Background(), in); err != nil {
		t.Fatalf("save: %v", err)
	}
	var names []string
	for _, f := range store.savedFiles[0] {
		names = append(names, f.Filename())
	}
	want := []string{"third.pdf", "first.pdf", "new.pdf"}
	if len(names) != len(want) {
		t.Fatalf("saved %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("saved %v, want %v", names, want)
		}
	}
}

func TestTemplateSaveRefusesAPositionThatIsNotThere(t *testing.T) {
	svc, store := newTemplateService()
	store.files = map[string][]domain.Attachment{"t1": {buildFile(t, "only.pdf", "one")}}

	in := validTemplateInput()
	in.ID = "t1"
	in.KeepFiles = []int{1}

	if err := svc.Save(context.Background(), in); err == nil {
		t.Fatal("expected a save naming a position that is not stored to fail")
	}
	if len(store.saved) != 0 {
		t.Errorf("nothing should have been saved, got %+v", store.saved)
	}
}

// A new template has no stored files, so a position naming one cannot be resolved against anything.
// Silently ignoring it would save a template missing files the editor was showing.
func TestTemplateSaveRefusesKeptFilesOnANewTemplate(t *testing.T) {
	svc, store := newTemplateService()
	in := validTemplateInput()
	in.KeepFiles = []int{0}

	if err := svc.Save(context.Background(), in); err == nil {
		t.Fatal("expected a new template keeping stored files to fail")
	}
	if len(store.saved) != 0 {
		t.Errorf("nothing should have been saved, got %+v", store.saved)
	}
}

func TestTemplateSaveReportsAFailedReadOfStoredFiles(t *testing.T) {
	svc, store := newTemplateService()
	store.files = map[string][]domain.Attachment{"t1": {buildFile(t, "only.pdf", "one")}}
	store.filesErr = errBoom

	in := validTemplateInput()
	in.ID = "t1"
	in.KeepFiles = []int{0}

	if err := svc.Save(context.Background(), in); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

// A template larger than a message may carry could be written and then fail at every send, so the save
// refuses it while the user is still looking at the editor.
func TestTemplateSaveRefusesFilesLargerThanAMessageMayCarry(t *testing.T) {
	svc, store := newTemplateService()
	oversized, err := domain.NewAttachment("huge.bin", "", make([]byte, domain.MaxTotalAttachmentBytes+1))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}

	in := validTemplateInput()
	in.AddFiles = []domain.Attachment{oversized}

	if err := svc.Save(context.Background(), in); !errors.Is(err, domain.ErrTemplateAttachmentsTooLarge) {
		t.Errorf("error = %v, want the too-large error", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("nothing should have been saved, got %+v", store.saved)
	}
}

func TestTemplateFiles(t *testing.T) {
	svc, store := newTemplateService()
	store.files = map[string][]domain.Attachment{"t1": {buildFile(t, "terms.pdf", "one")}}

	files, err := svc.Files(context.Background(), "t1")
	if err != nil {
		t.Fatalf("files: %v", err)
	}
	if len(files) != 1 || files[0].Filename() != "terms.pdf" {
		t.Fatalf("expected the stored file, got %+v", files)
	}

	store.filesErr = errBoom
	if _, err := svc.Files(context.Background(), "t1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}
