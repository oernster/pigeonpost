package domain

import (
	"errors"
	"testing"
)

func TestNewContactGroupValidatesRequiredFields(t *testing.T) {
	if _, err := NewContactGroup(" ", "Friends", nil); !errors.Is(err, ErrEmptyContactGroupID) {
		t.Errorf("blank id err = %v, want ErrEmptyContactGroupID", err)
	}
	if _, err := NewContactGroup("g1", "  ", nil); !errors.Is(err, ErrEmptyContactGroupName) {
		t.Errorf("blank name err = %v, want ErrEmptyContactGroupName", err)
	}
}

func TestNewContactGroupDedupesMembers(t *testing.T) {
	g, err := NewContactGroup("  g1 ", "  Friends ", []string{" c1 ", "c2", "c1", "  ", "c2"})
	if err != nil {
		t.Fatalf("NewContactGroup: %v", err)
	}
	if g.ID() != "g1" || g.Name() != "Friends" {
		t.Errorf("id/name not trimmed: %q / %q", g.ID(), g.Name())
	}
	got := g.Members()
	want := []string{"c1", "c2"}
	if len(got) != len(want) {
		t.Fatalf("members = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("member %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestContactGroupMembersEmptyIsNilCopy(t *testing.T) {
	g, _ := NewContactGroup("g1", "Empty", nil)
	if g.Members() != nil {
		t.Errorf("expected nil members, got %v", g.Members())
	}
}

func TestContactGroupMembersIsCopy(t *testing.T) {
	g, _ := NewContactGroup("g1", "Friends", []string{"c1"})
	got := g.Members()
	got[0] = "hacked"
	if g.Members()[0] != "c1" {
		t.Errorf("group shares its backing array with the returned members slice")
	}
}

func TestContactGroupContains(t *testing.T) {
	g, _ := NewContactGroup("g1", "Friends", []string{"c1", "c2"})
	if !g.Contains("c1") {
		t.Errorf("expected c1 to be a member")
	}
	if g.Contains("c9") {
		t.Errorf("did not expect c9 to be a member")
	}
}

func TestContactGroupWithMember(t *testing.T) {
	g, _ := NewContactGroup("g1", "Friends", []string{"c1"})

	added := g.WithMember(" c2 ")
	if !added.Contains("c2") || len(added.Members()) != 2 {
		t.Errorf("expected c2 added, members = %v", added.Members())
	}
	// The original is unchanged (immutability).
	if g.Contains("c2") {
		t.Errorf("original group mutated by WithMember")
	}
	// Adding an existing member is a no-op returning an equivalent group.
	if same := added.WithMember("c2"); len(same.Members()) != 2 {
		t.Errorf("adding an existing member changed the group: %v", same.Members())
	}
	// Adding a blank id is a no-op.
	if same := g.WithMember("   "); len(same.Members()) != 1 {
		t.Errorf("adding a blank member changed the group: %v", same.Members())
	}
}

func TestContactGroupWithoutMember(t *testing.T) {
	g, _ := NewContactGroup("g1", "Friends", []string{"c1", "c2"})

	removed := g.WithoutMember("c1")
	if removed.Contains("c1") || len(removed.Members()) != 1 || removed.Members()[0] != "c2" {
		t.Errorf("expected c1 removed, members = %v", removed.Members())
	}
	// The original is unchanged.
	if !g.Contains("c1") {
		t.Errorf("original group mutated by WithoutMember")
	}
	// Removing a non-member is a no-op.
	if same := g.WithoutMember("c9"); len(same.Members()) != 2 {
		t.Errorf("removing a non-member changed the group: %v", same.Members())
	}
}
