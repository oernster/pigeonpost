package remoteimage

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// The fetch limits are all named so the SSRF and resource-abuse guards read as policy rather than bare numbers.
const (
	// maxImageBytes caps a fetched image at 10 MiB, read through an io.LimitReader so a hostile or broken
	// server cannot stream an unbounded body into memory.
	maxImageBytes = 10 << 20
	// requestTimeout bounds the whole fetch (connect, redirects and body read) so a slow server cannot hold a
	// fetch open indefinitely.
	requestTimeout = 15 * time.Second
	// dialTimeout bounds a single TCP connect attempt.
	dialTimeout = 10 * time.Second
	// maxRedirects caps how many redirects a fetch follows; each hop is re-dialled through the same IP guard,
	// so a redirect cannot escape it.
	maxRedirects = 3
	// imageContentTypePrefix is required at the start of a response Content-Type, so only an image is inlined.
	imageContentTypePrefix = "image/"
	// The 100.64.0.0/10 carrier-grade NAT range has no net.IP predicate: its first octet is 100 and the top
	// two bits of its second octet are 01. These name that test.
	cgnatFirstOctet       = 100
	cgnatSecondOctetMask  = 0xC0
	cgnatSecondOctetValue = 0x40
)

// newSafeFetch builds the real fetch: an HTTP client hardened against server-side request forgery, using the
// production IP guard that blocks every non-public address.
func newSafeFetch() fetchFunc {
	return newGuardedFetch(isBlockedIP)
}

// newGuardedFetch builds a fetch whose dialer rejects any connection the isBlocked predicate refuses. The
// predicate is a parameter so a test can permit loopback and exercise the client against an httptest server;
// production passes isBlockedIP. The client blocks on the actual connect IP (after DNS, so DNS rebinding
// cannot slip past), follows only a capped number of redirects (each re-dialled through the same guard), reads
// at most maxImageBytes and accepts only a 200 response whose Content-Type is an image.
func newGuardedFetch(isBlocked func(net.IP) bool) fetchFunc {
	dialer := &net.Dialer{
		Timeout: dialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			return guardDial(address, isBlocked)
		},
	}
	client := &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			DialContext:       dialer.DialContext,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("remoteimage: stopped after %d redirects", maxRedirects)
			}
			if !isHTTPURL(req.URL.String()) {
				return fmt.Errorf("remoteimage: redirect to non-http(s) scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
	return func(ctx context.Context, rawURL string) ([]byte, string, error) {
		return fetchImage(ctx, client, rawURL)
	}
}

// fetchImage performs one guarded image GET and returns the bytes and Content-Type. Every failure mode (bad
// scheme, blocked address, non-200, non-image, oversize) is an error, so the caller leaves the image parked on
// any of them.
func fetchImage(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	if !isHTTPURL(rawURL) {
		return nil, "", fmt.Errorf("remoteimage: refusing non-http(s) URL %q", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("remoteimage: build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("remoteimage: fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("remoteimage: status %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), imageContentTypePrefix) {
		return nil, "", fmt.Errorf("remoteimage: non-image content type %q", contentType)
	}
	data, err := readCapped(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, contentType, nil
}

// readCapped reads up to maxImageBytes from r and errors if the body is larger. It reads one byte past the cap
// so an exactly-cap-sized body is accepted while an oversize one is rejected rather than silently truncated.
func readCapped(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("remoteimage: read body: %w", err)
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("remoteimage: image exceeds %d bytes", maxImageBytes)
	}
	return data, nil
}

// guardDial runs after DNS resolution with the concrete address the dialer is about to connect to, and rejects
// it unless isBlocked passes the IP. Checking the connect IP (not the hostname) is what defeats DNS rebinding:
// a name that resolved to a public IP for the guard check but to a loopback address for the connect would
// still be blocked here.
func guardDial(address string, isBlocked func(net.IP) bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("remoteimage: split connect address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("remoteimage: unparseable connect address %q", host)
	}
	if isBlocked(ip) {
		return fmt.Errorf("remoteimage: refusing to connect to non-public address %s", ip)
	}
	return nil
}

// isBlockedIP reports whether an IP must not be connected to: anything that is not a public unicast address. It
// blocks loopback, private (RFC 1918 and IPv6 unique-local), link-local, multicast, unspecified and
// carrier-grade-NAT (100.64.0.0/10) addresses.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	return isCGNAT(ip)
}

// isCGNAT reports whether ip is in the 100.64.0.0/10 carrier-grade NAT range, which net.IP has no predicate
// for. Such addresses are not globally routable, so they are treated like private space. To4 unwraps an
// IPv4-mapped IPv6 address, so a mapped CGNAT address is caught too.
func isCGNAT(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == cgnatFirstOctet && ip4[1]&cgnatSecondOctetMask == cgnatSecondOctetValue
}

// isHTTPURL reports whether rawURL is a well-formed absolute http or https URL with a host. Only those two
// schemes are fetched, so a data:, cid:, file: or other scheme is never followed.
func isHTTPURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
