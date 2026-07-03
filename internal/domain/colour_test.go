package domain

import (
	"errors"
	"testing"
)

func TestNewColourValid(t *testing.T) {
	c, err := NewColour("  #AABBCC  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Hex() != "#aabbcc" {
		t.Errorf("Hex = %q, want #aabbcc", c.Hex())
	}
	if c.IsZero() {
		t.Error("valid colour reported IsZero")
	}
}

func TestNewColourInvalid(t *testing.T) {
	for _, in := range []string{"", "aabbcc", "#GGGGGG", "#123", "#1234567", "red"} {
		if _, err := NewColour(in); !errors.Is(err, ErrInvalidColour) {
			t.Errorf("NewColour(%q) error = %v, want ErrInvalidColour", in, err)
		}
	}
}

func TestColourZeroValue(t *testing.T) {
	var c Colour
	if !c.IsZero() {
		t.Error("zero value should report IsZero")
	}
}
