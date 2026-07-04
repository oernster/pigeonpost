package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeCalendarStore is a hand-written in-memory CalendarStore with error-injection fields.
type fakeCalendarStore struct {
	calendars  []domain.Calendar
	events     []domain.Event
	gotEvent   domain.Event
	listCalErr error
	saveCalErr error
	delCalErr  error
	listEvtErr error
	getEvtErr  error
	saveEvtErr error
	delEvtErr  error
	savedCal   []domain.Calendar
	deletedCal []string
	savedEvt   []domain.Event
	deletedEvt []string
}

func (f *fakeCalendarStore) ListCalendars(context.Context) ([]domain.Calendar, error) {
	if f.listCalErr != nil {
		return nil, f.listCalErr
	}
	return f.calendars, nil
}

func (f *fakeCalendarStore) SaveCalendar(_ context.Context, c domain.Calendar) error {
	if f.saveCalErr != nil {
		return f.saveCalErr
	}
	f.savedCal = append(f.savedCal, c)
	return nil
}

func (f *fakeCalendarStore) DeleteCalendar(_ context.Context, id string) error {
	if f.delCalErr != nil {
		return f.delCalErr
	}
	f.deletedCal = append(f.deletedCal, id)
	return nil
}

func (f *fakeCalendarStore) ListEvents(context.Context) ([]domain.Event, error) {
	if f.listEvtErr != nil {
		return nil, f.listEvtErr
	}
	return f.events, nil
}

func (f *fakeCalendarStore) GetEvent(context.Context, string) (domain.Event, error) {
	if f.getEvtErr != nil {
		return domain.Event{}, f.getEvtErr
	}
	return f.gotEvent, nil
}

func (f *fakeCalendarStore) SaveEvent(_ context.Context, e domain.Event) error {
	if f.saveEvtErr != nil {
		return f.saveEvtErr
	}
	f.savedEvt = append(f.savedEvt, e)
	return nil
}

func (f *fakeCalendarStore) DeleteEvent(_ context.Context, id string) error {
	if f.delEvtErr != nil {
		return f.delEvtErr
	}
	f.deletedEvt = append(f.deletedEvt, id)
	return nil
}

// fakeCalendarCodec is a hand-written CalendarCodec with error-injection fields.
type fakeCalendarCodec struct {
	decoded   []domain.Event
	decodeErr error
	encoded   []byte
	encodeErr error
	gotEncode []domain.Event
}

func (f *fakeCalendarCodec) Decode([]byte) ([]domain.Event, error) {
	if f.decodeErr != nil {
		return nil, f.decodeErr
	}
	return f.decoded, nil
}

func (f *fakeCalendarCodec) Encode(es []domain.Event) ([]byte, error) {
	if f.encodeErr != nil {
		return nil, f.encodeErr
	}
	f.gotEncode = es
	return f.encoded, nil
}

func mustEvent(t *testing.T, id, summary string) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{
		ID: id, UID: id, Summary: summary, Start: time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return e
}

