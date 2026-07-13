package caldav

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ctagRec records the request line of the last PROPFIND, so a test can assert the method and Depth header.
type ctagRec struct {
	method string
	depth  string
}

// newCTagClient serves a fixed status and body for any request and records the request line.
func newCTagClient(t *testing.T, status int, body string) (*Client, *ctagRec) {
	t.Helper()
	rec := &ctagRec{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec.method = req.Method
		rec.depth = req.Header.Get("Depth")
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	client, err := NewClient(srv.URL, "user", "pass")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, rec
}

func TestCollectionCTagParses(t *testing.T) {
	const body = `<?xml version="1.0" encoding="utf-8"?>` +
		`<multistatus xmlns="DAV:" xmlns:cs="http://calendarserver.org/ns/">` +
		`<response><href>/cal/</href><propstat>` +
		`<prop><cs:getctag>ctag-abc</cs:getctag></prop>` +
		`<status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`
	client, rec := newCTagClient(t, http.StatusMultiStatus, body)
	ctag, err := client.CollectionCTag(context.Background(), "/cal/")
	if err != nil {
		t.Fatalf("CollectionCTag: %v", err)
	}
	if ctag != "ctag-abc" {
		t.Errorf("ctag = %q, want ctag-abc", ctag)
	}
	if rec.method != "PROPFIND" || rec.depth != "0" {
		t.Errorf("request = %s Depth=%q, want a Depth-0 PROPFIND", rec.method, rec.depth)
	}
}

func TestCollectionCTagAbsentIsEmpty(t *testing.T) {
	// A server that does not report getctag yields the empty string with no error, so the caller reconciles
	// the collection unconditionally rather than treating it as an error.
	const body = `<?xml version="1.0"?><multistatus xmlns="DAV:"><response><href>/cal/</href>` +
		`<propstat><prop></prop><status>HTTP/1.1 404 Not Found</status></propstat></response></multistatus>`
	client, _ := newCTagClient(t, http.StatusMultiStatus, body)
	ctag, err := client.CollectionCTag(context.Background(), "/cal/")
	if err != nil {
		t.Fatalf("CollectionCTag: %v", err)
	}
	if ctag != "" {
		t.Errorf("ctag = %q, want empty when the server reports none", ctag)
	}
}

func TestCollectionCTagServerError(t *testing.T) {
	client, _ := newCTagClient(t, http.StatusInternalServerError, "")
	if _, err := client.CollectionCTag(context.Background(), "/cal/"); err == nil {
		t.Fatal("expected an error on a non-2xx status")
	}
}

func TestCollectionCTagBadXML(t *testing.T) {
	client, _ := newCTagClient(t, http.StatusMultiStatus, "<not-xml")
	if _, err := client.CollectionCTag(context.Background(), "/cal/"); err == nil {
		t.Fatal("expected an error on an unparseable body")
	}
}
