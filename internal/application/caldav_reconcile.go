package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
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
		ctag, ctagErr := source.CollectionCTag(ctx, record.Href)
		if ctagErr == nil && ctag != "" && ctag == record.CTag {
			// The collection is unchanged since the last sync, so its objects need not be fetched or merged.
			continue
		}
		objects, err := source.ListObjects(ctx, RemoteCalendar{Path: record.Href, DisplayName: record.Name})
		if err != nil {
			continue
		}
		merged := s.reconcileCalendar(ctx, record, objects, pendingFor(pending, record.CalendarID))
		if merged && ctagErr == nil && ctag != "" {
			// Record the CTag just reconciled against, but only after the merge fully landed. A collection that
			// could not be read, or an object that failed to persist, leaves merged false so the CTag is not
			// advanced and the collection is re-reconciled next sync rather than being wrongly skipped.
			_ = s.store.UpdateCalendarCTag(ctx, record.CalendarID, ctag)
		}
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
// It reports whether the merge ran: a failure to read the local objects returns false, so the caller does not
// advance the collection's CTag over a merge that never happened.
func (s *CalDAVReconcileService) reconcileCalendar(ctx context.Context, record RemoteCalendarRecord, objects []RemoteObject, pending map[string]PendingCalendarObject) bool {
	local, err := s.store.ListSyncedObjects(ctx, record.CalendarID)
	if err != nil {
		return false
	}
	localETag := map[string]string{}
	for _, o := range local {
		localETag[o.Href] = o.ETag
	}
	merged := true
	onServer := map[string]bool{}
	for _, obj := range objects {
		onServer[obj.Href] = true
		if !s.reconcileServerObject(ctx, record, obj, localETag[obj.Href], pending) {
			merged = false
		}
	}
	for href := range localETag {
		if onServer[href] {
			continue
		}
		if !s.reconcileMissingObject(ctx, record, href, pending) {
			merged = false
		}
	}
	return merged
}

// reconcileServerObject settles one object present on the server, reporting whether it fully landed. A pending
// op guards its object unless the server changed under it (a conflict), in which case the server wins and the
// local version is kept as a safety copy; with no pending op the server wins whenever its etag differs from the
// local one. It returns false when any part of a change failed to persist, so the caller does not advance the
// collection's CTag over a half-applied object.
func (s *CalDAVReconcileService) reconcileServerObject(ctx context.Context, record RemoteCalendarRecord, obj RemoteObject, localETag string, pending map[string]PendingCalendarObject) bool {
	if p, ok := pending[obj.Href]; ok {
		if p.BaseETag == obj.ETag {
			return true
		}
		safe := s.saveSafetyCopy(ctx, obj.Href)
		applied := s.applyServerObject(ctx, record, obj)
		cleared := s.store.ClearPendingCalendarOp(ctx, record.CalendarID, obj.Href) == nil
		return safe && applied && cleared
	}
	if localETag == obj.ETag {
		return true
	}
	return s.applyServerObject(ctx, record, obj)
}

// reconcileMissingObject settles one object present locally but gone from the server, reporting whether it fully
// landed. A pending create is left for Flush to push; a pending update on a since-deleted object keeps a safety
// copy before dropping it; with no pending op the server's removal wins and the local rows are dropped.
func (s *CalDAVReconcileService) reconcileMissingObject(ctx context.Context, record RemoteCalendarRecord, href string, pending map[string]PendingCalendarObject) bool {
	if p, ok := pending[href]; ok {
		if p.Op == CalendarOpCreate {
			return true
		}
		safe := true
		if p.Op == CalendarOpUpdate {
			safe = s.saveSafetyCopy(ctx, href)
		}
		deleted := s.store.DeleteEventsByHref(ctx, href) == nil
		cleared := s.store.ClearPendingCalendarOp(ctx, record.CalendarID, href) == nil
		return safe && deleted && cleared
	}
	return s.store.DeleteEventsByHref(ctx, href) == nil
}

// applyServerObject decodes a server object and saves its events into the collection's local calendar, tagged
// with the object's href and etag, reporting whether every event persisted. A body that cannot be decoded, or
// an event that fails to save, returns false so the CTag is withheld and the object is retried next sync.
func (s *CalDAVReconcileService) applyServerObject(ctx context.Context, record RemoteCalendarRecord, obj RemoteObject) bool {
	events, _, err := s.codec.Decode(obj.Data)
	if err != nil {
		return false
	}
	tagged := make([]domain.Event, 0, len(events))
	for _, e := range events {
		tagged = append(tagged, e.WithCalendarID(record.CalendarID))
	}
	return s.persistAll(ctx, tagged, obj.Href, obj.ETag)
}

// saveSafetyCopy preserves the current local version of an object as new local-only rows (a fresh id, no href
// or etag) so a last-writer-wins overwrite does not lose the user's edit, reporting whether it fully persisted.
// An object with no local rows is nothing to copy and succeeds; a failure to read or save the rows returns
// false so the CTag is withheld.
func (s *CalDAVReconcileService) saveSafetyCopy(ctx context.Context, href string) bool {
	events, err := s.store.EventsByHref(ctx, href)
	if err != nil {
		return false
	}
	copies := make([]domain.Event, 0, len(events))
	for _, e := range events {
		copies = append(copies, e.WithID(s.newID()))
	}
	return s.persistAll(ctx, copies, "", "")
}

// persistAll saves every event tagged with href and etag, reporting whether all succeeded. A save failure does
// not stop the others (the merge stays best-effort), but it marks the merge incomplete so the caller withholds
// the collection's CTag and re-reconciles next sync rather than latching the failure into a permanent skip.
func (s *CalDAVReconcileService) persistAll(ctx context.Context, events []domain.Event, href, etag string) bool {
	ok := true
	for _, e := range events {
		if err := s.store.SaveSyncedEvent(ctx, e, href, etag); err != nil {
			ok = false
		}
	}
	return ok
}
