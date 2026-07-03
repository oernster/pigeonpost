package domain

import (
	"regexp"
	"strings"
)

var hexColourPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Colour is a validated #rrggbb hex colour used for message tags.
type Colour struct {
	hex string
}

// NewColour validates and normalises a hex colour to lower case.
func NewColour(hex string) (Colour, error) {
	trimmed := strings.TrimSpace(hex)
	if !hexColourPattern.MatchString(trimmed) {
		return Colour{}, ErrInvalidColour
	}
	return Colour{hex: strings.ToLower(trimmed)}, nil
}

// Hex returns the normalised #rrggbb value.
func (c Colour) Hex() string { return c.hex }

// IsZero reports whether this is the empty value.
func (c Colour) IsZero() bool { return c.hex == "" }
