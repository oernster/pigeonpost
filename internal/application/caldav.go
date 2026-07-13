package application

import (
	"context"
	"fmt"
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

// CalDAVSyncService performs a read-only pull of remote CalDAV calendars into the local calendar store. It
// is the first slice of the two-way DAV sync: it lists the account's calendars, fetches their objects,
// decodes each with the calendar codec and saves the events locally. Write-back, per-object etag tracking
// and incremental sync-collection come in later phases.
type CalDAVSyncService struct {
	source CalDAVSource
	codec  CalendarCodec
	store  CalendarStore
}

// NewCalDAVSyncService wires a read-only CalDAV pull over its source, codec and store.
func NewCalDAVSyncService(source CalDAVSource, codec CalendarCodec, store CalendarStore) *CalDAVSyncService {
	return &CalDAVSyncService{source: source, codec: codec, store: store}
}

// Pull fetches every remote calendar's objects, decodes them and saves the events locally, returning the
// number of events saved. A calendar whose objects cannot be listed, or an object that cannot be decoded,
// is skipped so one bad item does not fail the whole pull. A store write failure is fatal because it
// signals a broken local database rather than a single unusable remote item.
func (s *CalDAVSyncService) Pull(ctx context.Context) (int, error) {
	calendars, err := s.source.ListCalendars(ctx)
	if err != nil {
		return 0, fmt.Errorf("caldav: list calendars: %w", err)
	}
	saved := 0
	for _, calendar := range calendars {
		objects, err := s.source.ListObjects(ctx, calendar)
		if err != nil {
			continue
		}
		for _, object := range objects {
			events, _, decErr := s.codec.Decode(object.Data)
			if decErr != nil {
				continue
			}
			for _, event := range events {
				if err := s.store.SaveEvent(ctx, event); err != nil {
					return saved, fmt.Errorf("caldav: save event: %w", err)
				}
				saved++
			}
		}
	}
	return saved, nil
}
