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
