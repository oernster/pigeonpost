package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// RemoteCalendar is a calendar collection discovered on a CalDAV server.
type RemoteCalendar struct {
	// Path is the collection's resource path (href) on the server.
	Path string
	// DisplayName is the human-readable calendar name.
	DisplayName string
}

// RemoteObject is one calendar object (a VEVENT resource) fetched from a CalDAV collection: its resource
// path, entity tag and raw iCalendar body.
type RemoteObject struct {
	Href string
	ETag string
	Data []byte
}

// DefaultRemoteCalendarColour is the display colour given to a calendar mirrored from a CalDAV collection,
// since a server collection does not report one. It is a neutral blue the UI can override later.
const DefaultRemoteCalendarColour = "#4f8cc9"

// remoteCalendarID derives a local calendar's stable id from its owning account and the collection's resource
// path, so re-pulling a collection updates the same local calendar instead of duplicating it.
func remoteCalendarID(accountID, collectionHref string) string {
	return accountID + "|" + collectionHref
}

// CalDAVSyncService performs the discovery-and-population pull of an account's remote CalDAV calendars into the
// local store. It lists the account's collections, mirrors each as a local calendar (recording the account,
// resource path and, later, CTag), then fetches every object, decodes it and saves its events tagged with the
// object's href and etag so a later write-back can target and guard it. It is the population half of the
// two-way sync; the guarded three-way merge of ongoing changes belongs to CalDAVReconcileService.
type CalDAVSyncService struct {
	source    CalDAVSource
	codec     CalendarCodec
	store     CalendarSyncStore
	accountID string
}

// NewCalDAVSyncService wires the pull over its source, codec and sync store for one account: the account id
// stamps the mirrored calendars so their events are associated with the right local collection.
func NewCalDAVSyncService(source CalDAVSource, codec CalendarCodec, store CalendarSyncStore, accountID string) *CalDAVSyncService {
	return &CalDAVSyncService{source: source, codec: codec, store: store, accountID: accountID}
}

// Discover lists the account's remote collections and mirrors each as a local calendar, returning the records
// the reconcile needs. It is the collection-discovery half of the pull, kept separate so the two-way sync can
// establish the local calendars and then hand the object merge to CalDAVReconcileService, which guards pending
// local edits rather than overwriting them. A store write failure is fatal, since it signals a broken local
// database rather than a single unusable remote item.
func (s *CalDAVSyncService) Discover(ctx context.Context) ([]RemoteCalendarRecord, error) {
	calendars, err := s.source.ListCalendars(ctx)
	if err != nil {
		return nil, fmt.Errorf("caldav: list calendars: %w", err)
	}
	records := make([]RemoteCalendarRecord, 0, len(calendars))
	for _, calendar := range calendars {
		record, err := s.saveRemoteCalendar(ctx, calendar)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// Pull discovers the account's collections and, for each, saves its objects' events, returning the number of
// events saved. It is the one-way population path (server into local, no merge): a calendar whose objects
// cannot be listed, or an object that cannot be decoded, is skipped so one bad item does not fail the whole
// pull, and a store write failure is fatal. The two-way sync uses Discover plus the reconcile instead, since a
// naive object save here would overwrite an unpushed local edit.
func (s *CalDAVSyncService) Pull(ctx context.Context) (int, error) {
	records, err := s.Discover(ctx)
	if err != nil {
		return 0, err
	}
	saved := 0
	for _, record := range records {
		objects, err := s.source.ListObjects(ctx, RemoteCalendar{Path: record.Href, DisplayName: record.Name})
		if err != nil {
			continue
		}
		for _, object := range objects {
			n, err := s.saveObject(ctx, record.CalendarID, object)
			if err != nil {
				return saved, err
			}
			saved += n
		}
	}
	return saved, nil
}

// saveRemoteCalendar mirrors one remote collection as a local calendar and returns its record. The collection's
// display name falls back to its resource path when the server reports none, since a calendar name must be
// non-empty. Any CTag already recorded for the collection is preserved and carried on the record, so a
// re-discovery does not wipe the value the last sync stored (which the reconcile compares to skip an unchanged
// collection); a first discovery has none and stores the empty string.
func (s *CalDAVSyncService) saveRemoteCalendar(ctx context.Context, calendar RemoteCalendar) (RemoteCalendarRecord, error) {
	calendarID := remoteCalendarID(s.accountID, calendar.Path)
	name := calendar.DisplayName
	if name == "" {
		name = calendar.Path
	}
	ctag, err := s.store.CalendarCTag(ctx, calendarID)
	if err != nil {
		return RemoteCalendarRecord{}, fmt.Errorf("caldav: read calendar ctag: %w", err)
	}
	mirror, err := domain.NewCalendar(calendarID, name, DefaultRemoteCalendarColour)
	if err != nil {
		return RemoteCalendarRecord{}, fmt.Errorf("caldav: build calendar: %w", err)
	}
	if err := s.store.SaveRemoteCalendar(ctx, mirror, s.accountID, calendar.Path, ctag); err != nil {
		return RemoteCalendarRecord{}, fmt.Errorf("caldav: save calendar: %w", err)
	}
	return RemoteCalendarRecord{CalendarID: calendarID, AccountID: s.accountID, Href: calendar.Path, Name: name, CTag: ctag}, nil
}

// saveObject decodes one remote object and saves each of its events into the collection's local calendar,
// tagged with the object's href and etag, returning how many were saved. An object that cannot be decoded is
// skipped (zero saved, no error); a store write failure is fatal.
func (s *CalDAVSyncService) saveObject(ctx context.Context, calendarID string, object RemoteObject) (int, error) {
	events, _, decErr := s.codec.Decode(object.Data)
	if decErr != nil {
		return 0, nil
	}
	saved := 0
	for _, event := range events {
		if err := s.store.SaveSyncedEvent(ctx, event.WithCalendarID(calendarID), object.Href, object.ETag); err != nil {
			return saved, fmt.Errorf("caldav: save event: %w", err)
		}
		saved++
	}
	return saved, nil
}
