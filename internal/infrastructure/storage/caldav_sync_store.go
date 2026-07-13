package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// syncedEventUpsertSQL is eventUpsertSQL widened with the CalDAV href and etag columns, so a synced event
// carries the remote object it came from.
const syncedEventUpsertSQL = `INSERT INTO event (` + eventColumns + `, href, etag) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	 ON CONFLICT(id) DO UPDATE SET uid = excluded.uid, calendar_id = excluded.calendar_id,
	     summary = excluded.summary, description = excluded.description, location = excluded.location,
	     start_ms = excluded.start_ms, end_ms = excluded.end_ms, all_day = excluded.all_day,
	     recurrence = excluded.recurrence, extra = excluded.extra, rdate = excluded.rdate,
	     exdate = excluded.exdate, recurrence_id = excluded.recurrence_id, time_zone = excluded.time_zone,
	     alarms = excluded.alarms, organizer = excluded.organizer, attendees = excluded.attendees,
	     category = excluded.category, href = excluded.href, etag = excluded.etag;`

// SaveSyncedEvent upserts an event pulled from a CalDAV object, tagging it with the object's href and etag so
// a later write-back can target the object and guard the write with If-Match. Every event decoded from one
// object shares that object's href and etag.
func (s *Store) SaveSyncedEvent(ctx context.Context, e domain.Event, href, etag string) error {
	args, err := eventInsertArgs(e)
	if err != nil {
		return fmt.Errorf("save synced event %q: %w", e.ID(), err)
	}
	args = append(args, href, etag)
	if _, err := s.db.ExecContext(ctx, syncedEventUpsertSQL, args...); err != nil {
		return fmt.Errorf("save synced event %q: %w", e.ID(), err)
	}
	return nil
}

// ListSyncedObjects returns the distinct (href, etag) of every synced object in a local calendar. Local-only
// events (empty href) are excluded, so the result is exactly the objects the reconcile compares to the server.
func (s *Store) ListSyncedObjects(ctx context.Context, calendarID string) ([]application.SyncedObject, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT href, etag FROM event WHERE calendar_id = ? AND href <> '' ORDER BY href;", calendarID)
	if err != nil {
		return nil, fmt.Errorf("list synced objects for %q: %w", calendarID, err)
	}
	defer rows.Close()
	out := make([]application.SyncedObject, 0)
	for rows.Next() {
		var href, etag string
		if err := rows.Scan(&href, &etag); err != nil {
			return nil, fmt.Errorf("scan synced object: %w", err)
		}
		out = append(out, application.SyncedObject{Href: href, ETag: etag})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate synced objects: %w", err)
	}
	return out, nil
}

// EventsByHref returns every local event decoded from one remote object, ordered so the recurrence master
// (recurrence_id 0) comes before its overrides, ready for re-encoding into one object body.
func (s *Store) EventsByHref(ctx context.Context, href string) ([]domain.Event, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+eventColumns+" FROM event WHERE href = ? ORDER BY recurrence_id;", href)
	if err != nil {
		return nil, fmt.Errorf("events by href %q: %w", href, err)
	}
	defer rows.Close()
	out := make([]domain.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events by href %q: %w", href, err)
	}
	return out, nil
}

// DeleteEventsByHref removes every local event of one remote object, used when a sync finds the object gone
// from the server.
func (s *Store) DeleteEventsByHref(ctx context.Context, href string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM event WHERE href = ?;", href); err != nil {
		return fmt.Errorf("delete events by href %q: %w", href, err)
	}
	return nil
}

