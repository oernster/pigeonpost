package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListCalendars returns every calendar, ordered by name.
func (s *Store) ListCalendars(ctx context.Context) ([]domain.Calendar, error) {
	return queryRows(ctx, s.db, "calendars", "SELECT id, name, colour FROM calendar ORDER BY name;",
		func(row scanner) (domain.Calendar, error) {
			var id, name, colour string
			if err := row.Scan(&id, &name, &colour); err != nil {
				return domain.Calendar{}, fmt.Errorf("scan calendar: %w", err)
			}
			calendar, err := domain.NewCalendar(id, name, colour)
			if err != nil {
				return domain.Calendar{}, fmt.Errorf("rebuild calendar %q: %w", id, err)
			}
			return calendar, nil
		})
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

// eventKeyColumn is the event table's primary key, the column every upsert resolves its conflict on.
const eventKeyColumn = "id"

// eventColumnList is the one home of the event row's shape, in the order eventInsertArgs supplies values
// and scanEvent reads them. The key column comes first. Every SELECT list, upsert and placeholder list is
// derived from it, so a new column is added here and nowhere else in the SQL.
var eventColumnList = []string{
	eventKeyColumn, "uid", "calendar_id", "summary", "description", "location", "start_ms", "end_ms", "all_day",
	"recurrence", "extra", "rdate", "exdate", "recurrence_id", "time_zone", "alarms", "organizer", "attendees",
	"category", "sequence",
}

// eventColumns is eventColumnList as a SELECT list.
var eventColumns = strings.Join(eventColumnList, ", ")

// eventUpsert builds the insert-or-update of one event by id across eventColumnList plus any extra columns
// (the CalDAV href and etag of a synced event), updating every non-key column on a conflict.
func eventUpsert(extra ...string) string {
	columns := append(append([]string(nil), eventColumnList...), extra...)
	updates := make([]string, 0, len(columns))
	for _, c := range columns {
		if c != eventKeyColumn {
			updates = append(updates, c+" = excluded."+c)
		}
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ")
	return "INSERT INTO event (" + strings.Join(columns, ", ") + ") VALUES (" + placeholders + ")" +
		" ON CONFLICT(" + eventKeyColumn + ") DO UPDATE SET " + strings.Join(updates, ", ") + ";"
}

// eventUpsertSQL inserts or updates one event by id across the event columns. SaveSyncedEvent uses a wider
// variant that also carries the CalDAV href and etag (see caldav_sync_store.go).
var eventUpsertSQL = eventUpsert()

// eventInsertArgs builds the ordered argument list for the eventColumns, shared by SaveEvent and (with the
// href and etag appended) SaveSyncedEvent so the encoding of times, alarms, organiser and attendees lives in
// one place.
func eventInsertArgs(e domain.Event) ([]any, error) {
	var endMs int64
	if e.HasEnd() {
		endMs = e.End().UnixMilli()
	}
	var recurrenceIDMs int64
	if e.IsOverride() {
		recurrenceIDMs = e.RecurrenceID().UnixMilli()
	}
	organizer, err := encodeOrganizer(e.Organizer())
	if err != nil {
		return nil, err
	}
	attendees, err := encodeAttendees(e.Attendees())
	if err != nil {
		return nil, err
	}
	return []any{
		e.ID(), e.UID(), e.CalendarID(), e.Summary(), e.Description(), e.Location(),
		e.Start().UnixMilli(), endMs, boolToInt(e.AllDay()), e.Recurrence(), e.Extra(),
		encodeTimes(e.RDates()), encodeTimes(e.ExDates()), recurrenceIDMs, e.TimeZone(), encodeAlarms(e.Alarms()),
		organizer, attendees, e.Category(), e.Sequence(),
	}, nil
}

// ListEvents returns every event, ordered by start time.
func (s *Store) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return queryRows(ctx, s.db, "events", "SELECT "+eventColumns+" FROM event ORDER BY start_ms;", scanEvent)
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
	args, err := eventInsertArgs(e)
	if err != nil {
		return fmt.Errorf("save event %q: %w", e.ID(), err)
	}
	if _, err := s.db.ExecContext(ctx, eventUpsertSQL, args...); err != nil {
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
// end; a zero recurrence_id means the event is not an override.
func scanEvent(row scanner) (domain.Event, error) {
	var (
		id, uid, calendarID, summary, description, location, category, recurrence, extra, rdate, exdate, timeZone, alarms string
		organizer, attendees                                                                                              string
		startMs, endMs, recurrenceIDMs                                                                                    int64
		allDay, sequence                                                                                                  int
	)
	if err := row.Scan(&id, &uid, &calendarID, &summary, &description, &location,
		&startMs, &endMs, &allDay, &recurrence, &extra, &rdate, &exdate, &recurrenceIDMs, &timeZone, &alarms,
		&organizer, &attendees, &category, &sequence); err != nil {
		return domain.Event{}, fmt.Errorf("scan event: %w", err)
	}
	alarmList, err := decodeAlarms(alarms)
	if err != nil {
		return domain.Event{}, fmt.Errorf("rebuild event %q: %w", id, err)
	}
	organizerValue, err := decodeOrganizer(organizer)
	if err != nil {
		return domain.Event{}, fmt.Errorf("rebuild event %q: %w", id, err)
	}
	attendeeList, err := decodeAttendees(attendees)
	if err != nil {
		return domain.Event{}, fmt.Errorf("rebuild event %q: %w", id, err)
	}
	var end time.Time
	if endMs != 0 {
		end = time.UnixMilli(endMs).UTC()
	}
	rdates, err := decodeTimes(rdate)
	if err != nil {
		return domain.Event{}, fmt.Errorf("rebuild event %q: %w", id, err)
	}
	exdates, err := decodeTimes(exdate)
	if err != nil {
		return domain.Event{}, fmt.Errorf("rebuild event %q: %w", id, err)
	}
	var recurrenceID time.Time
	if recurrenceIDMs != 0 {
		recurrenceID = time.UnixMilli(recurrenceIDMs).UTC()
	}
	event, err := domain.NewEvent(domain.EventInput{
		ID:           id,
		UID:          uid,
		CalendarID:   calendarID,
		Summary:      summary,
		Description:  description,
		Location:     location,
		Category:     category,
		Start:        time.UnixMilli(startMs).UTC(),
		End:          end,
		AllDay:       allDay != 0,
		Recurrence:   recurrence,
		RDates:       rdates,
		ExDates:      exdates,
		RecurrenceID: recurrenceID,
		Sequence:     sequence,
		TimeZone:     timeZone,
		Alarms:       alarmList,
		Extra:        extra,
		Organizer:    organizerValue,
		Attendees:    attendeeList,
	})
	if err != nil {
		return domain.Event{}, fmt.Errorf("rebuild event %q: %w", id, err)
	}
	return event, nil
}

// encodeTimes serialises a list of times as comma-separated Unix millisecond values for storage. An
// empty list encodes to the empty string.
func encodeTimes(times []time.Time) string {
	if len(times) == 0 {
		return ""
	}
	parts := make([]string, len(times))
	for i, t := range times {
		parts[i] = strconv.FormatInt(t.UnixMilli(), 10)
	}
	return strings.Join(parts, ",")
}

// encodeAlarms serialises reminders as comma-separated trigger offsets in whole seconds. An empty list
// encodes to the empty string.
func encodeAlarms(alarms []domain.Alarm) string {
	if len(alarms) == 0 {
		return ""
	}
	parts := make([]string, len(alarms))
	for i, a := range alarms {
		parts[i] = strconv.FormatInt(int64(a.Offset()/time.Second), 10)
	}
	return strings.Join(parts, ",")
}

// decodeAlarms parses the comma-separated second offsets written by encodeAlarms back into alarms. The
// empty string decodes to no alarms.
func decodeAlarms(s string) ([]domain.Alarm, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]domain.Alarm, 0, len(parts))
	for _, p := range parts {
		secs, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("decode alarm %q: %w", p, err)
		}
		out = append(out, domain.NewAlarm(time.Duration(secs)*time.Second))
	}
	return out, nil
}

