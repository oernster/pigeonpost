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
	return queryRows(ctx, s.db, "synced objects for "+calendarID,
		"SELECT DISTINCT href, etag FROM event WHERE calendar_id = ? AND href <> '' ORDER BY href;",
		func(row scanner) (application.SyncedObject, error) {
			var href, etag string
			if err := row.Scan(&href, &etag); err != nil {
				return application.SyncedObject{}, fmt.Errorf("scan synced object: %w", err)
			}
			return application.SyncedObject{Href: href, ETag: etag}, nil
		}, calendarID)
}

// EventsByHref returns every local event decoded from one remote object, ordered so the recurrence master
// (recurrence_id 0) comes before its overrides, ready for re-encoding into one object body.
func (s *Store) EventsByHref(ctx context.Context, href string) ([]domain.Event, error) {
	return queryRows(ctx, s.db, "events by href "+href,
		"SELECT "+eventColumns+" FROM event WHERE href = ? ORDER BY recurrence_id;", scanEvent, href)
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
	return queryRows(ctx, s.db, "remote calendars for "+accountID,
		"SELECT id, account_id, href, ctag, name FROM calendar WHERE account_id = ? ORDER BY name;",
		func(row scanner) (application.RemoteCalendarRecord, error) {
			var id, acc, href, ctag, name string
			if err := row.Scan(&id, &acc, &href, &ctag, &name); err != nil {
				return application.RemoteCalendarRecord{}, fmt.Errorf("scan remote calendar: %w", err)
			}
			return application.RemoteCalendarRecord{CalendarID: id, AccountID: acc, Href: href, CTag: ctag, Name: name}, nil
		}, accountID)
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

// UpdateCalendarCTag records the CTag a collection was last reconciled against, leaving the calendar's other
// fields untouched. A calendar that does not exist is a no-op.
func (s *Store) UpdateCalendarCTag(ctx context.Context, calendarID, ctag string) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE calendar SET ctag = ? WHERE id = ?;", ctag, calendarID); err != nil {
		return fmt.Errorf("update calendar ctag %q: %w", calendarID, err)
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
	return queryRows(ctx, s.db, "pending calendar ops",
		"SELECT calendar_id, href, op, base_etag FROM calendar_pending;",
		func(row scanner) (application.PendingCalendarObject, error) {
			var calendarID, href, baseETag string
			var op int
			if err := row.Scan(&calendarID, &href, &op, &baseETag); err != nil {
				return application.PendingCalendarObject{}, fmt.Errorf("scan pending calendar op: %w", err)
			}
			return application.PendingCalendarObject{
				CalendarID: calendarID, Href: href, Op: application.CalendarOpKind(op), BaseETag: baseETag,
			}, nil
		})
}
