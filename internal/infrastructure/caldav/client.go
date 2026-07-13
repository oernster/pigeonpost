// Package caldav is the go-webdav-backed adapter for the application CalDAVSource port. It discovers an
// account's calendars and pulls their events, re-encoding each object to iCalendar bytes so the application
// decodes it with the same codec as a file import; the application never sees a DAV or go-ical type.
package caldav

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	dav "github.com/emersion/go-webdav/caldav"

	"github.com/oernster/pigeonpost/internal/application"
)

// Client implements application.CalDAVSource over a remote CalDAV server.
type Client struct {
	dav *dav.Client
}

// Client must satisfy the application read port.
var _ application.CalDAVSource = (*Client)(nil)

// NewClient builds a CalDAV client for a server that authenticates with HTTP Basic auth (an app password).
// endpoint is the account base URL; username and password are sent on every request.
func NewClient(endpoint, username, password string) (*Client, error) {
	httpClient := webdav.HTTPClientWithBasicAuth(http.DefaultClient, username, password)
	client, err := dav.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("caldav: new client: %w", err)
	}
	return &Client{dav: client}, nil
}

// ListCalendars discovers the account's calendar collections: it resolves the current-user principal, then
// the calendar-home-set, then lists the calendars under it.
func (c *Client) ListCalendars(ctx context.Context) ([]application.RemoteCalendar, error) {
	principal, err := c.dav.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return nil, fmt.Errorf("caldav: find current user principal: %w", err)
	}
	homeSet, err := c.dav.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("caldav: find calendar home set: %w", err)
	}
	calendars, err := c.dav.FindCalendars(ctx, homeSet)
	if err != nil {
		return nil, fmt.Errorf("caldav: find calendars: %w", err)
	}
	out := make([]application.RemoteCalendar, 0, len(calendars))
	for _, calendar := range calendars {
		out = append(out, application.RemoteCalendar{Path: calendar.Path, DisplayName: calendar.Name})
	}
	return out, nil
}

// allEventsQuery asks for every VEVENT in a collection with its full data, so a first pull fetches the whole
// calendar.
var allEventsQuery = &dav.CalendarQuery{
	CompRequest: dav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
	CompFilter:  dav.CompFilter{Name: "VCALENDAR", Comps: []dav.CompFilter{{Name: "VEVENT"}}},
}

// ListObjects fetches every event object in the calendar, re-encoding each parsed calendar back to
// iCalendar bytes. An object the server returned without data is skipped rather than failing the calendar.
func (c *Client) ListObjects(ctx context.Context, calendar application.RemoteCalendar) ([]application.RemoteObject, error) {
	objects, err := c.dav.QueryCalendar(ctx, calendar.Path, allEventsQuery)
	if err != nil {
		return nil, fmt.Errorf("caldav: query calendar %q: %w", calendar.Path, err)
	}
	out := make([]application.RemoteObject, 0, len(objects))
	for _, object := range objects {
		data, encErr := encodeICS(object.Data)
		if encErr != nil {
			continue
		}
		out = append(out, application.RemoteObject{Href: object.Path, ETag: object.ETag, Data: data})
	}
	return out, nil
}

// encodeICS serialises a parsed calendar back to iCalendar bytes. A nil calendar (an object the server
// returned without data) yields an error so the caller skips it.
func encodeICS(calendar *ical.Calendar) ([]byte, error) {
	if calendar == nil {
		return nil, fmt.Errorf("caldav: object carried no calendar data")
	}
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(calendar); err != nil {
		return nil, fmt.Errorf("caldav: encode calendar: %w", err)
	}
	return buf.Bytes(), nil
}