// SavePassthrough inserts or replaces (by UID) a preserved VTODO or VJOURNAL component.
func (s *Store) SavePassthrough(ctx context.Context, p domain.CalendarPassthrough) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO calendar_passthrough (uid, kind, raw) VALUES (?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET kind = excluded.kind, raw = excluded.raw;`,
		p.UID(), p.Kind(), p.Raw())
	if err != nil {
		return fmt.Errorf("save passthrough %q: %w", p.UID(), err)
	}
	return nil
}

// ListPassthrough returns every preserved passthrough component, ordered by UID for a stable export.
func (s *Store) ListPassthrough(ctx context.Context) ([]domain.CalendarPassthrough, error) {
	return queryRows(ctx, s.db, "passthrough", "SELECT uid, kind, raw FROM calendar_passthrough ORDER BY uid;",
		func(row scanner) (domain.CalendarPassthrough, error) {
			var uid, kind, raw string
			if err := row.Scan(&uid, &kind, &raw); err != nil {
				return domain.CalendarPassthrough{}, fmt.Errorf("scan passthrough: %w", err)
			}
			p, err := domain.NewCalendarPassthrough(uid, kind, raw)
			if err != nil {
				return domain.CalendarPassthrough{}, fmt.Errorf("rebuild passthrough %q: %w", uid, err)
			}
			return p, nil
		})
}

// decodeTimes parses the comma-separated Unix millisecond values written by encodeTimes back into UTC
// times. The empty string decodes to no times.
func decodeTimes(s string) ([]time.Time, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]time.Time, 0, len(parts))
	for _, p := range parts {
		ms, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("decode recurrence date %q: %w", p, err)
		}
		out = append(out, time.UnixMilli(ms).UTC())
	}
	return out, nil
}
