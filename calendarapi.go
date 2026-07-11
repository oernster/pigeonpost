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
	ID          string        `json:"id"`
	UID         string        `json:"uid"`
	CalendarID  string        `json:"calendarId"`
	Summary     string        `json:"summary"`
	Description string        `json:"description"`
	Location    string        `json:"location"`
	Category    string        `json:"category"`
	Start       string        `json:"start"`
	End         string        `json:"end"`
	AllDay      bool          `json:"allDay"`
	Recurrence  string        `json:"recurrence"`
	TimeZone    string        `json:"timeZone"`
	Reminders   []int         `json:"reminders"`
	Extra       string        `json:"extra"`
	Organizer   OrganizerDTO  `json:"organizer"`
	Attendees   []AttendeeDTO `json:"attendees"`
}

// EventRequest is the front-end payload for creating or updating an event. An empty id means a new
// event; Start is required and End may be empty. Extra is the opaque preserved ICS, round-tripped
// unchanged so an edit does not strip unmodelled properties. Organizer and Attendees carry the meeting
// scheduling data; both are empty for an ordinary calendar entry.
type EventRequest struct {
	ID          string        `json:"id"`
	UID         string        `json:"uid"`
	CalendarID  string        `json:"calendarId"`
	Summary     string        `json:"summary"`
	Description string        `json:"description"`
	Location    string        `json:"location"`
	Category    string        `json:"category"`
	Start       string        `json:"start"`
	End         string        `json:"end"`
	AllDay      bool          `json:"allDay"`
	Recurrence  string        `json:"recurrence"`
	TimeZone    string        `json:"timeZone"`
	Reminders   []int         `json:"reminders"`
	Extra       string        `json:"extra"`
	Organizer   OrganizerDTO  `json:"organizer"`
	Attendees   []AttendeeDTO `json:"attendees"`
}

