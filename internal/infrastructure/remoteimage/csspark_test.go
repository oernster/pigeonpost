package remoteimage

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
)

// parked builds the parked form the body parser writes for a remote CSS url, so these tests speak in the same
// wire shape mailparse produces rather than hard-coding an encoding by hand.
func parked(url string) string {
	return "url(" + parkedCSSURLScheme + base64.RawURLEncoding.EncodeToString([]byte(url)) + ")"
}

func TestResolveInlinesParkedBackgroundInStyleAttribute(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(string) ([]byte, string, error) {
		return []byte{0x01, 0x02, 0x03}, "image/jpeg", nil
	})}
	fragment := `<div style="background:` + parked("https://x.test/hero.jpg") + ` top / cover no-repeat #EAF5F5">hi</div>`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, "url(data:image/jpeg;base64,AQID)") {
		t.Errorf("expected the background inlined as a data URI, got: %s", out)
	}
	if strings.Contains(out, parkedCSSURLScheme) {
		t.Errorf("expected the parked reference gone once resolved, got: %s", out)
	}
	// The rest of the shorthand has to survive, since the fallback colour is what the text sits on if the
	// image ever fails.
	if !strings.Contains(out, "#EAF5F5") {
		t.Errorf("expected the rest of the declaration preserved, got: %s", out)
	}
}

func TestResolveInlinesParkedBackgroundInStyleElement(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(string) ([]byte, string, error) {
		return []byte{0x01}, "image/png", nil
	})}
	fragment := `<style>.hero{background-image:` + parked("https://x.test/a.png") + `}</style><div class="hero">x</div>`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, "url(data:image/png;base64,AQ==)") {
		t.Errorf("expected the <style> background inlined, got: %s", out)
	}
}

func TestResolveLeavesFailedBackgroundParked(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(string) ([]byte, string, error) {
		return nil, "", errors.New("boom")
	})}
	fragment := `<div style="background-image:` + parked("https://x.test/a.png") + `">x</div>`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Left parked rather than emptied: the reference still paints nothing, and asking again can succeed.
	if !strings.Contains(out, parkedCSSURLScheme) {
		t.Errorf("expected a failed background to stay parked, got: %s", out)
	}
	if strings.Contains(out, "data:image/") {
		t.Errorf("expected no data URI for a failed fetch, got: %s", out)
	}
}

func TestResolveFetchesEachBackgroundOnce(t *testing.T) {
	// An email routinely repeats one hero image across breakpoints and on the element itself, so the same URL
	// appears many times. Fetching it once per occurrence would multiply the outbound requests for nothing.
	var mu sync.Mutex
	calls := map[string]int{}
	r := &Resolver{fetch: stubFetch(func(url string) ([]byte, string, error) {
		mu.Lock()
		calls[url]++
		mu.Unlock()
		return []byte{0x01}, "image/png", nil
	})}
	ref := parked("https://x.test/hero.jpg")
	fragment := `<style>@media(max-width:650px){.h{background-image:` + ref + `}}` +
		`@media(max-width:520px){.h{background-image:` + ref + `}}</style>` +
		`<div class="h" style="background-image:` + ref + `">x</div>`
	if _, err := r.Resolve(context.Background(), fragment); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if calls["https://x.test/hero.jpg"] != 1 {
		t.Errorf("expected one fetch for the repeated background, got %d", calls["https://x.test/hero.jpg"])
	}
}

func TestResolveIgnoresParkedTargetThatIsNotRemote(t *testing.T) {
	// Only http and https are ever fetched. A parked value that decodes to anything else, or does not decode
	// at all, is left exactly as it is rather than guessed at.
	r := &Resolver{fetch: stubFetch(func(url string) ([]byte, string, error) {
		t.Errorf("nothing should be fetched, got a request for %s", url)
		return nil, "", errors.New("unexpected")
	})}
	fragment := `<div style="background-image:` + parked("file:///etc/passwd") +
		`;border-image:url(` + parkedCSSURLScheme + `!!!notbase64)">x</div>`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, parkedCSSURLScheme) {
		t.Errorf("expected the unfetchable references left alone, got: %s", out)
	}
}

func TestResolveLeavesBodyWithNoParkedResourcesUntouched(t *testing.T) {
	r := &Resolver{fetch: stubFetch(func(url string) ([]byte, string, error) {
		t.Errorf("nothing should be fetched, got a request for %s", url)
		return nil, "", errors.New("unexpected")
	})}
	const fragment = `<div style="background:#ffffff">plain</div>`
	out, err := r.Resolve(context.Background(), fragment)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out != fragment {
		t.Errorf("expected the body returned unchanged, got: %s", out)
	}
}
