package caldav

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	dav "github.com/emersion/go-webdav/caldav"

	"github.com/oernster/pigeonpost/internal/application"
)

// testBackend is a minimal in-memory CalDAV backend for driving a real go-webdav server that the client is
// tested against, so the discovery and pull go over HTTP rather than through hand-crafted XML. Only the read
// methods the pull exercises carry data; the write methods are stubs.
type testBackend struct {
	calendars []dav.Calendar
	objects   map[string][]dav.CalendarObject
}

func (b testBackend) CurrentUserPrincipal(context.Context) (string, error) { return "/user/", nil }
func (b testBackend) CalendarHomeSetPath(context.Context) (string, error) {
	return "/user/calendars/", nil
}
func (b testBackend) ListCalendars(context.Context) ([]dav.Calendar, error) { return b.calendars, nil }

func (b testBackend) GetCalendar(_ context.Context, path string) (*dav.Calendar, error) {
	for _, c := range b.calendars {
		if c.Path == path {
			found := c
			return &found, nil
		}
	}
	return nil, fmt.Errorf("no calendar at %s", path)
}

func (b testBackend) QueryCalendarObjects(_ context.Context, path string, _ *dav.CalendarQuery) ([]dav.CalendarObject, error) {
	return b.objects[path], nil
}

func (b testBackend) ListCalendarObjects(_ context.Context, path string, _ *dav.CalendarCompRequest) ([]dav.CalendarObject, error) {
	return b.objects[path], nil
}

func (b testBackend) GetCalendarObject(context.Context, string, *dav.CalendarCompRequest) (*dav.CalendarObject, error) {
	return nil, fmt.Errorf("caldav test: object not found")
}

func (b testBackend) CreateCalendar(context.Context, *dav.Calendar) error { return nil }

func (b testBackend) PutCalendarObject(context.Context, string, *ical.Calendar, *dav.PutCalendarObjectOptions) (*dav.CalendarObject, error) {
	return nil, nil
}

func (b testBackend) DeleteCalendarObject(context.Context, string) error { return nil }

func testEvent(summary string) *ical.Calendar {
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, "uid-"+summary)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC))
	event.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC))
	event.Props.SetText(ical.PropSummary, summary)
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//pigeonpost test//EN")
	cal.Children = []*ical.Component{event.Component}
	return cal
}

func serverFor(t *testing.T, backend testBackend) *Client {
	t.Helper()
	server := httptest.NewServer(&dav.Handler{Backend: backend})
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, "user", "pass")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestClientListCalendars(t *testing.T) {
	client := serverFor(t, testBackend{calendars: []dav.Calendar{
		{Path: "/user/calendars/a", Name: "Personal"},
		{Path: "/user/calendars/b", Name: "Work"},
	}})
	calendars, err := client.ListCalendars(context.Background())
	if err != nil {
		t.Fatalf("ListCalendars: %v", err)
	}
	names := map[string]string{}
	for _, c := range calendars {
		names[c.Path] = c.DisplayName
	}
	if names["/user/calendars/a"] != "Personal" || names["/user/calendars/b"] != "Work" {
		t.Errorf("calendars = %+v", calendars)
	}
}

func TestClientListObjects(t *testing.T) {
	client := serverFor(t, testBackend{
		calendars: []dav.Calendar{{Path: "/user/calendars/a", Name: "Personal"}},
		objects: map[string][]dav.CalendarObject{
			"/user/calendars/a": {{Path: "/user/calendars/a/1.ics", ETag: "etag-1", Data: testEvent("Standup")}},
		},
	})
	objects, err := client.ListObjects(context.Background(), application.RemoteCalendar{Path: "/user/calendars/a"})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("got %d objects, want 1", len(objects))
	}
	if objects[0].Href != "/user/calendars/a/1.ics" || objects[0].ETag != "etag-1" {
		t.Errorf("object metadata = %+v", objects[0])
	}
	if !bytes.Contains(objects[0].Data, []byte("SUMMARY:Standup")) {
		t.Errorf("object data does not carry the event:\n%s", objects[0].Data)
	}
}

func TestEncodeICSNilCalendar(t *testing.T) {
	if _, err := encodeICS(nil); err == nil {
		t.Error("expected an error for a nil calendar")
	}
}
