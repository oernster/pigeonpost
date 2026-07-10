package domain

import (
	"errors"
	"testing"
)

func TestFolderKindString(t *testing.T) {
	cases := map[FolderKind]string{
		FolderInbox:    "inbox",
		FolderSent:     "sent",
		FolderDrafts:   "drafts",
		FolderTrash:    "trash",
		FolderJunk:     "junk",
		FolderArchive:  "archive",
		FolderCustom:   "custom",
		FolderKind(99): "unknown",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("FolderKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestNewFolder(t *testing.T) {
	f, err := NewFolder(" f1 ", " acc1 ", " INBOX/Projects ", FolderCustom, 3, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ID() != "f1" {
		t.Errorf("ID = %q", f.ID())
	}
	if f.AccountID() != "acc1" {
		t.Errorf("AccountID = %q", f.AccountID())
	}
	if f.Path() != "INBOX/Projects" {
		t.Errorf("Path = %q", f.Path())
	}
	if f.Name() != "Projects" {
		t.Errorf("Name = %q, want Projects", f.Name())
	}
	if f.Kind() != FolderCustom {
		t.Errorf("Kind = %v", f.Kind())
	}
	if f.Unread() != 3 || f.Total() != 10 {
		t.Errorf("counts = %d/%d, want 3/10", f.Unread(), f.Total())
	}
}

func TestNewFolderTopLevelName(t *testing.T) {
	f, err := NewFolder("f", "a", "INBOX", FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Name() != "INBOX" {
		t.Errorf("Name = %q, want INBOX", f.Name())
	}
}

func TestNewFolderInvalid(t *testing.T) {
	cases := map[string]struct {
		id, account, path string
		unread, total     int
		want              error
	}{
		"empty id":        {" ", "a", "p", 0, 0, ErrEmptyFolderID},
		"empty account":   {"id", " ", "p", 0, 0, ErrEmptyAccountID},
		"empty path":      {"id", "a", " ", 0, 0, ErrEmptyFolderPath},
		"negative unread": {"id", "a", "p", -1, 0, ErrNegativeCount},
		"negative total":  {"id", "a", "p", 0, -1, ErrNegativeCount},
		"unread > total":  {"id", "a", "p", 5, 2, ErrUnreadExceedsTotal},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewFolder(tc.id, tc.account, tc.path, FolderCustom, tc.unread, tc.total)
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFolderWithCounts(t *testing.T) {
	f, _ := NewFolder("f", "a", "INBOX", FolderInbox, 0, 0)
	updated, err := f.WithCounts(2, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Unread() != 2 || updated.Total() != 5 {
		t.Errorf("counts = %d/%d, want 2/5", updated.Unread(), updated.Total())
	}
	if f.Total() != 0 {
		t.Errorf("original mutated: total = %d", f.Total())
	}
	if _, err := f.WithCounts(9, 1); !errors.Is(err, ErrUnreadExceedsTotal) {
		t.Errorf("invalid WithCounts error = %v", err)
	}
}

func TestFolderRenamedTo(t *testing.T) {
	nested, err := NewFolder("id", "a1", "Parent/Old", FolderCustom, 0, 0)
	if err != nil {
		t.Fatalf("build nested folder: %v", err)
	}
	if got := nested.RenamedTo("New"); got != "Parent/New" {
		t.Errorf("nested RenamedTo = %q, want Parent/New", got)
	}

	top, err := NewFolder("id2", "a1", "Old", FolderCustom, 0, 0)
	if err != nil {
		t.Fatalf("build top folder: %v", err)
	}
	if got := top.RenamedTo("New"); got != "New" {
		t.Errorf("top-level RenamedTo = %q, want New", got)
	}
}

func TestFolderIsSentLike(t *testing.T) {
	cases := map[string]struct {
		path string
		kind FolderKind
		want bool
	}{
		"canonical sent kind":        {"Sent", FolderSent, true},
		"custom named sent messages": {"Sent Messages", FolderCustom, true},
		"custom sent under a parent": {"Money/Sent", FolderCustom, true},
		"case-insensitive name":      {"SENT MAIL", FolderCustom, true},
		"ordinary custom folder":     {"Money", FolderCustom, false},
		"inbox is not sent-like":     {"INBOX", FolderInbox, false},
		"trash named plainly":        {"Trash", FolderTrash, false},
		"sent items name":            {"Sent Items", FolderCustom, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			folder, err := NewFolder("id", "a1", tc.path, tc.kind, 0, 0)
			if err != nil {
				t.Fatalf("build folder: %v", err)
			}
			if got := folder.IsSentLike(); got != tc.want {
				t.Errorf("IsSentLike(%q kind %v) = %v, want %v", tc.path, tc.kind, got, tc.want)
			}
		})
	}
}

func TestFolderWithSpecialUse(t *testing.T) {
	folder, err := NewFolder("id", "a1", "Sent", FolderSent, 0, 0)
	if err != nil {
		t.Fatalf("build folder: %v", err)
	}
	if folder.SpecialUse() {
		t.Error("a new folder must not be marked special-use by default")
	}
	declared := folder.WithSpecialUse(true)
	if !declared.SpecialUse() {
		t.Error("WithSpecialUse(true) must mark the copy")
	}
	if folder.SpecialUse() {
		t.Error("original mutated by WithSpecialUse")
	}
}

func TestFolderMovedUnder(t *testing.T) {
	nested, err := NewFolder("id", "a1", "Parent/Reports", FolderCustom, 0, 0)
	if err != nil {
		t.Fatalf("build nested folder: %v", err)
	}
	if got := nested.MovedUnder("Archive"); got != "Archive/Reports" {
		t.Errorf("MovedUnder(Archive) = %q, want Archive/Reports", got)
	}
	if got := nested.MovedUnder(""); got != "Reports" {
		t.Errorf("MovedUnder(top level) = %q, want Reports", got)
	}
	dotted, err := NewFolderWithSeparator("id2", "a1", "Archived.Debt", ".", FolderCustom, 0, 0)
	if err != nil {
		t.Fatalf("build dotted folder: %v", err)
	}
	if got := dotted.MovedUnder("Money"); got != "Money.Debt" {
		t.Errorf("dotted MovedUnder(Money) = %q, want Money.Debt", got)
	}
}

func TestFolderHasAncestorPath(t *testing.T) {
	folder, err := NewFolder("id", "a1", "Projects/Client", FolderCustom, 0, 0)
	if err != nil {
		t.Fatalf("build folder: %v", err)
	}
	cases := map[string]struct {
		ancestor string
		want     bool
	}{
		"is the folder itself":      {"Projects/Client", true},
		"is a direct ancestor":      {"Projects", true},
		"is an unrelated sibling":   {"Projects/Other", false},
		"is a descendant path":      {"Projects/Client/Deep", false},
		"shares a name prefix only": {"Projects/Cli", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := folder.HasAncestorPath(tc.ancestor); got != tc.want {
				t.Errorf("HasAncestorPath(%q) = %v, want %v", tc.ancestor, got, tc.want)
			}
		})
	}
}

func TestFolderWithSeparator(t *testing.T) {
	// A server using "." as its delimiter must yield the correct leaf name and rename path, not treat
	// the whole dotted path as one name.
	dotted, err := NewFolderWithSeparator("id", "a1", "Archived.Debt", ".", FolderCustom, 0, 0)
	if err != nil {
		t.Fatalf("build dotted folder: %v", err)
	}
	if dotted.Name() != "Debt" {
		t.Errorf("Name = %q, want Debt", dotted.Name())
	}
	if dotted.Separator() != "." {
		t.Errorf("Separator = %q, want .", dotted.Separator())
	}
	if got := dotted.RenamedTo("Loans"); got != "Archived.Loans" {
		t.Errorf("dotted RenamedTo = %q, want Archived.Loans", got)
	}
}

func TestFolderEmptySeparatorDefaults(t *testing.T) {
	folder, err := NewFolderWithSeparator("id", "a1", "Parent/Child", "", FolderCustom, 0, 0)
	if err != nil {
		t.Fatalf("build folder: %v", err)
	}
	if folder.Separator() != "/" {
		t.Errorf("Separator = %q, want the default /", folder.Separator())
	}
	if folder.Name() != "Child" {
		t.Errorf("Name = %q, want Child", folder.Name())
	}
}
