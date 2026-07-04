package domain

import "strings"

// Calendar is a named collection of events, optionally given a colour for display. It is immutable once
// constructed.
type Calendar struct {
	id     string
	name   string
	colour string
}

// NewCalendar validates and constructs a calendar. The id and name must be non-empty; the colour is
// optional and, when present, must be a valid #rrggbb value.
func NewCalendar(id, name, colour string) (Calendar, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Calendar{}, ErrEmptyCalendarID
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Calendar{}, ErrEmptyCalendarName
	}
	colour = strings.TrimSpace(colour)
	if colour != "" {
		if _, err := NewColour(colour); err != nil {
			return Calendar{}, err
		}
	}
	return Calendar{id: id, name: name, colour: colour}, nil
}

// ID returns the calendar identifier.
func (c Calendar) ID() string { return c.id }

// Name returns the calendar name.
func (c Calendar) Name() string { return c.name }

// Colour returns the optional #rrggbb colour, which may be empty.
func (c Calendar) Colour() string { return c.colour }
