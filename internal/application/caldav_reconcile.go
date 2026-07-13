package application

import (
	"context"
	"fmt"
)

// CalDAVReconcileService is the pull-and-merge side of two-way CalDAV sync. Reconcile fetches each known
// collection's objects and aligns the local store: a new or server-changed object is saved, an object removed
// on the server is deleted, and a pending local change guards its object until the server agrees. A genuine
// conflict (the server changed under a pending local edit) resolves last-writer-wins in the server's favour,
// preserving the losing local version as a safety copy so the user's edit is never silently lost. It mirrors
// the tag two-way sync's ReconcileFetched and is best-effort: a collection or object that cannot be read is
// skipped rather than failing the whole sync.
type CalDAVReconcileService struct {
	store CalendarSyncStore
	codec CalendarCodec
	newID func() string
}

// NewCalDAVReconcileService wires the reconcile engine over its sync store, codec and id generator (used for
// safety-copy rows).
func NewCalDAVReconcileService(store CalendarSyncStore, codec CalendarCodec, newID func() string) *CalDAVReconcileService {
	return &CalDAVReconcileService{store: store, codec: codec, newID: newID}
}

// Reconcile aligns the local store for each given collection against a fresh pull from source. The only fatal
// error is failing to read the pending list; a collection whose objects cannot be listed is skipped.
func (s *CalDAVReconcileService) Reconcile(ctx context.Context, source CalDAVSource, records []RemoteCalendarRecord) error {
	pending, err := s.store.ListPendingCalendarOps(ctx)
	if err != nil {
		return fmt.Errorf("caldav: list pending calendar ops: %w", err)
	}
	for _, record := range records {
		objects, err := source.ListObjects(ctx, RemoteCalendar{Path: record.Href, DisplayName: record.Name})
		if err != nil {
			continue
		}
		s.reconcileCalendar(ctx, record, objects, pendingFor(pending, record.CalendarID))
	}
	return nil
}

// pendingFor indexes the pending ops of one calendar by object href.
func pendingFor(all []PendingCalendarObject, calendarID string) map[string]PendingCalendarObject {
	out := map[string]PendingCalendarObject{}
	for _, p := range all {
		if p.CalendarID == calendarID {
			out[p.Href] = p
		}
	}
	return out
}

// reconcileCalendar compares the server objects against the local ones for a collection and applies the merge.
func (s *CalDAVReconcileService) reconcileCalendar(ctx context.Context, record RemoteCalendarRecord, objects []RemoteObject, pending map[string]PendingCalendarObject) {
	local, err := s.store.ListSyncedObjects(ctx, record.CalendarID)
	if err != nil {
		return
	}
	localETag := map[string]string{}
	for _, o := range local {
		localETag[o.Href] = o.ETag
	}
	onServer := map[string]bool{}
	for _, obj := range objects {
		onServer[obj.Href] = true
		s.reconcileServerObject(ctx, record, obj, localETag[obj.Href], pending)
	}
	for href := range localETag {
		if onServer[href] {
			continue
		}
		s.reconcileMissingObject(ctx, record, href, pending)
	}
}

// reconcileServerObject settles one object present on the server. A pending op guards its object unless the
// server changed under it (a conflict), in which case the server wins and the local version is kept as a
// safety copy; with no pending op the server wins whenever its etag differs from the local one.
func (s *CalDAVReconcileService) reconcileServerObject(ctx context.Context, record RemoteCalendarRecord, obj RemoteObject, localETag string, pending map[string]PendingCalendarObject) {
	if p, ok := pending[obj.Href]; ok {
		if p.BaseETag == obj.ETag {
			return
		}
		s.saveSafetyCopy(ctx, obj.Href)
		s.applyServerObject(ctx, record, obj)
		_ = s.store.ClearPendingCalendarOp(ctx, record.CalendarID, obj.Href)
		return
	}
	if localETag == obj.ETag {
		return
	}
	s.applyServerObject(ctx, record, obj)
}

// reconcileMissingObject settles one object present locally but gone from the server. A pending create is left
// for Flush to push; a pending update on a since-deleted object keeps a safety copy before dropping it; with
// no pending op the server's removal wins and the local rows are dropped.
func (s *CalDAVReconcileService) reconcileMissingObject(ctx context.Context, record RemoteCalendarRecord, href string, pending map[string]PendingCalendarObject) {
	if p, ok := pending[href]; ok {
		if p.Op == CalendarOpCreate {
			return
		}
		if p.Op == CalendarOpUpdate {
			s.saveSafetyCopy(ctx, href)
		}
		_ = s.store.DeleteEventsByHref(ctx, href)
		_ = s.store.ClearPendingCalendarOp(ctx, record.CalendarID, href)
		return
	}
	_ = s.store.DeleteEventsByHref(ctx, href)
}

// applyServerObject decodes a server object and saves its events into the collection's local calendar, tagged
// with the object's href and etag. A body that cannot be decoded is skipped.
func (s *CalDAVReconcileService) applyServerObject(ctx context.Context, record RemoteCalendarRecord, obj RemoteObject) {
	events, _, err := s.codec.Decode(obj.Data)
	if err != nil {
		return
	}
	for _, e := range events {
		_ = s.store.SaveSyncedEvent(ctx, e.WithCalendarID(record.CalendarID), obj.Href, obj.ETag)
	}
}

// saveSafetyCopy preserves the current local version of an object as new local-only rows (a fresh id, no href
// or etag) so a last-writer-wins overwrite does not lose the user's edit.
func (s *CalDAVReconcileService) saveSafetyCopy(ctx context.Context, href string) {
	events, err := s.store.EventsByHref(ctx, href)
	if err != nil || len(events) == 0 {
		return
	}
	for _, e := range events {
		_ = s.store.SaveSyncedEvent(ctx, e.WithID(s.newID()), "", "")
	}
}
