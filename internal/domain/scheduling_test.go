package domain

import (
	"errors"
	"testing"
)

func TestParseMethod(t *testing.T) {
	cases := map[string]Method{
		"PUBLISH":   MethodPublish,
		"request":   MethodRequest,
		"  REPLY  ": MethodReply,
		"CANCEL":    MethodCancel,
	}
	for in, want := range cases {
		got, err := ParseMethod(in)
		if err != nil {
			t.Fatalf("ParseMethod(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseMethod(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "ADD", "COUNTER"} {
		if _, err := ParseMethod(bad); !errors.Is(err, ErrInvalidMethod) {
			t.Errorf("ParseMethod(%q) err = %v, want ErrInvalidMethod", bad, err)
		}
	}
}

func schedulingEvent(t *testing.T, id string) Event {
	t.Helper()
	e, err := NewEvent(EventInput{ID: id, UID: "uid-" + id, Summary: "Sync", Start: eventStart()})
	if err != nil {
		t.Fatalf("NewEvent(%q): %v", id, err)
	}
	return e
}

func TestNewSchedulingMessage(t *testing.T) {
	master := schedulingEvent(t, "e1")
	override := schedulingEvent(t, "e2")
	in := []Event{master, override}
	m, err := NewSchedulingMessage(MethodRequest, in)
	if err != nil {
		t.Fatalf("NewSchedulingMessage: %v", err)
	}
	if m.Method() != MethodRequest {
		t.Errorf("Method() = %q, want REQUEST", m.Method())
	}
	if got := m.Events(); len(got) != 2 || got[0].ID() != "e1" || got[1].ID() != "e2" {
		t.Errorf("Events() = %v", got)
	}
	if m.PrimaryEvent().ID() != "e1" {
		t.Errorf("PrimaryEvent() = %q, want e1", m.PrimaryEvent().ID())
	}
	// The message must not share backing storage with the caller's slice.
	in[0] = override
	if m.PrimaryEvent().ID() != "e1" {
		t.Errorf("message shares backing storage with caller input")
	}
	// A returned slice must not alias the message storage.
	got := m.Events()
	got[0] = override
	if m.PrimaryEvent().ID() != "e1" {
		t.Errorf("returned slice aliases message storage")
	}
}

func TestNewSchedulingMessageValidation(t *testing.T) {
	if _, err := NewSchedulingMessage("ADD", []Event{schedulingEvent(t, "e1")}); !errors.Is(err, ErrInvalidMethod) {
		t.Errorf("bad method err = %v, want ErrInvalidMethod", err)
	}
	if _, err := NewSchedulingMessage(MethodReply, nil); !errors.Is(err, ErrNoSchedulingEvents) {
		t.Errorf("no-events err = %v, want ErrNoSchedulingEvents", err)
	}
}

func TestNewCalendarPart(t *testing.T) {
	content := []byte("BEGIN:VCALENDAR\r\nMETHOD:REPLY\r\nEND:VCALENDAR\r\n")
	part, err := NewCalendarPart(MethodReply, content)
	if err != nil {
		t.Fatalf("NewCalendarPart: %v", err)
	}
	if part.Method() != MethodReply {
		t.Errorf("Method() = %q, want REPLY", part.Method())
	}
	if string(part.Content()) != string(content) {
		t.Errorf("Content() = %q, want the payload", part.Content())
	}
	if part.IsZero() {
		t.Errorf("a part with content reported zero")
	}
	// The part must not share backing storage with the caller's slice.
	content[0] = 'X'
	if part.Content()[0] == 'X' {
		t.Errorf("NewCalendarPart shares backing storage with the caller")
	}
	// A returned slice must not alias the part's storage.
	got := part.Content()
	got[0] = 'Y'
	if part.Content()[0] == 'Y' {
		t.Errorf("Content() returns a slice aliasing the part storage")
	}
}

func TestNewCalendarPartValidation(t *testing.T) {
	if _, err := NewCalendarPart("ADD", []byte("x")); !errors.Is(err, ErrInvalidMethod) {
		t.Errorf("bad method err = %v, want ErrInvalidMethod", err)
	}
	if _, err := NewCalendarPart(MethodReply, nil); !errors.Is(err, ErrEmptyCalendarPart) {
		t.Errorf("empty-content err = %v, want ErrEmptyCalendarPart", err)
	}
	if !(CalendarPart{}).IsZero() {
		t.Errorf("the zero CalendarPart should report IsZero")
	}
}
