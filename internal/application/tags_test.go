package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func testTag(t *testing.T, id string) domain.Tag {
	t.Helper()
	colour, err := domain.NewColour("#ff8800")
	if err != nil {
		t.Fatalf("build colour: %v", err)
	}
	tag, err := domain.NewTag(id, "Important", colour)
	if err != nil {
		t.Fatalf("build tag: %v", err)
	}
	return tag
}

func TestTagServiceList(t *testing.T) {
	store := newFakeTagStore()
	store.tags["t1"] = testTag(t, "t1")
	svc := NewTagService(store)

	tags, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 || tags[0].ID() != "t1" {
		t.Fatalf("List returned %+v", tags)
	}

	store.listErr = errBoom
	if _, err := svc.List(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("List error = %v, want wrapped boom", err)
	}
}

func TestTagServiceSave(t *testing.T) {
	store := newFakeTagStore()
	svc := NewTagService(store)

	if err := svc.Save(context.Background(), testTag(t, "t1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.tags["t1"]; !ok {
		t.Error("tag not saved")
	}

	store.saveErr = errBoom
	if err := svc.Save(context.Background(), testTag(t, "t2")); !errors.Is(err, errBoom) {
		t.Errorf("Save error = %v, want wrapped boom", err)
	}
}

func TestTagServiceDelete(t *testing.T) {
	store := newFakeTagStore()
	store.tags["t1"] = testTag(t, "t1")
	svc := NewTagService(store)

	if err := svc.Delete(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.tags["t1"]; ok {
		t.Error("tag not deleted")
	}

	store.deleteErr = errBoom
	if err := svc.Delete(context.Background(), "t1"); !errors.Is(err, errBoom) {
		t.Errorf("Delete error = %v, want wrapped boom", err)
	}
}

func TestTagServiceForMessage(t *testing.T) {
	store := newFakeTagStore()
	store.tags["t1"] = testTag(t, "t1")
	store.byMessage["m1"] = []string{"t1"}
	svc := NewTagService(store)

	tags, err := svc.ForMessage(context.Background(), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 || tags[0].ID() != "t1" {
		t.Fatalf("ForMessage returned %+v", tags)
	}

	store.forMsgErr = errBoom
	if _, err := svc.ForMessage(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("ForMessage error = %v, want wrapped boom", err)
	}
}

func TestTagServiceColoursForMessages(t *testing.T) {
	store := newFakeTagStore()
	store.tags["t1"] = testTag(t, "t1")
	store.byMessage["m1"] = []string{"t1"}
	svc := NewTagService(store)

	colours, err := svc.ColoursForMessages(context.Background(), []string{"m1", "m2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := store.tags["t1"].Colour().Hex()
	if len(colours["m1"]) != 1 || colours["m1"][0] != want {
		t.Fatalf("ColoursForMessages returned %+v, want [%s] for m1", colours, want)
	}
	if len(colours["m2"]) != 0 {
		t.Errorf("expected no colours for m2, got %+v", colours["m2"])
	}

	store.forMsgErr = errBoom
	if _, err := svc.ColoursForMessages(context.Background(), []string{"m1"}); !errors.Is(err, errBoom) {
		t.Errorf("ColoursForMessages error = %v, want wrapped boom", err)
	}
}

func TestTagServiceAssign(t *testing.T) {
	store := newFakeTagStore()
	svc := NewTagService(store)

	if err := svc.Assign(context.Background(), "m1", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.byMessage["m1"]) != 1 {
		t.Error("tag not assigned")
	}

	store.addErr = errBoom
	if err := svc.Assign(context.Background(), "m1", "t2"); !errors.Is(err, errBoom) {
		t.Errorf("Assign error = %v, want wrapped boom", err)
	}
}

func TestTagServiceUnassign(t *testing.T) {
	store := newFakeTagStore()
	store.byMessage["m1"] = []string{"t1"}
	svc := NewTagService(store)

	if err := svc.Unassign(context.Background(), "m1", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.byMessage["m1"]) != 0 {
		t.Error("tag not unassigned")
	}

	store.removeErr = errBoom
	if err := svc.Unassign(context.Background(), "m1", "t1"); !errors.Is(err, errBoom) {
		t.Errorf("Unassign error = %v, want wrapped boom", err)
	}
}
