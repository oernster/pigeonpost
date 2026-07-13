package application

import (
	"context"
	"fmt"
)

// CalDAVWriteService is the write side of two-way CalDAV sync. Flush pushes pending local changes to the
// server with conditional writes: a create with If-None-Match:* so it never overwrites, an update with
// If-Match against the base etag, a delete with DELETE If-Match. It mirrors the tag two-way sync's
// FlushPending: it is best-effort, so one object's failure never stops the others, and it does not resolve a
// conflict itself. A 412, or any other error, leaves the pending intent in place for a later Reconcile to
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

// Flush pushes every pending write intent to the server through writer. The only fatal error is failing to
// read the pending list; a per-object failure is swallowed so a single bad object does not strand the rest.
func (s *CalDAVWriteService) Flush(ctx context.Context, writer CalDAVWriter) error {
	ops, err := s.store.ListPendingCalendarOps(ctx)
	if err != nil {
		return fmt.Errorf("caldav: list pending calendar ops: %w", err)
	}
	for _, op := range ops {
		if op.Op == CalendarOpDelete {
			s.flushDelete(ctx, writer, op)
			continue
		}
		s.flushPut(ctx, writer, op)
	}
	return nil
}

// flushPut re-encodes the object's events and writes them: a create guards on If-None-Match:*, an update
// guards on If-Match against the base etag. On a confirmed write it stamps the returned etag onto every event
// of the object and clears the intent; on any error it leaves the intent for reconcile or a later retry. An
// object whose events have vanished locally is skipped, leaving the intent for reconcile to clean up.
func (s *CalDAVWriteService) flushPut(ctx context.Context, writer CalDAVWriter, op PendingCalendarObject) {
	events, err := s.store.EventsByHref(ctx, op.Href)
	if err != nil || len(events) == 0 {
		return
	}
	body, err := s.codec.Encode(events, nil)
	if err != nil {
		return
	}
	ifMatch, ifNoneMatch := op.BaseETag, ""
	if op.Op == CalendarOpCreate {
		ifMatch, ifNoneMatch = "", "*"
	}
	etag, err := writer.PutObject(ctx, op.Href, body, ifMatch, ifNoneMatch)
	if err != nil {
		return
	}
	if etag != "" {
		for _, e := range events {
			_ = s.store.SaveSyncedEvent(ctx, e, op.Href, etag)
		}
	}
	_ = s.store.ClearPendingCalendarOp(ctx, op.CalendarID, op.Href)
}

// flushDelete removes the object with DELETE If-Match. On success it drops any local remnant and clears the
// intent; on any error, a 412 conflict included, it leaves the intent for reconcile or a later retry.
func (s *CalDAVWriteService) flushDelete(ctx context.Context, writer CalDAVWriter, op PendingCalendarObject) {
	if err := writer.DeleteObject(ctx, op.Href, op.BaseETag); err != nil {
		return
	}
	_ = s.store.DeleteEventsByHref(ctx, op.Href)
	_ = s.store.ClearPendingCalendarOp(ctx, op.CalendarID, op.Href)
}
