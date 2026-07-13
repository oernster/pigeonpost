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

// Pull mirrors each remote collection as a local calendar and saves its objects' events, returning the number
// of events saved. A calendar whose objects cannot be listed, or an object that cannot be decoded, is skipped
// so one bad item does not fail the whole pull. A store write failure (saving a calendar or an event) is fatal
// because it signals a broken local database rather than a single unusable remote item.
func (s *CalDAVSyncService) Pull(ctx context.Context) (int, error) {
	calendars, err := s.source.ListCalendars(ctx)
	if err != nil {
		return 0, fmt.Errorf("caldav: list calendars: %w", err)
	}
	saved := 0
	for _, calendar := range calendars {
		calendarID, err := s.saveRemoteCalendar(ctx, calendar)
		if err != nil {
			return saved, err
		}
		objects, err := s.source.ListObjects(ctx, calendar)
		if err != nil {
			continue
		}
		for _, object := range objects {
			n, err := s.saveObject(ctx, calendarID, object)
			if err != nil {
				return saved, err
			}
			saved += n
		}
	}
	return saved, nil
}

// saveRemoteCalendar mirrors one remote collection as a local calendar and returns the local calendar id. The
// collection's display name falls back to its resource path when the server reports none, since a calendar
// name must be non-empty. The CTag is left empty for now; a later slice reads it to skip an unchanged pull.
func (s *CalDAVSyncService) saveRemoteCalendar(ctx context.Context, calendar RemoteCalendar) (string, error) {
	calendarID := remoteCalendarID(s.accountID, calendar.Path)
	name := calendar.DisplayName
	if name == "" {
		name = calendar.Path
	}
	mirror, err := domain.NewCalendar(calendarID, name, DefaultRemoteCalendarColour)
	if err != nil {
		return "", fmt.Errorf("caldav: build calendar: %w", err)
	}
	if err := s.store.SaveRemoteCalendar(ctx, mirror, s.accountID, calendar.Path, ""); err != nil {
		return "", fmt.Errorf("caldav: save calendar: %w", err)
	}
	return calendarID, nil
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