func TestCalendarServiceListCalendars(t *testing.T) {
	cal, _ := domain.NewCalendar("cal1", "Work", "")
	store := &fakeCalendarStore{calendars: []domain.Calendar{cal}}
	svc := NewCalendarService(store, fixedID("x"))
	got, err := svc.ListCalendars(context.Background())
	if err != nil || len(got) != 1 || got[0].ID() != "cal1" {
		t.Fatalf("ListCalendars = %v, %v", got, err)
	}
	store.listCalErr = errBoom
	if _, err := svc.ListCalendars(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCalendarServiceSaveCalendar(t *testing.T) {
	store := &fakeCalendarStore{}
	svc := NewCalendarService(store, fixedID("generated"))

	if err := svc.SaveCalendar(context.Background(), CalendarInput{Name: "Work"}); err != nil {
		t.Fatalf("SaveCalendar: %v", err)
	}
	if store.savedCal[0].ID() != "generated" {
		t.Errorf("id = %q, want generated", store.savedCal[0].ID())
	}
	if err := svc.SaveCalendar(context.Background(), CalendarInput{ID: " c2 ", Name: "Home"}); err != nil {
		t.Fatalf("SaveCalendar: %v", err)
	}
	if store.savedCal[1].ID() != "c2" {
		t.Errorf("id = %q, want c2", store.savedCal[1].ID())
	}
	if err := svc.SaveCalendar(context.Background(), CalendarInput{Name: "  "}); !errors.Is(err, domain.ErrEmptyCalendarName) {
		t.Errorf("err = %v, want ErrEmptyCalendarName", err)
	}
	store.saveCalErr = errBoom
	if err := svc.SaveCalendar(context.Background(), CalendarInput{Name: "Work"}); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCalendarServiceDeleteCalendar(t *testing.T) {
	store := &fakeCalendarStore{}
	svc := NewCalendarService(store, fixedID("x"))
	if err := svc.DeleteCalendar(context.Background(), "cal1"); err != nil {
		t.Fatalf("DeleteCalendar: %v", err)
	}
	if len(store.deletedCal) != 1 || store.deletedCal[0] != "cal1" {
		t.Errorf("deleted = %v", store.deletedCal)
	}
	store.delCalErr = errBoom
	if err := svc.DeleteCalendar(context.Background(), "cal1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCalendarServiceListEvents(t *testing.T) {
	store := &fakeCalendarStore{events: []domain.Event{mustEvent(t, "e1", "Standup")}}
	svc := NewCalendarService(store, fixedID("x"))
	got, err := svc.ListEvents(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("ListEvents = %v, %v", got, err)
	}
	store.listEvtErr = errBoom
	if _, err := svc.ListEvents(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCalendarServiceGetEvent(t *testing.T) {
	store := &fakeCalendarStore{gotEvent: mustEvent(t, "e1", "Standup")}
	svc := NewCalendarService(store, fixedID("x"))
	got, err := svc.GetEvent(context.Background(), "e1")
	if err != nil || got.ID() != "e1" {
		t.Fatalf("GetEvent = %v, %v", got, err)
	}
	store.getEvtErr = errBoom
	if _, err := svc.GetEvent(context.Background(), "e1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCalendarServiceSaveEvent(t *testing.T) {
	store := &fakeCalendarStore{}
	svc := NewCalendarService(store, fixedID("generated"))
	start := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)

	if err := svc.SaveEvent(context.Background(), EventInput{Summary: "Standup", Start: start}); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if store.savedEvt[0].ID() != "generated" {
		t.Errorf("id = %q, want generated", store.savedEvt[0].ID())
	}
	if err := svc.SaveEvent(context.Background(), EventInput{ID: " e2 ", Summary: "Review", Start: start}); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if store.savedEvt[1].ID() != "e2" {
		t.Errorf("id = %q, want e2", store.savedEvt[1].ID())
	}
	if err := svc.SaveEvent(context.Background(), EventInput{Summary: "  ", Start: start}); !errors.Is(err, domain.ErrEmptyEventSummary) {
		t.Errorf("err = %v, want ErrEmptyEventSummary", err)
	}
	store.saveEvtErr = errBoom
	if err := svc.SaveEvent(context.Background(), EventInput{Summary: "Standup", Start: start}); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCalendarServiceDeleteEvent(t *testing.T) {
	store := &fakeCalendarStore{}
	svc := NewCalendarService(store, fixedID("x"))
	if err := svc.DeleteEvent(context.Background(), "e1"); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if len(store.deletedEvt) != 1 || store.deletedEvt[0] != "e1" {
		t.Errorf("deleted = %v", store.deletedEvt)
	}
	store.delEvtErr = errBoom
	if err := svc.DeleteEvent(context.Background(), "e1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCalendarServiceImportEvents(t *testing.T) {
	svc := NewCalendarService(&fakeCalendarStore{}, fixedID("x"))
	if n, err := svc.ImportEvents(context.Background(), &fakeCalendarCodec{decodeErr: errBoom}, nil); n != 0 || !errors.Is(err, errBoom) {
		t.Errorf("decode err path = %d, %v", n, err)
	}

	store := &fakeCalendarStore{saveEvtErr: errBoom}
	codec := &fakeCalendarCodec{decoded: []domain.Event{mustEvent(t, "e1", "A"), mustEvent(t, "e2", "B")}}
	if n, err := NewCalendarService(store, fixedID("x")).ImportEvents(context.Background(), codec, nil); n != 0 || !errors.Is(err, errBoom) {
		t.Errorf("save err path = %d, %v", n, err)
	}

	good := &fakeCalendarStore{}
	if n, err := NewCalendarService(good, fixedID("x")).ImportEvents(context.Background(), codec, []byte("x")); err != nil || n != 2 {
		t.Fatalf("import = %d, %v; want 2", n, err)
	}
	if len(good.savedEvt) != 2 {
		t.Errorf("saved %d events, want 2", len(good.savedEvt))
	}
}

func TestCalendarServiceExportEvents(t *testing.T) {
	if _, err := NewCalendarService(&fakeCalendarStore{listEvtErr: errBoom}, fixedID("x")).
		ExportEvents(context.Background(), &fakeCalendarCodec{}); !errors.Is(err, errBoom) {
		t.Errorf("list err = %v, want wrapped errBoom", err)
	}
	store := &fakeCalendarStore{events: []domain.Event{mustEvent(t, "e1", "A")}}
	if _, err := NewCalendarService(store, fixedID("x")).
		ExportEvents(context.Background(), &fakeCalendarCodec{encodeErr: errBoom}); !errors.Is(err, errBoom) {
		t.Errorf("encode err = %v, want wrapped errBoom", err)
	}
	codec := &fakeCalendarCodec{encoded: []byte("BEGIN:VCALENDAR")}
	data, err := NewCalendarService(store, fixedID("x")).ExportEvents(context.Background(), codec)
	if err != nil || string(data) != "BEGIN:VCALENDAR" {
		t.Fatalf("export = %q, %v", data, err)
	}
	if len(codec.gotEncode) != 1 || codec.gotEncode[0].ID() != "e1" {
		t.Errorf("codec received %+v", codec.gotEncode)
	}
}
