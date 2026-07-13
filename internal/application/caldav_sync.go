package application

import (
	"context"
	"errors"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ErrCalDAVConflict signals a 412 Precondition Failed from a conditional write: the server's copy changed
// under the write's If-Match, or a create's If-None-Match:* found the object already present. The engine
// refetches the object and reconciles it last-writer-wins rather than treating the write as a hard failure.
var ErrCalDAVConflict = errors.New("caldav: conflict")

// CalDAVWriter is the write half of the two-way DAV sync: conditional PUT and DELETE of a collection's
// objects. go-webdav v0.7.0 cannot send If-Match/If-None-Match or delete a CalDAV object, so the adapter
// issues these as raw conditional HTTP requests. A 412 is returned as ErrCalDAVConflict.
type CalDAVWriter interface {
	// PutObject writes an object's iCalendar body at href. ifMatch guards an update against the object's last
	// etag (empty to skip the header); ifNoneMatch is the literal If-None-Match header value ("*" for a create
	// that must not overwrite an existing object, empty to skip). It returns the server's new etag, which may
	// be empty when the server omits the header on the response.
	PutObject(ctx context.Context, href string, ics []byte, ifMatch, ifNoneMatch string) (etag string, err error)
	// DeleteObject removes the object at href, guarded by ifMatch against its last etag. A server 404 (the
	// object is already gone) is treated as success, since the delete intent is satisfied.
	DeleteObject(ctx context.Context, href, ifMatch string) error
}

// CalDAVWriterFactory builds a CalDAVWriter for an account and password, the write counterpart to
// CalDAVSourceFactory.
type CalDAVWriterFactory interface {
	NewWriter(account domain.CalendarAccount, password string) (CalDAVWriter, error)
}

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

// SyncedEventIdentity is the CalDAV object a single local event belongs to: its resource path, last-seen etag
// and owning calendar. A local-only event (one not pulled from a server) has an empty Href. It is read when a
// local edit or delete needs to record the matching pending write intent.
type SyncedEventIdentity struct {
	Href       string
	ETag       string
	CalendarID string
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
	// RemoteCalendarByID returns a local calendar's remote-collection record and reports whether it is a remote
	// calendar (one with an owning account). A local-only calendar, or an unknown id, reports false.
	RemoteCalendarByID(ctx context.Context, calendarID string) (RemoteCalendarRecord, bool, error)
	// SyncedEventIdentity returns the CalDAV object a local event belongs to (its href, etag and calendar) and
	// reports whether the event exists. It is read to build the pending write intent for a local edit or delete.
	SyncedEventIdentity(ctx context.Context, eventID string) (SyncedEventIdentity, bool, error)
	// SaveEventWithPending upserts a synced event (tagged with href and etag) and records the pending write
	// intent in one transaction, so a local create or edit of a remote-calendar event can never be saved
	// without the intent that will push it to the server.
	SaveEventWithPending(ctx context.Context, event domain.Event, href, etag string, op PendingCalendarObject) error
	// DeleteObjectWithPending removes every local event of one object and records the pending delete intent in
	// one transaction, so the tombstone that will remove the object on the server outlives the local rows.
	DeleteObjectWithPending(ctx context.Context, href string, op PendingCalendarObject) error
	// CalendarCTag returns a local calendar's last-seen CTag, or the empty string when it has none.
	CalendarCTag(ctx context.Context, calendarID string) (string, error)
	// UpdateCalendarCTag records the CTag a collection was last reconciled against, so the next sync can skip
	// the collection when the server still reports the same one.
	UpdateCalendarCTag(ctx context.Context, calendarID, ctag string) error
	// SetPendingCalendarOp records or replaces the pending write intent for one object.
	SetPendingCalendarOp(ctx context.Context, op PendingCalendarObject) error
	// ClearPendingCalendarOp removes the pending intent for one object, once the server agrees.
	ClearPendingCalendarOp(ctx context.Context, calendarID, href string) error
	// ListPendingCalendarOps returns every pending write intent across all collections, for the flush.
	ListPendingCalendarOps(ctx context.Context) ([]PendingCalendarObject, error)
}
