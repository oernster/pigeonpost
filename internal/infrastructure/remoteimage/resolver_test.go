package remoteimage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubFetch builds a fetchFunc from a function over the URL, so a test can script success, failure or an
// assertion that a URL is never fetched, without touching the network.
func stubFetch(fn func(url string) ([]byte, string, error)) fetchFunc {
	return func(_ context.Context, url string) ([]byte, string, error) {
		return fn(url)
	}
}

func TestResolveInlinesFetchedImage(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(string) ([]byte, string, error) {
		return []byte{0x01, 0x02, 0x03}, "image/png", nil
	})}
	out, err := r.Resolve(context.Background(), `<img data-pp-src="https://x.test/a.png" alt="a">`)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, `src="data:image/png;base64,AQID"`) {
		t.Errorf("expected the image inlined as a data URI, got: %s", out)
	}
	if strings.Contains(out, blockedImageAttr) {
		t.Errorf("expected the parked attribute gone once resolved, got: %s", out)
	}
	if !strings.Contains(out, `alt="a"`) {
		t.Errorf("expected other attributes preserved, got: %s", out)
	}
}

func TestResolveLeavesFailedFetchParked(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(string) ([]byte, string, error) {
		return nil, "", errors.New("boom")
	})}
	out, err := r.Resolve(context.Background(), `<img data-pp-src="https://x.test/a.png">`)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, `data-pp-src="https://x.test/a.png"`) {
		t.Errorf("expected a failed image to stay parked, got: %s", out)
	}
	if strings.Contains(out, "data:image/") {
		t.Errorf("expected no data URI for a failed fetch, got: %s", out)
	}
}

func TestResolveIgnoresNonHTTPParkedSource(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(url string) ([]byte, string, error) {
		t.Errorf("must not fetch a non-http source, tried %q", url)
		return nil, "", errors.New("must not fetch")
	})}
	out, err := r.Resolve(context.Background(), `<img data-pp-src="cid:logo">`)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, `data-pp-src="cid:logo"`) {
		t.Errorf("expected a non-http parked source left alone, got: %s", out)
	}
}

func TestResolveWithNoBlockedImagesReturnsInputUnchanged(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(url string) ([]byte, string, error) {
		t.Errorf("must not fetch when nothing is parked, tried %q", url)
		return nil, "", errors.New("must not fetch")
	})}
	fragment := `<p>hello <b>world</b></p>`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out != fragment {
		t.Errorf("expected the fragment returned unchanged, got: %s", out)
	}
}

func TestResolvePreservesLeadingStyleBlock(t *testing.T) {
	// A sanitised body can begin with a <style> block. Parsing in a body context (not a whole document) keeps
	// it as a sibling of the content instead of hoisting it into a synthesised head a fragment render drops.
	r := &Resolver{fetch: stubFetch(func(string) ([]byte, string, error) {
		return []byte{0xFF}, "image/gif", nil
	})}
	fragment := `<style>.x{color:red}</style><img data-pp-src="https://x.test/a.png">`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, `<style>.x{color:red}</style>`) {
		t.Errorf("expected the leading style block preserved, got: %s", out)
	}
	if !strings.Contains(out, "data:image/gif;base64,") {
		t.Errorf("expected the image inlined, got: %s", out)
	}
}

func TestResolveInlinesEachImageIndependently(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(url string) ([]byte, string, error) {
		if url == "https://ok.test/a.png" {
			return []byte{0x09}, "image/gif", nil
		}
		return nil, "", errors.New("unreachable")
	})}
	fragment := `<img data-pp-src="https://ok.test/a.png"><img data-pp-src="https://bad.test/b.png">`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, `src="data:image/gif;base64,CQ=="`) {
		t.Errorf("expected the reachable image inlined, got: %s", out)
	}
	if !strings.Contains(out, `data-pp-src="https://bad.test/b.png"`) {
		t.Errorf("expected the unreachable image left parked, got: %s", out)
	}
}