// SaveRemoteCalendar upserts a local calendar that mirrors a remote collection, recording the owning account,
// the collection resource path and its CTag alongside the calendar's own fields.
func (s *Store) SaveRemoteCalendar(ctx context.Context, c domain.Calendar, accountID, href, ctag string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO calendar (id, name, colour, account_id, href, ctag) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, colour = excluded.colour,
		     account_id = excluded.account_id, href = excluded.href, ctag = excluded.ctag;`,
		c.ID(), c.Name(), c.Colour(), accountID, href, ctag)
	if err != nil {
		return fmt.Errorf("save remote calendar %q: %w", c.ID(), err)
	}
	return nil
}

// ListRemoteCalendars returns the collections mirrored for one account, ordered by name.
func (s *Store) ListRemoteCalendars(ctx context.Context, accountID string) ([]application.RemoteCalendarRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, account_id, href, ctag, name FROM calendar WHERE account_id = ? ORDER BY name;", accountID)
	if err != nil {
		return nil, fmt.Errorf("list remote calendars for %q: %w", accountID, err)
	}
	defer rows.Close()
	out := make([]application.RemoteCalendarRecord, 0)
	for rows.Next() {
		var id, acc, href, ctag, name string
		if err := rows.Scan(&id, &acc, &href, &ctag, &name); err != nil {
			return nil, fmt.Errorf("scan remote calendar: %w", err)
		}
		out = append(out, application.RemoteCalendarRecord{CalendarID: id, AccountID: acc, Href: href, CTag: ctag, Name: name})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate remote calendars: %w", err)
	}
	return out, nil
}

// RemoteCalendarByID returns a local calendar's remote-collection record and reports whether it is a remote
// calendar. A calendar is remote when it carries an owning account; a local-only calendar (empty account_id)
// and an unknown id both report false.
func (s *Store) RemoteCalendarByID(ctx context.Context, calendarID string) (application.RemoteCalendarRecord, bool, error) {
	var id, accountID, href, ctag, name string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, account_id, href, ctag, name FROM calendar WHERE id = ?;", calendarID).
		Scan(&id, &accountID, &href, &ctag, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return application.RemoteCalendarRecord{}, false, nil
	}
	if err != nil {
		return application.RemoteCalendarRecord{}, false, fmt.Errorf("remote calendar %q: %w", calendarID, err)
	}
	record := application.RemoteCalendarRecord{CalendarID: id, AccountID: accountID, Href: href, CTag: ctag, Name: name}
	return record, accountID != "", nil
}

// SyncedEventIdentity returns the CalDAV object a local event belongs to (its href, etag and calendar) and
// reports whether the event exists. A local-only event exists with an empty href.
func (s *Store) SyncedEventIdentity(ctx context.Context, eventID string) (application.SyncedEventIdentity, bool, error) {
	var href, etag, calendarID string
	err := s.db.QueryRowContext(ctx,
		"SELECT href, etag, calendar_id FROM event WHERE id = ?;", eventID).Scan(&href, &etag, &calendarID)
	if errors.Is(err, sql.ErrNoRows) {
		return application.SyncedEventIdentity{}, false, nil
	}
	if err != nil {
		return application.SyncedEventIdentity{}, false, fmt.Errorf("synced event identity %q: %w", eventID, err)
	}
	return application.SyncedEventIdentity{Href: href, ETag: etag, CalendarID: calendarID}, true, nil
}

// SaveEventWithPending upserts a synced event (tagged with href and etag) and records its pending write intent
// in one transaction, so a local create or edit of a remote-calendar event can never be saved without the
// intent that will push it to the server.
func (s *Store) SaveEventWithPending(ctx context.Context, e domain.Event, href, etag string, op application.PendingCalendarObject) error {
	args, err := eventInsertArgs(e)
	if err != nil {
		return fmt.Errorf("save event with pending %q: %w", e.ID(), err)
	}
	args = append(args, href, etag)
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, syncedEventUpsertSQL, args...); err != nil {
			return fmt.Errorf("save synced event %q: %w", e.ID(), err)
		}
		if err := setPendingCalendarOpTx(ctx, tx, op); err != nil {
			return err
		}
		return nil
	})
}

// DeleteObjectWithPending removes every local event of one object and records the pending delete intent in one
// transaction, so the tombstone that removes the object on the server outlives the local rows.
func (s *Store) DeleteObjectWithPending(ctx context.Context, href string, op application.PendingCalendarObject) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM event WHERE href = ?;", href); err != nil {
			return fmt.Errorf("delete events by href %q: %w", href, err)
		}
		if err := setPendingCalendarOpTx(ctx, tx, op); err != nil {
			return err
		}
		return nil
	})
}

// pendingCalendarUpsertSQL records or replaces one pending write intent, keyed by (calendar_id, href).
const pendingCalendarUpsertSQL = "INSERT OR REPLACE INTO calendar_pending (calendar_id, href, op, base_etag) VALUES (?, ?, ?, ?);"

// setPendingCalendarOpTx records or replaces a pending write intent inside a transaction, shared by the atomic
// save and delete so the pending-op write lives in one place.
func setPendingCalendarOpTx(ctx context.Context, tx *sql.Tx, op application.PendingCalendarObject) error {
	if _, err := tx.ExecContext(ctx, pendingCalendarUpsertSQL, op.CalendarID, op.Href, int(op.Op), op.BaseETag); err != nil {
		return fmt.Errorf("set pending calendar op %q %q: %w", op.CalendarID, op.Href, err)
	}
	return nil
}

// CalendarCTag returns a local calendar's last-seen CTag, or the empty string when the calendar is unknown or
// carries none.
func (s *Store) CalendarCTag(ctx context.Context, calendarID string) (string, error) {
	var ctag string
	err := s.db.QueryRowContext(ctx, "SELECT ctag FROM calendar WHERE id = ?;", calendarID).Scan(&ctag)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("calendar ctag %q: %w", calendarID, err)
	}
	return ctag, nil
}

// SetPendingCalendarOp records or replaces the pending write intent for one object.
func (s *Store) SetPendingCalendarOp(ctx context.Context, op application.PendingCalendarObject) error {
	if _, err := s.db.ExecContext(ctx, pendingCalendarUpsertSQL, op.CalendarID, op.Href, int(op.Op), op.BaseETag); err != nil {
		return fmt.Errorf("set pending calendar op %q %q: %w", op.CalendarID, op.Href, err)
	}
	return nil
}

// ClearPendingCalendarOp removes the pending intent for one object, called once the server agrees.
func (s *Store) ClearPendingCalendarOp(ctx context.Context, calendarID, href string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM calendar_pending WHERE calendar_id = ? AND href = ?;", calendarID, href)
	if err != nil {
		return fmt.Errorf("clear pending calendar op %q %q: %w", calendarID, href, err)
	}
	return nil
}

// ListPendingCalendarOps returns every pending write intent across all collections, for the flush.
func (s *Store) ListPendingCalendarOps(ctx context.Context) ([]application.PendingCalendarObject, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT calendar_id, href, op, base_etag FROM calendar_pending;")
	if err != nil {
		return nil, fmt.Errorf("list pending calendar ops: %w", err)
	}
	defer rows.Close()
	out := make([]application.PendingCalendarObject, 0)
	for rows.Next() {
		var calendarID, href, baseETag string
		var op int
		if err := rows.Scan(&calendarID, &href, &op, &baseETag); err != nil {
			return nil, fmt.Errorf("scan pending calendar op: %w", err)
		}
		out = append(out, application.PendingCalendarObject{
			CalendarID: calendarID, Href: href, Op: application.CalendarOpKind(op), BaseETag: baseETag,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending calendar ops: %w", err)
	}
	return out, nil
}
