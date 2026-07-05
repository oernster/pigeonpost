package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListCalendars returns every calendar, ordered by name.
func (s *Store) ListCalendars(ctx context.Context) ([]domain.Calendar, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, colour FROM calendar ORDER BY name;")
	if err != nil {
		return nil, fmt.Errorf("query calendars: %w", err)
	}
	defer rows.Close()
	var calendars []domain.Calendar
	for rows.Next() {
		var id, name, colour string
		if err := rows.Scan(&id, &name, &colour); err != nil {
			return nil, fmt.Errorf("scan calendar: %w", err)
		}
		calendar, err := domain.NewCalendar(id, name, colour)
		if err != nil {
			return nil, fmt.Errorf("rebuild calendar %q: %w", id, err)
		}
		calendars = append(calendars, calendar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate calendars: %w", err)
	}
	return calendars, nil
}

// SaveCalendar inserts or updates a calendar by id.
func (s *Store) SaveCalendar(ctx context.Context, c domain.Calendar) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO calendar (id, name, colour) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, colour = excluded.colour;`,
		c.ID(), c.Name(), c.Colour())
	if err != nil {
		return fmt.Errorf("save calendar %q: %w", c.ID(), err)
	}
	return nil
}

// DeleteCalendar removes a calendar and all of its events in one transaction.
func (s *Store) DeleteCalendar(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM event WHERE calendar_id = ?;", id); err != nil {
			return fmt.Errorf("delete calendar events: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM calendar WHERE id = ?;", id); err != nil {
			return fmt.Errorf("delete calendar %q: %w", id, err)
		}
		return nil
	})
}

const eventColumns = "id, uid, calendar_id, summary, description, location, start_ms, end_ms, all_day, recurrence, extra"

// ListEvents returns every event, ordered by start time.
func (s *Store) ListEvents(ctx context.Context) ([]domain.Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+eventColumns+" FROM event ORDER BY start_ms;")
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	var events []domain.Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

// GetEvent returns a single event by id.
func (s *Store) GetEvent(ctx context.Context, id string) (domain.Event, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+eventColumns+" FROM event WHERE id = ?;", id)
	event, err := scanEvent(row)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get event %q: %w", id, err)
	}
	return event, nil
}

// SaveEvent inserts or updates an event by id.
func (s *Store) SaveEvent(ctx context.Context, e domain.Event) error {
	var endMs int64
	if e.HasEnd() {
		endMs = e.End().UnixMilli()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO event (`+eventColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET uid = excluded.uid, calendar_id = excluded.calendar_id,
		     summary = excluded.summary, description = excluded.description, location = excluded.location,
		     start_ms = excluded.start_ms, end_ms = excluded.end_ms, all_day = excluded.all_day,
		     recurrence = excluded.recurrence, extra = excluded.extra;`,
		e.ID(), e.UID(), e.CalendarID(), e.Summary(), e.Description(), e.Location(),
		e.Start().UnixMilli(), endMs, boolToInt(e.AllDay()), e.Recurrence(), e.Extra())
	if err != nil {
		return fmt.Errorf("save event %q: %w", e.ID(), err)
	}
	return nil
}

// DeleteEvent removes an event by id.
func (s *Store) DeleteEvent(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM event WHERE id = ?;", id); err != nil {
		return fmt.Errorf("delete event %q: %w", id, err)
	}
	return nil
}

// scanEvent reads one event row into a validated domain event. A zero end_ms means the event has no
// end.
func scanEvent(row interface{ Scan(...any) error }) (domain.Event, error) {
	var (
		id, uid, calendarID, summary, description, location, recurrence, extra string
		startMs, endMs                                                         int64
		allDay                                                                 int
	)
	if err := row.Scan(&id, &uid, &calendarID, &summary, &description, &location,
		&startMs, &endMs, &allDay, &recurrence, &extra); err != nil {
		return domain.Event{}, fmt.Errorf("scan event: %w", err)
	}
	var end time.Time
	if endMs != 0 {
		end = time.UnixMilli(endMs).UTC()
	}
	event, err := domain.NewEvent(domain.EventInput{
		ID:          id,
		UID:         uid,
		CalendarID:  calendarID,
		Summary:     summary,
		Description: description,
		Location:    location,
		Start:       time.UnixMilli(startMs).UTC(),
		End:         end,
		AllDay:      allDay != 0,
		Recurrence:  recurrence,
		Extra:       extra,
	})
	if err != nil {
		return domain.Event{}, fmt.Errorf("rebuild event %q: %w", id, err)
	}
	return event, nil
}
