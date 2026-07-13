// Package caldav is the go-webdav-backed adapter for the application CalDAVSource port. It discovers an
// account's calendars and pulls their events, re-encoding each object to iCalendar bytes so the application
// decodes it with the same codec as a file import; the application never sees a DAV or go-ical type.
package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	dav "github.com/emersion/go-webdav/caldav"

	"github.com/oernster/pigeonpost/internal/application"
)

// calendarContentType is the media type of an iCalendar object body sent on a PUT.
const calendarContentType = "text/calendar; charset=utf-8"

// ctagPropfindBody asks for a collection's calendarserver.org CTag with a Depth-0 PROPFIND. go-webdav v0.7.0
// has no CTag helper, so the request is raw, matching the raw conditional writes.
const ctagPropfindBody = `<?xml version="1.0" encoding="utf-8"?>` +
	`<propfind xmlns="DAV:" xmlns:cs="http://calendarserver.org/ns/"><prop><cs:getctag/></prop></propfind>`

// ctagMultistatus parses just the getctag value out of a PROPFIND multistatus response. Elements are matched by
// local name, ignoring namespace prefixes, so it is robust across the various ways servers qualify the tag.
type ctagMultistatus struct {
	Responses []struct {
		CTag string `xml:"propstat>prop>getctag"`
	} `xml:"response"`
}

// Client implements the application CalDAVSource read port and the CalDAVWriter write port over a remote
// CalDAV server. Reads go through go-webdav's caldav client; writes are raw conditional HTTP requests
// (PUT/DELETE with If-Match/If-None-Match), which go-webdav v0.7.0 does not support.
type Client struct {
	dav      *dav.Client
	http     webdav.HTTPClient
	endpoint string
}

// Client must satisfy both the read and write ports.
var (
	_ application.CalDAVSource = (*Client)(nil)
	_ application.CalDAVWriter = (*Client)(nil)
)

// NewClient builds a CalDAV client for a server that authenticates with HTTP Basic auth (an app password).
// endpoint is the account base URL; username and password are sent on every request. The basic-auth HTTP
// client and the endpoint are retained so the write path can issue raw conditional requests.
func NewClient(endpoint, username, password string) (*Client, error) {
	httpClient := webdav.HTTPClientWithBasicAuth(http.DefaultClient, username, password)
	client, err := dav.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("caldav: new client: %w", err)
	}
	return &Client{dav: client, http: httpClient, endpoint: endpoint}, nil
}

// PutObject writes an object's iCalendar body at href with a raw conditional PUT. ifMatch, when set, is sent
// (quoted) as If-Match to guard an update; ifNoneMatch, when set, is sent verbatim as If-None-Match ("*" for
// a create). A 412 is mapped to application.ErrCalDAVConflict; a 2xx returns the server's new etag (which may
// be empty when the server omits the header).
func (c *Client) PutObject(ctx context.Context, href string, ics []byte, ifMatch, ifNoneMatch string) (string, error) {
	target, err := c.resolve(href)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, bytes.NewReader(ics))
	if err != nil {
		return "", fmt.Errorf("caldav: build put %q: %w", href, err)
	}
	req.Header.Set("Content-Type", calendarContentType)
	if ifMatch != "" {
		req.Header.Set("If-Match", quoteETag(ifMatch))
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("caldav: put %q: %w", href, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusPreconditionFailed {
		return "", fmt.Errorf("caldav: put %q: %w", href, application.ErrCalDAVConflict)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("caldav: put %q: unexpected status %s", href, resp.Status)
	}
	return unquoteETag(resp.Header.Get("ETag")), nil
}

// DeleteObject removes the object at href with a raw conditional DELETE, sending ifMatch (quoted) as If-Match
// when set. A 412 is mapped to application.ErrCalDAVConflict; a 404 (already gone) is treated as success.
func (c *Client) DeleteObject(ctx context.Context, href, ifMatch string) error {
	target, err := c.resolve(href)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return fmt.Errorf("caldav: build delete %q: %w", href, err)
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", quoteETag(ifMatch))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("caldav: delete %q: %w", href, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusPreconditionFailed {
		return fmt.Errorf("caldav: delete %q: %w", href, application.ErrCalDAVConflict)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("caldav: delete %q: unexpected status %s", href, resp.Status)
	}
	return nil
}

// resolve turns an object href (an absolute path or a full URL) into an absolute request URL against the
// account endpoint, so a server-relative href from a listing and a client-built create path both work.
func (c *Client) resolve(href string) (string, error) {
	base, err := url.Parse(c.endpoint)
	if err != nil {
		return "", fmt.Errorf("caldav: parse endpoint %q: %w", c.endpoint, err)
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", fmt.Errorf("caldav: parse href %q: %w", href, err)
	}
	return base.ResolveReference(ref).String(), nil
}

// quoteETag wraps a bare etag value in the double quotes an If-Match/If-None-Match header requires, leaving an
// already-quoted or weak (W/) etag untouched.
func quoteETag(etag string) string {
	if strings.HasPrefix(etag, "\"") || strings.HasPrefix(etag, "W/") {
		return etag
	}
	return "\"" + etag + "\""
}

// unquoteETag strips the optional weak marker and surrounding quotes from a response ETag header, matching how
// the read path stores etags, so the stored value is ready to re-quote for the next If-Match.
func unquoteETag(etag string) string {
	etag = strings.TrimSpace(etag)
	etag = strings.TrimPrefix(etag, "W/")
	return strings.Trim(etag, "\"")
}

// CollectionCTag reads a collection's CTag with a raw Depth-0 PROPFIND for calendarserver.org getctag. A server
// that does not report the property yields the empty string (no error), so the caller reconciles the collection
// unconditionally; a transport, status or parse failure is returned as an error, which the caller treats the
// same way. go-webdav v0.7.0 exposes no CTag helper, hence the raw request.
func (c *Client) CollectionCTag(ctx context.Context, collectionHref string) (string, error) {
	target, err := c.resolve(collectionHref)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", target, strings.NewReader(ctagPropfindBody))
	if err != nil {
		return "", fmt.Errorf("caldav: build propfind %q: %w", collectionHref, err)
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "0")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("caldav: propfind %q: %w", collectionHref, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("caldav: propfind %q: unexpected status %s", collectionHref, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("caldav: read propfind %q: %w", collectionHref, err)
	}
	var ms ctagMultistatus
	if err := xml.Unmarshal(body, &ms); err != nil {
		return "", fmt.Errorf("caldav: parse propfind %q: %w", collectionHref, err)
	}
	for _, r := range ms.Responses {
		if r.CTag != "" {
			return r.CTag, nil
		}
	}
	return "", nil
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
