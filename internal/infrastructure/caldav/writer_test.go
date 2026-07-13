package caldav

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
)

// recordingDAV is an httptest handler that records the last write request and returns a configured status and
// etag, so the raw conditional PUT/DELETE logic (headers, status mapping, etag quoting) is exercised without a
// full DAV server.
type recordingDAV struct {
	method      string
	path        string
	ifMatch     string
	ifNoneMatch string
	body        string
	status      int
	etag        string
}

func (r *recordingDAV) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.method = req.Method
		r.path = req.URL.Path
		r.ifMatch = req.Header.Get("If-Match")
		r.ifNoneMatch = req.Header.Get("If-None-Match")
		b, _ := io.ReadAll(req.Body)
		r.body = string(b)
		if r.etag != "" {
			w.Header().Set("ETag", r.etag)
		}
		w.WriteHeader(r.status)
	}
}

func newWriter(t *testing.T, rec *recordingDAV) *Client {
	t.Helper()
	srv := httptest.NewServer(rec.handler())
	t.Cleanup(srv.Close)
	client, err := NewClient(srv.URL, "user", "pass")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestPutObjectCreate(t *testing.T) {
	rec := &recordingDAV{status: http.StatusCreated, etag: `"new-etag"`}
	client := newWriter(t, rec)
	etag, err := client.PutObject(context.Background(), "/cal/obj.ics", []byte("BEGIN:VCALENDAR"), "", "*")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if etag != "new-etag" {
		t.Errorf("etag = %q, want new-etag", etag)
	}
	if rec.method != http.MethodPut || rec.path != "/cal/obj.ics" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.ifNoneMatch != "*" || rec.ifMatch != "" {
		t.Errorf("create headers: If-None-Match=%q If-Match=%q", rec.ifNoneMatch, rec.ifMatch)
	}
	if rec.body != "BEGIN:VCALENDAR" {
		t.Errorf("body = %q", rec.body)
	}
}

func TestPutObjectUpdateQuotesIfMatch(t *testing.T) {
	rec := &recordingDAV{status: http.StatusNoContent, etag: `"newer"`}
	client := newWriter(t, rec)
	etag, err := client.PutObject(context.Background(), "/cal/obj.ics", []byte("X"), "old", "")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if etag != "newer" {
		t.Errorf("etag = %q, want newer", etag)
	}
	if rec.ifMatch != `"old"` {
		t.Errorf("If-Match = %q, want quoted old", rec.ifMatch)
	}
}

func TestPutObjectConflict(t *testing.T) {
	rec := &recordingDAV{status: http.StatusPreconditionFailed}
	client := newWriter(t, rec)
	if _, err := client.PutObject(context.Background(), "/cal/obj.ics", []byte("X"), "old", ""); !errors.Is(err, application.ErrCalDAVConflict) {
		t.Fatalf("err = %v, want ErrCalDAVConflict", err)
	}
}

func TestPutObjectServerError(t *testing.T) {
	rec := &recordingDAV{status: http.StatusInternalServerError}
	client := newWriter(t, rec)
	_, err := client.PutObject(context.Background(), "/cal/obj.ics", []byte("X"), "", "*")
	if err == nil || errors.Is(err, application.ErrCalDAVConflict) {
		t.Fatalf("err = %v, want a non-conflict error", err)
	}
}

func TestPutObjectWeakAndUnquotedEtags(t *testing.T) {
	// A weak or already-quoted If-Match passes through unchanged; a weak response etag is unwrapped for storage.
	rec := &recordingDAV{status: http.StatusNoContent, etag: `W/"resp-weak"`}
	client := newWriter(t, rec)
	etag, err := client.PutObject(context.Background(), "/cal/obj.ics", []byte("X"), `W/"weak"`, "")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if rec.ifMatch != `W/"weak"` {
		t.Errorf("weak If-Match should pass through: %q", rec.ifMatch)
	}
	if etag != "resp-weak" {
		t.Errorf("unquote weak etag = %q, want resp-weak", etag)
	}

	rec2 := &recordingDAV{status: http.StatusNoContent}
	client2 := newWriter(t, rec2)
	if _, err := client2.PutObject(context.Background(), "/cal/obj.ics", []byte("X"), `"already"`, ""); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if rec2.ifMatch != `"already"` {
		t.Errorf("already-quoted If-Match should pass through: %q", rec2.ifMatch)
	}
}

func TestDeleteObject(t *testing.T) {
	rec := &recordingDAV{status: http.StatusNoContent}
	client := newWriter(t, rec)
	if err := client.DeleteObject(context.Background(), "/cal/obj.ics", "e1"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if rec.method != http.MethodDelete || rec.path != "/cal/obj.ics" || rec.ifMatch != `"e1"` {
		t.Errorf("request = %s %s If-Match=%q", rec.method, rec.path, rec.ifMatch)
	}
}

func TestDeleteObjectGoneIsSuccess(t *testing.T) {
	rec := &recordingDAV{status: http.StatusNotFound}
	client := newWriter(t, rec)
	if err := client.DeleteObject(context.Background(), "/cal/obj.ics", "e1"); err != nil {
		t.Fatalf("a 404 delete should be success: %v", err)
	}
}

func TestDeleteObjectConflict(t *testing.T) {
	rec := &recordingDAV{status: http.StatusPreconditionFailed}
	client := newWriter(t, rec)
	if err := client.DeleteObject(context.Background(), "/cal/obj.ics", "e1"); !errors.Is(err, application.ErrCalDAVConflict) {
		t.Fatalf("err = %v, want ErrCalDAVConflict", err)
	}
}

func TestDeleteObjectServerError(t *testing.T) {
	rec := &recordingDAV{status: http.StatusInternalServerError}
	client := newWriter(t, rec)
	if err := client.DeleteObject(context.Background(), "/cal/obj.ics", "e1"); err == nil || errors.Is(err, application.ErrCalDAVConflict) {
		t.Fatalf("err = %v, want a non-conflict error", err)
	}
}
