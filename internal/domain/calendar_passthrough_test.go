package domain

import (
	"errors"
	"testing"
)

func TestNewCalendarPassthroughValid(t *testing.T) {
	p, err := NewCalendarPassthrough("todo-1", PassthroughToDo, "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.UID() != "todo-1" {
		t.Errorf("UID() = %q, want %q", p.UID(), "todo-1")
	}
	if p.Kind() != PassthroughToDo {
		t.Errorf("Kind() = %q, want %q", p.Kind(), PassthroughToDo)
	}
	if p.Raw() == "" {
		t.Error("Raw() should carry the serialised content")
	}
}

func TestNewCalendarPassthroughAcceptsJournal(t *testing.T) {
	if _, err := NewCalendarPassthrough("j-1", PassthroughJournal, "raw"); err != nil {
		t.Errorf("VJOURNAL should be a valid kind, got %v", err)
	}
}

func TestNewCalendarPassthroughRejectsInvalid(t *testing.T) {
	cases := map[string]struct {
		uid, kind, raw string
		want           error
	}{
		"empty uid":    {"", PassthroughToDo, "raw", ErrEmptyPassthroughUID},
		"unknown kind": {"id", "VEVENT", "raw", ErrUnknownPassthroughKind},
		"empty raw":    {"id", PassthroughToDo, "", ErrEmptyPassthroughRaw},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewCalendarPassthrough(c.uid, c.kind, c.raw)
			if !errors.Is(err, c.want) {
				t.Errorf("error = %v, want %v", err, c.want)
			}
		})
	}
}
