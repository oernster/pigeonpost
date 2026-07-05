package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/ics"
)

// CalendarDTO is the JSON-serialisable view of a calendar.
type CalendarDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Colour string `json:"colour"`
}

// CalendarRequest is the front-end payload for creating or updating a calendar. An empty id means a new
// calendar.
type CalendarRequest struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Colour string `json:"colour"`
}

// EventDTO is the JSON-serialisable view of an event. Times are RFC 3339 strings; end is empty when the
// event has no end.
type EventDTO struct {
	ID          string `json:"id"`
	UID         string `json:"uid"`
	CalendarID  string `json:"calendarId"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"allDay"`
	Recurrence  string `json:"recurrence"`
	Extra       string `json:"extra"`
}

// EventRequest is the front-end payload for creating or updating an event. An empty id means a new
// event; Start is required and End may be empty. Extra is the opaque preserved ICS, round-tripped
// unchanged so an edit does not strip unmodelled properties.
type EventRequest struct {
	ID          string `json:"id"`
	UID         string `json:"uid"`
	CalendarID  string `json:"calendarId"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"allDay"`
	Recurrence  string `json:"recurrence"`
	Extra       string `json:"extra"`
}

// ListCalendars returns every calendar.
func (a *App) ListCalendars() ([]CalendarDTO, error) {
	calendars, err := a.calendar.ListCalendars(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CalendarDTO, 0, len(calendars))
	for _, c := range calendars {
		out = append(out, CalendarDTO{ID: c.ID(), Name: c.Name(), Colour: c.Colour()})
	}
	return out, nil
}

// SaveCalendar creates or updates a calendar.
func (a *App) SaveCalendar(req CalendarRequest) error {
	return a.calendar.SaveCalendar(a.ctx, application.CalendarInput{ID: req.ID, Name: req.Name, Colour: req.Colour})
}

// DeleteCalendar removes a calendar and its events by id.
func (a *App) DeleteCalendar(id string) error {
	return a.calendar.DeleteCalendar(a.ctx, id)
}

// ListEvents returns every event.
func (a *App) ListEvents() ([]EventDTO, error) {
	events, err := a.calendar.ListEvents(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]EventDTO, 0, len(events))
	for _, e := range events {
		out = append(out, toEventDTO(e))
	}
	return out, nil
}

// GetEvent returns a single event by id.
func (a *App) GetEvent(id string) (EventDTO, error) {
	e, err := a.calendar.GetEvent(a.ctx, id)
	if err != nil {
		return EventDTO{}, err
	}
	return toEventDTO(e), nil
}

// SaveEvent creates or updates an event.
func (a *App) SaveEvent(req EventRequest) error {
	start, err := parseEventTime(req.Start)
	if err != nil {
		return err
	}
	end, err := parseOptionalEventTime(req.End)
	if err != nil {
		return err
	}
	return a.calendar.SaveEvent(a.ctx, application.EventInput{
		ID:          req.ID,
		UID:         req.UID,
		CalendarID:  req.CalendarID,
		Summary:     req.Summary,
		Description: req.Description,
		Location:    req.Location,
		Start:       start,
		End:         end,
		AllDay:      req.AllDay,
		Recurrence:  req.Recurrence,
		Extra:       req.Extra,
	})
}

// DeleteEvent removes an event by id.
func (a *App) DeleteEvent(id string) error {
	return a.calendar.DeleteEvent(a.ctx, id)
}

// ImportEventsFromFile opens a native file dialog, reads the chosen .ics file and imports its events,
// returning the number imported. A cancelled dialog is a no-op returning zero.
func (a *App) ImportEventsFromFile() (int, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import calendar",
		Filters: []runtime.FileFilter{{DisplayName: "Calendar (*.ics)", Pattern: "*.ics"}},
	})
	if err != nil {
		return 0, fmt.Errorf("import calendar dialog: %w", err)
	}
	if path == "" {
		return 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read calendar file %q: %w", path, err)
	}
	return a.calendar.ImportEvents(a.ctx, ics.New(), data)
}

// ExportEventsToFile encodes every event as iCalendar and writes it to a file the user chooses through
// a native save dialog. It returns true when a file was written and false when the dialog was cancelled.
func (a *App) ExportEventsToFile() (bool, error) {
	data, err := a.calendar.ExportEvents(a.ctx, ics.New())
	if err != nil {
		return false, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "calendar.ics",
		Title:           "Export calendar",
		Filters:         []runtime.FileFilter{{DisplayName: "Calendar (*.ics)", Pattern: "*.ics"}},
	})
	if err != nil {
		return false, fmt.Errorf("export calendar dialog: %w", err)
	}
	if path == "" {
		return false, nil
	}
	if err := os.WriteFile(path, data, messageFileMode); err != nil {
		return false, fmt.Errorf("write calendar file %q: %w", path, err)
	}
	return true, nil
}

// toEventDTO maps a domain event to its DTO, formatting times as RFC 3339.
func toEventDTO(e domain.Event) EventDTO {
	end := ""
	if e.HasEnd() {
		end = e.End().Format(time.RFC3339)
	}
	return EventDTO{
		ID:          e.ID(),
		UID:         e.UID(),
		CalendarID:  e.CalendarID(),
		Summary:     e.Summary(),
		Description: e.Description(),
		Location:    e.Location(),
		Start:       e.Start().Format(time.RFC3339),
		End:         end,
		AllDay:      e.AllDay(),
		Recurrence:  e.Recurrence(),
		Extra:       e.Extra(),
	}
}

// parseEventTime parses a required RFC 3339 time.
func parseEventTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t, nil
}

// parseOptionalEventTime parses an RFC 3339 time, returning the zero time for an empty string.
func parseOptionalEventTime(s string) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, nil
	}
	return parseEventTime(s)
}
