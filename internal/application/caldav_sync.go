package application

import (
	"context"

	"github.com/oernster/pigeonpost/internal/domain"
)

// CalendarOpKind is the kind of pending write-back held for a remote CalDAV object.
type CalendarOpKind int

const (
	// CalendarOpCreate is a locally created object not yet on the server; it is pushed with If-None-Match:*.
	CalendarOpCreate CalendarOpKind = iota
	// CalendarOpUpdate is a locally edited object; it is pushed with If-Match against its base etag.
	CalendarOpUpdate
	// CalendarOpDelete is a locally deleted object; it is removed with DELETE If-Match against its base etag.
	// A delete intent outlives the local event rows, so it also serves as the object's tombstone.
	CalendarOpDelete
)

// SyncedObject is the identity of a remote CalDAV object as held locally: its resource path and the last etag
// seen. The reconcile compares these against what the server reports to find changed and removed objects.
type SyncedObject struct {
	Href string
	ETag string
}

// RemoteCalendarRecord is a locally stored CalDAV collection: the local calendar id plus the owning account,
// the collection's resource path and its last-seen CTag (used to skip an unchanged collection on a sync).
type RemoteCalendarRecord struct {
	CalendarID string
	AccountID  string
	Href       string
	CTag       string
	Name       string
}

// PendingCalendarObject is one remote object with an unpushed local change: which collection and object it
// targets, whether the intent is create, update or delete, and the base etag the conditional write guards on
// (empty for a create). It is the calendar analogue of a pending tag op, keyed by (CalendarID, Href).
type PendingCalendarObject struct {
	CalendarID string
	Href       string
	Op         CalendarOpKind
	BaseETag   string
}

// CalendarSyncStore is the storage surface for two-way CalDAV sync, kept separate from CalendarStore so the
// read-only calendar features do not depend on it. It persists a synced event with its object href and etag,
// reads back the local object identities and events for a reconcile, tracks each collection's account, href
// and CTag, and records the pending write intents.
type CalendarSyncStore interface {
	// SaveSyncedEvent upserts an event pulled from a CalDAV object, tagging it with the object's href and etag.
	SaveSyncedEvent(ctx context.Context, event domain.Event, href, etag string) error
	// ListSyncedObjects returns the distinct (href, etag) of every synced object in a local calendar.
	ListSyncedObjects(ctx context.Context, calendarID string) ([]SyncedObject, error)
	// EventsByHref returns every local event decoded from one remote object (a master plus its overrides).
	EventsByHref(ctx context.Context, href string) ([]domain.Event, error)
	// DeleteEventsByHref removes every local event of one remote object, used when the server drops the object.
	DeleteEventsByHref(ctx context.Context, href string) error
	// SaveRemoteCalendar upserts a local calendar mirroring a remote collection, recording its account,
	// resource path and CTag.
	SaveRemoteCalendar(ctx context.Context, calendar domain.Calendar, accountID, href, ctag string) error
	// ListRemoteCalendars returns the collections mirrored for one account.
	ListRemoteCalendars(ctx context.Context, accountID string) ([]RemoteCalendarRecord, error)
	// CalendarCTag returns a local calendar's last-seen CTag, or the empty string when it has none.
	CalendarCTag(ctx context.Context, calendarID string) (string, error)
	// SetPendingCalendarOp records or replaces the pending write intent for one object.
	SetPendingCalendarOp(ctx context.Context, op PendingCalendarObject) error
	// ClearPendingCalendarOp removes the pending intent for one object, once the server agrees.
	ClearPendingCalendarOp(ctx context.Context, calendarID, href string) error
	// ListPendingCalendarOps returns every pending write intent across all collections, for the flush.
	ListPendingCalendarOps(ctx context.Context) ([]PendingCalendarObject, error)
}