// EventInstanceDTO is one concrete occurrence of an event within a queried window. Event is the source
// event (the recurring master, an override, or a one-off); Start and End are this occurrence's times as
// RFC 3339 (End empty when the event has no end); RecurrenceID identifies the occurrence within its
// series (empty for a non-recurring event) and is the value passed back when editing or deleting it.
type EventInstanceDTO struct {
	Event        EventDTO `json:"event"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	RecurrenceID string   `json:"recurrenceId"`
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

// SaveEvent creates or updates an event and returns its id, so a newly created meeting can be acted on
// (for example to send its invitations) without a reload.
func (a *App) SaveEvent(req EventRequest) (string, error) {
	input, err := eventInputFromRequest(req)
	if err != nil {
		return "", err
	}
	return a.calendar.SaveEvent(a.ctx, input)
}

// eventInputFromRequest parses a request's start and end times and maps it to the application EventInput
// that both SaveEvent and SaveEventScoped build, so the field mapping lives in one place rather than two
// identical literals that could drift when a field is added.
func eventInputFromRequest(req EventRequest) (application.EventInput, error) {
	start, err := parseEventTime(req.Start)
	if err != nil {
		return application.EventInput{}, err
	}
	end, err := parseOptionalEventTime(req.End)
	if err != nil {
		return application.EventInput{}, err
	}
	return application.EventInput{
		ID:          req.ID,
		UID:         req.UID,
		CalendarID:  req.CalendarID,
		Summary:     req.Summary,
		Description: req.Description,
		Location:    req.Location,
		Category:    req.Category,
		Start:       start,
		End:         end,
		AllDay:      req.AllDay,
		Recurrence:  req.Recurrence,
		TimeZone:    req.TimeZone,
		Alarms:      remindersToAlarms(req.Reminders),
		Extra:       req.Extra,

		OrganizerAddress: req.Organizer.Address,
		OrganizerName:    req.Organizer.CommonName,
		Attendees:        attendeeInputs(req.Attendees),
	}, nil
}

// DeleteEvent removes an event by id.
func (a *App) DeleteEvent(id string) error {
	return a.calendar.DeleteEvent(a.ctx, id)
}

// ListEventInstances expands every event into its concrete occurrences within the inclusive window
// [from, to] (RFC 3339), sorted by start, so the calendar view can render recurring events.
func (a *App) ListEventInstances(from, to string) ([]EventInstanceDTO, error) {
	fromTime, err := parseEventTime(from)
	if err != nil {
		return nil, err
	}
	toTime, err := parseEventTime(to)
	if err != nil {
		return nil, err
	}
	instances, err := a.calendar.ListEventInstances(a.ctx, fromTime, toTime)
	if err != nil {
		return nil, err
	}
	out := make([]EventInstanceDTO, 0, len(instances))
	for _, instance := range instances {
		out = append(out, toEventInstanceDTO(instance))
	}
	return out, nil
}

// SaveEventScoped applies an edit to a recurring occurrence at the given scope (0 this, 1 this and
// future, 2 all). occurrence is the RFC 3339 original start of the instance being edited (its
// recurrenceId); req carries the edited fields and its id identifies the series.
func (a *App) SaveEventScoped(req EventRequest, scope int, occurrence string) error {
	input, err := eventInputFromRequest(req)
	if err != nil {
		return err
	}
	occurrenceTime, err := parseEventTime(occurrence)
	if err != nil {
		return err
	}
	return a.calendar.UpdateEventScope(a.ctx, application.EventScope(scope), input, occurrenceTime)
}

// DeleteEventScoped removes a recurring occurrence at the given scope (0 this, 1 this and future, 2 all).
// seriesID is the id of the occurrence's event (master or override); occurrence is its RFC 3339 original
// start.
func (a *App) DeleteEventScoped(scope int, seriesID, occurrence string) error {
	occurrenceTime, err := parseEventTime(occurrence)
	if err != nil {
		return err
	}
	return a.calendar.DeleteEventScope(a.ctx, application.EventScope(scope), seriesID, occurrenceTime)
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
		Category:    e.Category(),
		Start:       e.Start().Format(time.RFC3339),
		End:         end,
		AllDay:      e.AllDay(),
		Recurrence:  e.Recurrence(),
		TimeZone:    e.TimeZone(),
		Reminders:   alarmsToReminders(e.Alarms()),
		Extra:       e.Extra(),
		Organizer:   OrganizerDTO{Address: e.Organizer().Address().Address(), CommonName: e.Organizer().CommonName()},
		Attendees:   toAttendeeDTOs(e.Attendees()),
	}
}

// attendeeInputs maps the front-end attendee DTOs to the application attendee inputs.
func attendeeInputs(dtos []AttendeeDTO) []application.AttendeeInput {
	out := make([]application.AttendeeInput, 0, len(dtos))
	for _, a := range dtos {
		out = append(out, application.AttendeeInput{
			Address: a.Address, CommonName: a.CommonName, Role: a.Role, Status: a.Status, RSVP: a.Rsvp,
		})
	}
	return out
}

// alarmsToReminders renders alarms as whole minutes before the event start (a negative offset before the
// start becomes a positive minutes-before value), the friendly form the front end works in.
func alarmsToReminders(alarms []domain.Alarm) []int {
	reminders := make([]int, 0, len(alarms))
	for _, a := range alarms {
		reminders = append(reminders, int(-a.Offset()/time.Minute))
	}
	return reminders
}

// remindersToAlarms is the inverse of alarmsToReminders: each minutes-before value becomes an alarm whose
// trigger offset is that many minutes before the start.
func remindersToAlarms(reminders []int) []domain.Alarm {
	alarms := make([]domain.Alarm, 0, len(reminders))
	for _, m := range reminders {
		alarms = append(alarms, domain.NewAlarm(time.Duration(-m)*time.Minute))
	}
	return alarms
}

// toEventInstanceDTO maps a domain event instance to its DTO, formatting times as RFC 3339.
func toEventInstanceDTO(i domain.EventInstance) EventInstanceDTO {
	end := ""
	if i.HasEnd() {
		end = i.End().Format(time.RFC3339)
	}
	recurrenceID := ""
	if !i.RecurrenceID().IsZero() {
		recurrenceID = i.RecurrenceID().Format(time.RFC3339)
	}
	return EventInstanceDTO{
		Event:        toEventDTO(i.Event()),
		Start:        i.Start().Format(time.RFC3339),
		End:          end,
		RecurrenceID: recurrenceID,
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
