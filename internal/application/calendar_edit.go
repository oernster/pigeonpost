package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// CalendarEditService is the calendar-editing entry point that keeps a remote CalDAV calendar in step with the
// user's local changes. Creating, editing or deleting an event in a calendar mirrored from a CalDAV collection
// records a pending write intent (a create, update or delete) that a later sync flushes to the server; the
// same operations on a purely local calendar are delegated unchanged to the plain CalendarService. It is the
// calendar analogue of TagSyncService's assign and unassign: the local change and its intent are written
// together, so an edit can never be applied locally with no intent recorded to push it.
type CalendarEditService struct {
	calendar *CalendarService
	store    CalendarSyncStore
	newID    IDGenerator
}

// NewCalendarEditService wires the service over the plain calendar service (used for local calendars and as
// the event builder's id source), the sync store and the id generator.
func NewCalendarEditService(calendar *CalendarService, store CalendarSyncStore, newID IDGenerator) *CalendarEditService {
	return &CalendarEditService{calendar: calendar, store: store, newID: newID}
}

// SaveEvent creates or updates an event and returns its id. For an event in a remote calendar it saves the
// event and records the matching pending write intent atomically: a create when the event is new to the
// collection (pushed later with If-None-Match:*), an update when it already maps to an object (pushed with
// If-Match on the last-seen etag). For an event in a local calendar it delegates to the plain save unchanged.
func (s *CalendarEditService) SaveEvent(ctx context.Context, in EventInput) (string, error) {
	record, remote, err := s.store.RemoteCalendarByID(ctx, in.CalendarID)
	if err != nil {
		return "", fmt.Errorf("calendar: remote calendar: %w", err)
	}
	if !remote {
		return s.calendar.SaveEvent(ctx, in)
	}
	event, err := buildEvent(in, s.newID)
	if err != nil {
		return "", err
	}
	identity, found, err := s.store.SyncedEventIdentity(ctx, event.ID())
	if err != nil {
		return "", fmt.Errorf("calendar: event identity: %w", err)
	}
	if found && identity.Href != "" {
		op := PendingCalendarObject{CalendarID: in.CalendarID, Href: identity.Href, Op: CalendarOpUpdate, BaseETag: identity.ETag}
		if err := s.store.SaveEventWithPending(ctx, event, identity.Href, identity.ETag, op); err != nil {
			return "", fmt.Errorf("calendar: save remote event: %w", err)
		}
		return event.ID(), nil
	}
	href := objectHref(record.Href, event.UID())
	op := PendingCalendarObject{CalendarID: in.CalendarID, Href: href, Op: CalendarOpCreate}
	if err := s.store.SaveEventWithPending(ctx, event, href, "", op); err != nil {
		return "", fmt.Errorf("calendar: save remote event: %w", err)
	}
	return event.ID(), nil
}

// DeleteEvent removes an event. For an event that maps to a remote object it removes every local row of that
// object and records a pending delete intent (the tombstone a later sync sends to the server as DELETE
// If-Match) atomically; for a local-only event it delegates to the plain delete.
func (s *CalendarEditService) DeleteEvent(ctx context.Context, id string) error {
	identity, found, err := s.store.SyncedEventIdentity(ctx, id)
	if err != nil {
		return fmt.Errorf("calendar: event identity: %w", err)
	}
	if !found || identity.Href == "" {
		return s.calendar.DeleteEvent(ctx, id)
	}
	op := PendingCalendarObject{CalendarID: identity.CalendarID, Href: identity.Href, Op: CalendarOpDelete, BaseETag: identity.ETag}
	if err := s.store.DeleteObjectWithPending(ctx, identity.Href, op); err != nil {
		return fmt.Errorf("calendar: delete remote event: %w", err)
	}
	return nil
}

// objectHref builds the resource path for a new object in a collection from the collection's href and the
// event's UID, percent-escaping the UID so an unusual character in it cannot break the path. It is the name a
// created object is written under with If-None-Match:*.
func objectHref(collectionHref, uid string) string {
	return strings.TrimRight(collectionHref, "/") + "/" + url.PathEscape(uid) + ".ics"
}
