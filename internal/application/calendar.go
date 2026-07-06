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
	// TimeZone is the IANA name the event's wall-clock times are kept in (empty for floating or UTC).
	TimeZone string
	// Alarms are the event's reminders, each a lead time relative to its start.
	Alarms []domain.Alarm
	// Extra carries the preserved original ICS opaquely so an in-app edit does not strip the
	// properties PigeonPost does not model. The UI round-trips it unchanged.
	Extra string
	// OrganizerAddress and OrganizerName describe the meeting organizer. OrganizerAddress is empty for an
	// event that is not a scheduled meeting.
	OrganizerAddress string
	OrganizerName    string
	// Attendees are the meeting's invited parties. It is empty for an event that is not a meeting.
	Attendees []AttendeeInput
}

// AttendeeInput carries the fields to build one meeting attendee. Address is required; Role and Status
// are ICS values (empty falls back to REQ-PARTICIPANT and NEEDS-ACTION).
type AttendeeInput struct {
	Address    string
	CommonName string
	Role       string
	Status     string
	RSVP       bool
}

// CalendarService is the use-case boundary for managing calendars and their events.
type CalendarService struct {
	store      CalendarStore
	newID      IDGenerator
	recurrence RecurrenceService
}

// NewCalendarService constructs the service with its injected store, id generator and recurrence
// service (used to expand and split recurring events).
func NewCalendarService(store CalendarStore, newID IDGenerator, recurrence RecurrenceService) *CalendarService {
	return &CalendarService{store: store, newID: newID, recurrence: recurrence}
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

// SaveEvent creates or updates an event and returns its id: the given id, or a freshly generated one for
// a new event. The caller needs the id to act on a just-created event, such as sending a meeting's
// invitations.
func (s *CalendarService) SaveEvent(ctx context.Context, in EventInput) (string, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.newID()
	}
	// Every event needs a stable UID: it is the identity a meeting keeps across the iTIP REQUEST, REPLY
	// and CANCEL round-trip (replies are matched to a stored meeting by UID) and the key an ICS export
	// re-imports on. An event created in-app arrives without one, so derive it from the id.
	uid := strings.TrimSpace(in.UID)
	if uid == "" {
		uid = id
	}
	organizer, err := buildOrganizer(in.OrganizerAddress, in.OrganizerName)
	if err != nil {
		return "", fmt.Errorf("calendar: build organizer: %w", err)
	}
	attendees, err := buildAttendees(in.Attendees)
	if err != nil {
		return "", fmt.Errorf("calendar: build attendees: %w", err)
	}
	event, err := domain.NewEvent(domain.EventInput{
		ID:          id,
		UID:         uid,
		CalendarID:  in.CalendarID,
		Summary:     in.Summary,
		Description: in.Description,
		Location:    in.Location,
		Start:       in.Start,
		End:         in.End,
		AllDay:      in.AllDay,
		Recurrence:  in.Recurrence,
		TimeZone:    in.TimeZone,
		Alarms:      in.Alarms,
		Extra:       in.Extra,
		Organizer:   organizer,
		Attendees:   attendees,
	})
	if err != nil {
		return "", fmt.Errorf("calendar: build event: %w", err)
	}
	if err := s.store.SaveEvent(ctx, event); err != nil {
		return "", fmt.Errorf("calendar: save event: %w", err)
	}
	return id, nil
}

// buildOrganizer builds a meeting organizer from an address and optional name, or the zero organizer
// when the address is empty (the event is not a meeting).
func buildOrganizer(address, name string) (domain.Organizer, error) {
	if strings.TrimSpace(address) == "" {
		return domain.Organizer{}, nil
	}
	addr, err := domain.NewEmailAddress("", address)
	if err != nil {
		return domain.Organizer{}, err
	}
	return domain.NewOrganizer(addr, name)
}

// buildAttendees builds the meeting attendees from their inputs, failing on the first invalid one.
func buildAttendees(inputs []AttendeeInput) ([]domain.Attendee, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	out := make([]domain.Attendee, 0, len(inputs))
	for _, in := range inputs {
		attendee, err := buildAttendee(in)
		if err != nil {
			return nil, err
		}
		out = append(out, attendee)
	}
	return out, nil
}

// buildAttendee builds one meeting attendee, validating its address, role and status.
func buildAttendee(in AttendeeInput) (domain.Attendee, error) {
	addr, err := domain.NewEmailAddress("", in.Address)
	if err != nil {
		return domain.Attendee{}, err
	}
	role, err := domain.ParseRole(in.Role)
	if err != nil {
		return domain.Attendee{}, err
	}
	status, err := domain.ParseParticipationStatus(in.Status)
	if err != nil {
		return domain.Attendee{}, err
	}
	return domain.NewAttendee(domain.AttendeeInput{
		Address: addr, CommonName: in.CommonName, Role: role, Status: status, RSVP: in.RSVP,
	})
}

// DeleteEvent removes an event by id.
func (s *CalendarService) DeleteEvent(ctx context.Context, id string) error {
	if err := s.store.DeleteEvent(ctx, id); err != nil {
		return fmt.Errorf("calendar: delete event %q: %w", id, err)
	}
	return nil
}

// ImportEvents decodes events from the given bytes with the codec and saves each, along with any
// preserved passthrough components (to-dos and journal entries). A decoded event or passthrough keeps
// its own id (an ICS UID where present), so re-importing updates the matching records rather than
// duplicating them. It returns the number of events imported.
func (s *CalendarService) ImportEvents(ctx context.Context, codec CalendarCodec, data []byte) (int, error) {
	events, passthrough, err := codec.Decode(data)
	if err != nil {
		return 0, fmt.Errorf("calendar: decode import: %w", err)
	}
	for i, e := range events {
		if err := s.store.SaveEvent(ctx, e); err != nil {
			return i, fmt.Errorf("calendar: import save %q: %w", e.ID(), err)
		}
	}
	for _, p := range passthrough {
		if err := s.store.SavePassthrough(ctx, p); err != nil {
			return len(events), fmt.Errorf("calendar: import save passthrough %q: %w", p.UID(), err)
		}
	}
	return len(events), nil
}

// ExportEvents encodes every event and preserved passthrough component with the codec into its
// serialised form, so an imported calendar's to-dos and journal entries survive the round-trip.
func (s *CalendarService) ExportEvents(ctx context.Context, codec CalendarCodec) ([]byte, error) {
	events, err := s.store.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar: list for export: %w", err)
	}
	passthrough, err := s.store.ListPassthrough(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar: list passthrough for export: %w", err)
	}
	data, err := codec.Encode(events, passthrough)
	if err != nil {
		return nil, fmt.Errorf("calendar: encode export: %w", err)
	}
	return data, nil
}
