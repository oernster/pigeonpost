package domain

import (
	"errors"
	"testing"
)

func TestNewCalendarValidatesRequiredFields(t *testing.T) {
	if _, err := NewCalendar("  ", "Work", ""); !errors.Is(err, ErrEmptyCalendarID) {
		t.Errorf("blank id err = %v, want ErrEmptyCalendarID", err)
	}
	if _, err := NewCalendar("cal1", "  ", ""); !errors.Is(err, ErrEmptyCalendarName) {
		t.Errorf("blank name err = %v, want ErrEmptyCalendarName", err)
	}
}

func TestNewCalendarRejectsBadColour(t *testing.T) {
	if _, err := NewCalendar("cal1", "Work", "not-a-colour"); !errors.Is(err, ErrInvalidColour) {
		t.Errorf("bad colour err = %v, want ErrInvalidColour", err)
	}
}

func TestNewCalendarTrimsAndExposesFields(t *testing.T) {
	c, err := NewCalendar("  cal1 ", "  Work ", "  #ff8800 ")
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}
	if c.ID() != "cal1" || c.Name() != "Work" || c.Colour() != "#ff8800" {
		t.Errorf("fields not trimmed/exposed: %q / %q / %q", c.ID(), c.Name(), c.Colour())
	}
}

func TestNewCalendarAllowsEmptyColour(t *testing.T) {
	c, err := NewCalendar("cal1", "Personal", "")
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}
	if c.Colour() != "" {
		t.Errorf("colour = %q, want empty", c.Colour())
	}
}
