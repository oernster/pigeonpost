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
