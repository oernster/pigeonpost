package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// CalendarInput carries the fields to create or update a calendar. An empty ID means a new calendar.
type CalendarInput struct {
	ID     string
	Name   string
	Colour string
}

// EventInput carries the fields to create or update an event. An empty ID means a new event; times are
// supplied as already-resolved values.
type EventInput struct {
	ID          string
	UID         string
	CalendarID  string
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	AllDay      bool
	Recurrence  string
}

// CalendarService is the use-case boundary for managing calendars and their events.
type CalendarService struct {
	store CalendarStore
	newID IDGenerator
}

// NewCalendarService constructs the service with its injected store and id generator.
func NewCalendarService(store CalendarStore, newID IDGenerator) *CalendarService {
	return &CalendarService{store: store, newID: newID}
}

// ListCalendars returns all calendars.
func (s *CalendarService) ListCalendars(ctx context.Context) ([]domain.Calendar, error) {
	calendars, err := s.store.ListCalendars(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar: list calendars: %w", err)
	}
	return calendars, nil
}

// SaveCalendar validates and persists a calendar, generating an id when one is not supplied.
func (s *CalendarService) SaveCalendar(ctx context.Context, in CalendarInput) error {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.newID()
	}
	calendar, err := domain.NewCalendar(id, in.Name, in.Colour)
	if err != nil {
		return fmt.Errorf("calendar: build calendar: %w", err)
	}
	if err := s.store.SaveCalendar(ctx, calendar); err != nil {
		return fmt.Errorf("calendar: save calendar: %w", err)
	}
	return nil
}

// DeleteCalendar removes a calendar by id.
func (s *CalendarService) DeleteCalendar(ctx context.Context, id string) error {
	if err := s.store.DeleteCalendar(ctx, id); err != nil {
		return fmt.Errorf("calendar: delete calendar %q: %w", id, err)
	}
	return nil
}

// ListEvents returns all events.
func (s *CalendarService) ListEvents(ctx context.Context) ([]domain.Event, error) {
	events, err := s.store.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar: list events: %w", err)
	}
	return events, nil
}

// GetEvent returns a single event by id.
func (s *CalendarService) GetEvent(ctx context.Context, id string) (domain.Event, error) {
	event, err := s.store.GetEvent(ctx, id)
	if err != nil {
		return domain.Event{}, fmt.Errorf("calendar: get event %q: %w", id, err)
	}
	return event, nil
}

// SaveEvent validates and persists an event, generating an id when one is not supplied.
func (s *CalendarService) SaveEvent(ctx context.Context, in EventInput) error {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.newID()
	}
	event, err := domain.NewEvent(domain.EventInput{
		ID:          id,
		UID:         in.UID,
		CalendarID:  in.CalendarID,
		Summary:     in.Summary,
		Description: in.Description,
		Location:    in.Location,
		Start:       in.Start,
		End:         in.End,
		AllDay:      in.AllDay,
		Recurrence:  in.Recurrence,
	})
	if err != nil {
		return fmt.Errorf("calendar: build event: %w", err)
	}
	if err := s.store.SaveEvent(ctx, event); err != nil {
		return fmt.Errorf("calendar: save event: %w", err)
	}
	return nil
}

// DeleteEvent removes an event by id.
func (s *CalendarService) DeleteEvent(ctx context.Context, id string) error {
	if err := s.store.DeleteEvent(ctx, id); err != nil {
		return fmt.Errorf("calendar: delete event %q: %w", id, err)
	}
	return nil
}

// ImportEvents decodes events from the given bytes with the codec and saves each. A decoded event keeps
// its own id (an ICS UID where present), so re-importing updates the matching events rather than
// duplicating them. It returns the number imported.
func (s *CalendarService) ImportEvents(ctx context.Context, codec CalendarCodec, data []byte) (int, error) {
	events, err := codec.Decode(data)
	if err != nil {
		return 0, fmt.Errorf("calendar: decode import: %w", err)
	}
	for i, e := range events {
		if err := s.store.SaveEvent(ctx, e); err != nil {
			return i, fmt.Errorf("calendar: import save %q: %w", e.ID(), err)
		}
	}
	return len(events), nil
}

// ExportEvents encodes every event with the codec into its serialised form.
func (s *CalendarService) ExportEvents(ctx context.Context, codec CalendarCodec) ([]byte, error) {
	events, err := s.store.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar: list for export: %w", err)
	}
	data, err := codec.Encode(events)
	if err != nil {
		return nil, fmt.Errorf("calendar: encode export: %w", err)
	}
	return data, nil
}
