package remoteimage

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},             // loopback
		{"::1", true},                   // loopback (IPv6)
		{"10.0.0.1", true},              // private
		{"192.168.1.1", true},           // private
		{"172.16.5.4", true},            // private
		{"fc00::1", true},               // unique-local (IPv6 private)
		{"169.254.1.1", true},           // link-local
		{"fe80::1", true},               // link-local (IPv6)
		{"224.0.0.1", true},             // multicast
		{"ff02::1", true},               // multicast (IPv6)
		{"0.0.0.0", true},               // unspecified
		{"::", true},                    // unspecified (IPv6)
		{"100.64.0.0", true},            // carrier-grade NAT (low edge)
		{"100.127.255.255", true},       // carrier-grade NAT (high edge)
		{"8.8.8.8", false},              // public
		{"1.1.1.1", false},              // public
		{"2606:4700:4700::1111", false}, // public (IPv6)
		{"100.63.255.255", false},       // just below the CGNAT range
		{"100.128.0.1", false},          // just above the CGNAT range
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := isBlockedIP(ip); got != c.blocked {
			t.Errorf("isBlockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
}

func TestIsHTTPURL(t *testing.T) {
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://x.test/a.png", true},
		{"http://x.test", true},
		{"cid:logo", false},
		{"ftp://x.test/a", false},
		{"data:image/png;base64,AA==", false},
		{"", false},
		{"/relative/path", false},
		{"https://", false},
	}
	for _, c := range cases {
		if got := isHTTPURL(c.url); got != c.ok {
			t.Errorf("isHTTPURL(%q) = %v, want %v", c.url, got, c.ok)
		}
	}
}

// countingReader yields count zero bytes then EOF without allocating them up front, so the size cap can be
// exercised at its boundary cheaply.
type countingReader struct{ remaining int }

func (c *countingReader) Read(p []byte) (int, error) {
	if c.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > c.remaining {
		n = c.remaining
	}
	c.remaining -= n
	return n, nil
}

func TestReadCappedAcceptsExactlyCap(t *testing.T) {
	data, err := readCapped(&countingReader{remaining: maxImageBytes})
	if err != nil {
		t.Fatalf("readCapped at the cap: %v", err)
	}
	if len(data) != maxImageBytes {
		t.Errorf("read %d bytes, want %d", len(data), maxImageBytes)
	}
}

func TestReadCappedRejectsOversize(t *testing.T) {
	if _, err := readCapped(&countingReader{remaining: maxImageBytes + 1}); err == nil {
		t.Fatal("expected an oversize body to be rejected")
	}
}

// permissiveFetch builds a fetch whose IP guard blocks nothing, so the other guards (content type, status,
// redirects, scheme) can be exercised against a loopback httptest server the production guard would refuse.
func permissiveFetch() fetchFunc {
	return newGuardedFetch(func(net.IP) bool { return false })
}

func TestFetchImageSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x01, 0x02})
	}))
	defer srv.Close()
	data, contentType, err := permissiveFetch()(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if contentType != "image/png" || len(data) != 2 {
		t.Errorf("got content type %q and %d bytes, want image/png and 2 bytes", contentType, len(data))
	}
}

func TestFetchRejectsNonImageContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>"))
	}))
	defer srv.Close()
	if _, _, err := permissiveFetch()(context.Background(), srv.URL); err == nil {
		t.Fatal("expected a non-image content type to be rejected")
	}
}

func TestFetchRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if _, _, err := permissiveFetch()(context.Background(), srv.URL); err == nil {
		t.Fatal("expected a non-200 status to be rejected")
	}
}

func TestFetchStopsAfterTooManyRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()
	if _, _, err := permissiveFetch()(context.Background(), srv.URL); err == nil {
		t.Fatal("expected a redirect loop to be stopped")
	}
}

func TestFetchRejectsNonHTTPScheme(t *testing.T) {
	if _, _, err := permissiveFetch()(context.Background(), "ftp://x.test/a.png"); err == nil {
		t.Fatal("expected a non-http scheme to be rejected")
	}
}

func TestSafeFetchBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x01})
	}))
	defer srv.Close()
	// The production guard must refuse to connect to the loopback address the test server listens on, which is
	// the SSRF defence: a message image URL cannot be used to reach a service on the host or the local network.
	if _, _, err := newSafeFetch()(context.Background(), srv.URL); err == nil {
		t.Fatal("expected the SSRF guard to block a loopback address")
	}
}
