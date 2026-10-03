package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// CalDAVReconcileService is the pull-and-merge side of two-way CalDAV sync. Reconcile fetches each known
// collection's objects and aligns the local store: a new or server-changed object is saved; an object removed
// on the server is deleted; a pending local change guards its object until the server agrees. A genuine
// conflict (the server changed under a pending local edit) resolves last-writer-wins in the server's favour,
// preserving the losing local version as a safety copy so the user's edit is never silently lost. It mirrors
// the tag two-way sync's ReconcileFetched and is best-effort: a collection or object that cannot be read does
// not stop the others. Every such failure is still returned so the sync never reads as complete when it was not.
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

// Reconcile aligns the local store for each given collection against a fresh pull from source. Failing to
// read the pending list stops it. A collection that cannot be listed is skipped with its failure collected
// into the returned error; so is one whose merge did not fully land.
func (s *CalDAVReconcileService) Reconcile(ctx context.Context, source CalDAVSource, records []RemoteCalendarRecord) error {
	pending, err := s.store.ListPendingCalendarOps(ctx)
	if err != nil {
		return fmt.Errorf("caldav: list pending calendar ops: %w", err)
	}
	var failures []error
	for _, record := range records {
		if err := s.reconcileCollection(ctx, source, record, pending); err != nil {
			failures = append(failures, fmt.Errorf("caldav: calendar %q: %w", record.Name, err))
		}
	}
	return errors.Join(failures...)
}

// reconcileCollection reconciles one collection, skipping it when its CTag is unchanged since the last sync.
func (s *CalDAVReconcileService) reconcileCollection(ctx context.Context, source CalDAVSource, record RemoteCalendarRecord, pending []PendingCalendarObject) error {
	ctag, ctagErr := source.CollectionCTag(ctx, record.Href)
	if ctagErr == nil && ctag != "" && ctag == record.CTag {
		// The collection is unchanged since the last sync, so its objects need not be fetched or merged.
		return nil
	}
	objects, err := source.ListObjects(ctx, RemoteCalendar{Path: record.Href, DisplayName: record.Name})
	if err != nil {
		return fmt.Errorf("list objects: %w", err)
	}
	if err := s.reconcileCalendar(ctx, record, objects, pendingFor(pending, record.CalendarID)); err != nil {
		// The CTag is not advanced over a merge that did not fully land, so the collection is re-reconciled
		// next sync rather than being wrongly skipped.
		return err
	}
	if ctagErr == nil && ctag != "" {
		if err := s.store.UpdateCalendarCTag(ctx, record.CalendarID, ctag); err != nil {
			return fmt.Errorf("record ctag: %w", err)
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

// reconcileCalendar compares the server objects against the local ones for a collection and applies the merge,
// returning every failure. A failure to read the local objects returns at once, since no merge can run.
func (s *CalDAVReconcileService) reconcileCalendar(ctx context.Context, record RemoteCalendarRecord, objects []RemoteObject, pending map[string]PendingCalendarObject) error {
	local, err := s.store.ListSyncedObjects(ctx, record.CalendarID)
	if err != nil {
		return fmt.Errorf("read local objects: %w", err)
	}
	localETag := map[string]string{}
	for _, o := range local {
		localETag[o.Href] = o.ETag
	}
	var failures []error
	onServer := map[string]bool{}
	for _, obj := range objects {
		onServer[obj.Href] = true
		failures = append(failures, s.reconcileServerObject(ctx, record, obj, localETag[obj.Href], pending))
	}
	for href := range localETag {
		if !onServer[href] {
			failures = append(failures, s.reconcileMissingObject(ctx, record, href, pending))
		}
	}
	return errors.Join(failures...)
}

// reconcileServerObject settles one object present on the server, returning any part that failed to land. A
// pending op guards its object unless the server changed under it (a conflict), in which case the server wins
// and the local version is kept as a safety copy; with no pending op the server wins whenever its etag differs
// from the local one.
func (s *CalDAVReconcileService) reconcileServerObject(ctx context.Context, record RemoteCalendarRecord, obj RemoteObject, localETag string, pending map[string]PendingCalendarObject) error {
	if p, ok := pending[obj.Href]; ok {
		if p.BaseETag == obj.ETag {
			return nil
		}
		return errors.Join(
			s.saveSafetyCopy(ctx, obj.Href),
			s.applyServerObject(ctx, record, obj),
			s.store.ClearPendingCalendarOp(ctx, record.CalendarID, obj.Href),
		)
	}
	if localETag == obj.ETag {
		return nil
	}
	return s.applyServerObject(ctx, record, obj)
}

// reconcileMissingObject settles one object present locally but gone from the server, returning any part that
// failed to land. A pending create is left for Flush to push; a pending update on a since-deleted object keeps
// a safety copy before dropping it; with no pending op the server's removal wins and the local rows are dropped.
func (s *CalDAVReconcileService) reconcileMissingObject(ctx context.Context, record RemoteCalendarRecord, href string, pending map[string]PendingCalendarObject) error {
	p, ok := pending[href]
	if !ok {
		return s.store.DeleteEventsByHref(ctx, href)
	}
	if p.Op == CalendarOpCreate {
		return nil
	}
	var safe error
	if p.Op == CalendarOpUpdate {
		safe = s.saveSafetyCopy(ctx, href)
	}
	return errors.Join(
		safe,
		s.store.DeleteEventsByHref(ctx, href),
		s.store.ClearPendingCalendarOp(ctx, record.CalendarID, href),
	)
}

// applyServerObject decodes a server object and saves its events into the collection's local calendar, tagged
// with the object's href and etag. A body that cannot be decoded returns an error so the CTag is withheld and
// the object is retried next sync; so does an event that fails to save.
func (s *CalDAVReconcileService) applyServerObject(ctx context.Context, record RemoteCalendarRecord, obj RemoteObject) error {
	tagged, err := decodeTagged(s.codec, obj.Data, record.CalendarID)
	if err != nil {
		return fmt.Errorf("decode %q: %w", obj.Href, err)
	}
	return s.persistAll(ctx, tagged, obj.Href, obj.ETag)
}

// saveSafetyCopy preserves the current local version of an object as new local-only rows (a fresh id, no href
// or etag) so a last-writer-wins overwrite does not lose the user's edit. An object with no local rows is
// nothing to copy and succeeds; a failure to read or save the rows is returned so the CTag is withheld.
func (s *CalDAVReconcileService) saveSafetyCopy(ctx context.Context, href string) error {
	events, err := s.store.EventsByHref(ctx, href)
	if err != nil {
		return fmt.Errorf("read local copy of %q: %w", href, err)
	}
	copies := make([]domain.Event, 0, len(events))
	for _, e := range events {
		copies = append(copies, e.WithID(s.newID()))
	}
	return s.persistAll(ctx, copies, "", "")
}

// persistAll saves every event tagged with href and etag, returning every failure. A save failure does not
// stop the others (the merge stays best-effort); it marks the merge incomplete so the caller withholds the
// collection's CTag and re-reconciles next sync rather than latching the failure into a permanent skip.
func (s *CalDAVReconcileService) persistAll(ctx context.Context, events []domain.Event, href, etag string) error {
	var failures []error
	for _, e := range events {
		if err := s.store.SaveSyncedEvent(ctx, e, href, etag); err != nil {
			failures = append(failures, fmt.Errorf("save %q: %w", e.ID(), err))
		}
	}
	return errors.Join(failures...)
}
