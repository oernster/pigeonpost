package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ErrCalDAVCollapsedSeries refuses to push an object whose local rows are an override stored under its bare
// UID with no master: the shape a series took before overrides had their own key, when an override
// overwrote its master. Pushing it would replace the server's whole series with that one occurrence, so the
// intent is held until a reconcile restores the series from the server.
var ErrCalDAVCollapsedSeries = errors.New("caldav: the local copy of this recurring event lost its series; it is not pushed until a sync restores it from the server")

// CalDAVWriteService is the write side of two-way CalDAV sync. Flush pushes pending local changes to the
// server with conditional writes: a create with If-None-Match:* so it never overwrites, an update with
// If-Match against the base etag, a delete with DELETE If-Match. It mirrors the tag two-way sync's
// FlushPending: it is best-effort, so one object's failure never stops the others; it does not resolve a
// conflict itself. A 412 leaves the pending intent (as does any other error) in place for a later Reconcile to
// settle or a later flush to retry; only a confirmed 2xx clears the intent, recording the server's new etag
// first so the next If-Match is current.
type CalDAVWriteService struct {
	store CalendarSyncStore
	codec CalendarCodec
}

// NewCalDAVWriteService wires the write engine over its sync store and codec.
func NewCalDAVWriteService(store CalendarSyncStore, codec CalendarCodec) *CalDAVWriteService {
	return &CalDAVWriteService{store: store, codec: codec}
}

// Flush pushes the pending write intents of the given account's collections to the server through writer.
// calendarIDs is the set of local calendar ids owned by the account being synced: an op for any other
// calendar is skipped, since writer is bound to this account's endpoint and credentials and must never carry
// another account's object to the wrong server. Failing to read the pending list stops the flush; a
// per-object failure does not stop the others but is collected and returned, so the sync reports that a
// change was not pushed. A conflict (ErrCalDAVConflict) is not a failure: the reconcile settles it.
func (s *CalDAVWriteService) Flush(ctx context.Context, writer CalDAVWriter, calendarIDs map[string]bool) error {
	ops, err := s.store.ListPendingCalendarOps(ctx)
	if err != nil {
		return fmt.Errorf("caldav: list pending calendar ops: %w", err)
	}
	var failures []error
	for _, op := range ops {
		if !calendarIDs[op.CalendarID] {
			continue
		}
		var err error
		if op.Op == CalendarOpDelete {
			err = s.flushDelete(ctx, writer, op)
		} else {
			err = s.flushPut(ctx, writer, op)
		}
		if err != nil && !errors.Is(err, ErrCalDAVConflict) {
			failures = append(failures, fmt.Errorf("caldav: push %q: %w", op.Href, err))
		}
	}
	return errors.Join(failures...)
}

// flushPut re-encodes the object's events and writes them: a create guards on If-None-Match:*, an update
// guards on If-Match against the base etag. On a confirmed write it stamps the returned etag onto every event
// of the object and clears the intent; on any error it leaves the intent for reconcile or a later retry and
// returns the error. An object whose events have vanished locally is skipped, leaving the intent for
// reconcile to clean up. A collapsed series (see collapsedSeries) is refused rather than pushed. Once the
// server has confirmed the write the intent is cleared even when stamping the new etag fails, since
// re-pushing an accepted write would only meet a conflict; that failure is still returned.
func (s *CalDAVWriteService) flushPut(ctx context.Context, writer CalDAVWriter, op PendingCalendarObject) error {
	events, err := s.store.EventsByHref(ctx, op.Href)
	if err != nil {
		return fmt.Errorf("read local object: %w", err)
	}
	if len(events) == 0 {
		return nil
	}
	if collapsedSeries(events) {
		return ErrCalDAVCollapsedSeries
	}
	body, err := s.codec.Encode(events, nil)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	ifMatch, ifNoneMatch := op.BaseETag, ""
	if op.Op == CalendarOpCreate {
		ifMatch, ifNoneMatch = "", "*"
	}
	etag, err := writer.PutObject(ctx, op.Href, body, ifMatch, ifNoneMatch)
	if err != nil {
		return err
	}
	var failures []error
	if etag != "" {
		for _, e := range events {
			failures = append(failures, s.store.SaveSyncedEvent(ctx, e, op.Href, etag))
		}
	}
	failures = append(failures, s.store.ClearPendingCalendarOp(ctx, op.CalendarID, op.Href))
	return errors.Join(failures...)
}

// collapsedSeries reports whether an object's rows include an override stored under its master's id (the
// bare UID), the signature of a series whose master an override overwrote. An override keyed by
// domain.EventIDFor never has it; nor does one created locally with a fresh id.
func collapsedSeries(events []domain.Event) bool {
	for _, e := range events {
		if e.IsOverride() && e.UID() != "" && e.ID() == domain.EventIDFor(e.UID(), time.Time{}) {
			return true
		}
	}
	return false
}

// flushDelete removes the object with DELETE If-Match. On success it drops any local remnant and clears the
// intent, returning any failure to do so; on any error, a 412 conflict included, it leaves the intent for
// reconcile or a later retry and returns the error.
func (s *CalDAVWriteService) flushDelete(ctx context.Context, writer CalDAVWriter, op PendingCalendarObject) error {
	if err := writer.DeleteObject(ctx, op.Href, op.BaseETag); err != nil {
		return err
	}
	return errors.Join(
		s.store.DeleteEventsByHref(ctx, op.Href),
		s.store.ClearPendingCalendarOp(ctx, op.CalendarID, op.Href),
	)
}
