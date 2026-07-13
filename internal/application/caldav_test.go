package application

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

type fakeCalDAVSource struct {
	calendars  []RemoteCalendar
	listCalErr error
	objects    map[string][]RemoteObject
	listObjErr map[string]error
}

func (f *fakeCalDAVSource) ListCalendars(context.Context) ([]RemoteCalendar, error) {
	if f.listCalErr != nil {
		return nil, f.listCalErr
	}
	return f.calendars, nil
}

func (f *fakeCalDAVSource) ListObjects(_ context.Context, c RemoteCalendar) ([]RemoteObject, error) {
	if err := f.listObjErr[c.Path]; err != nil {
		return nil, err
	}
	return f.objects[c.Path], nil
}

// davCodec decodes an object body by looking its bytes up in decode; a body absent from the map
// yields a decode error, which the sync must skip.
type davCodec struct {
	decode map[string][]domain.Event
}

func (f *davCodec) Decode(data []byte) ([]domain.Event, []domain.CalendarPassthrough, error) {
	events, ok := f.decode[string(data)]
	if !ok {
		return nil, nil, errBoom
	}
	return events, nil, nil
}

func (f *davCodec) Encode([]domain.Event, []domain.CalendarPassthrough) ([]byte, error) {
	return nil, nil
}

func davEvent(t *testing.T, id string) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{ID: id, CalendarID: "cal1", Summary: "Ev", Start: day(4, 9)})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return e
}

func TestCalDAVPullListCalendarsError(t *testing.T) {
	svc := NewCalDAVSyncService(&fakeCalDAVSource{listCalErr: errBoom}, &davCodec{}, &fakeCalendarStore{})
	if _, err := svc.Pull(context.Background()); err == nil {
		t.Fatal("expected an error when listing calendars fails")
	}
}

func TestCalDAVPullSkipsCalendarWhoseObjectsFail(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars:  []RemoteCalendar{{Path: "/a"}, {Path: "/b"}},
		listObjErr: map[string]error{"/a": errBoom},
		objects:    map[string][]RemoteObject{"/b": {{Href: "/b/1", Data: []byte("EV1")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV1": {davEvent(t, "e1")}}}
	store := &fakeCalendarStore{}
	n, err := NewCalDAVSyncService(src, codec, store).Pull(context.Background())
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 1 {
		t.Errorf("saved = %d, want 1 (calendar /a skipped, /b pulled)", n)
	}
}

func TestCalDAVPullSkipsUndecodableObject(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a"}},
		objects:   map[string][]RemoteObject{"/a": {{Data: []byte("BAD")}, {Data: []byte("GOOD")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"GOOD": {davEvent(t, "e1")}}}
	n, err := NewCalDAVSyncService(src, codec, &fakeCalendarStore{}).Pull(context.Background())
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 1 {
		t.Errorf("saved = %d, want 1 (undecodable object skipped)", n)
	}
}

func TestCalDAVPullSaveErrorIsFatal(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a"}},
		objects:   map[string][]RemoteObject{"/a": {{Data: []byte("EV")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV": {davEvent(t, "e1")}}}
	svc := NewCalDAVSyncService(src, codec, &fakeCalendarStore{saveEvtErr: errBoom})
	if _, err := svc.Pull(context.Background()); err == nil {
		t.Fatal("expected a fatal error when SaveEvent fails")
	}
}

func TestCalDAVPullSavesAllEvents(t *testing.T) {
	src := &fakeCalDAVSource{
		calendars: []RemoteCalendar{{Path: "/a"}},
		objects:   map[string][]RemoteObject{"/a": {{Data: []byte("EV")}}},
	}
	codec := &davCodec{decode: map[string][]domain.Event{"EV": {davEvent(t, "e1"), davEvent(t, "e2")}}}
	store := &fakeCalendarStore{}
	n, err := NewCalDAVSyncService(src, codec, store).Pull(context.Background())
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if n != 2 {
		t.Errorf("saved = %d, want 2", n)
	}
	if len(store.savedEvt) != 2 {
		t.Errorf("store saved %d events, want 2", len(store.savedEvt))
	}
}
