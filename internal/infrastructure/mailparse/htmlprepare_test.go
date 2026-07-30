package mailparse

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestHTMLToTextDropsScriptAndBreaks(t *testing.T) {
	out := htmlToText("<p>A</p><script>evil()</script><p>B<br>C</p>")
	if strings.Contains(out, "evil") {
		t.Errorf("script content leaked into %q", out)
	}
	for _, want := range []string{"A", "B", "C"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}

func TestParseBodyRemovesHiddenPreheader(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<div style="display:none;font-size:0;max-height:0;">Hidden preheader duplicate</div>` +
		`<span style="opacity:0 !important">Zero opacity teaser</span>` +
		`<div style="height:0;overflow:hidden">Zero height preheader</div>` +
		`<span hidden>Hidden attribute snippet</span>` +
		`<span style="font-size:0">Zero font leaf teaser</span>` +
		`<h1 style="opacity:0.9">Visible headline</h1>` +
		`<p style="line-height:0">Line height kept</p>` +
		`<p style="font-size:0.9em">Visible body text</p>` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	for _, gone := range []string{"Hidden preheader duplicate", "Zero opacity teaser", "Zero height preheader", "Hidden attribute snippet", "Zero font leaf teaser"} {
		if strings.Contains(html, gone) {
			t.Errorf("sender-hidden node should be removed, still present %q: %s", gone, html)
		}
	}
	// line-height:0 does not hide content, so it must not be mistaken for a preheader marker.
	for _, kept := range []string{"Visible headline", "Line height kept", "Visible body text"} {
		if !strings.Contains(html, kept) {
			t.Errorf("visible content should survive, missing %q: %s", kept, html)
		}
	}
}

func TestParseBodyKeepsFontSizeZeroLayoutWrapper(t *testing.T) {
	// Email frameworks such as MJML nest the whole body inside font-size:0 wrapper cells to collapse the
	// whitespace between inline-block columns; the real text inside re-sets its own size. Every such wrapper
	// has element children, so none may be mistaken for a hidden preheader or the entire visible body is
	// deleted, the blank render seen for MJML-built transactional mail such as the Claude sign-in email.
	// This body mirrors that message's structure (token-free): heading, sub-line, button and footer, each
	// wrapped in a font-size:0 cell whose child re-sets the size.
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<body style="background-color:#FAF9F5;"><div style="margin:0px auto;max-width:600px;">` +
		`<table role="presentation" style="width:100%;"><tbody><tr>` +
		`<td style="direction:ltr;font-size:0px;padding:20px 32px;text-align:center;">` +
		`<div class="auth-centered-content" style="font-size:0px;display:inline-block;width:100%;text-align:left;">` +
		`<table role="presentation" width="100%"><tbody>` +
		`<tr><td align="left" style="font-size:0px;padding:10px 25px;word-break:break-word;">` +
		`<div style="font-family:Helvetica,sans-serif;font-size:32px;color:#0F0F0D;">Lets get you signed in</div></td></tr>` +
		`<tr><td align="left" style="font-size:0px;padding:10px 25px;word-break:break-word;">` +
		`<div style="font-family:Helvetica,sans-serif;font-size:18px;color:#3D3929;">Sign in with the secure link below</div></td></tr>` +
		`<tr><td align="center" style="font-size:0px;padding:10px 25px;word-break:break-word;">` +
		`<p style="display:inline-block;background:#000000;color:#ffffff;">` +
		`<a href="https://example.com/magic-link" style="color:white;font-size:18px;">Sign in to Claude.ai</a></p></td></tr>` +
		`<tr><td align="left" style="font-size:0px;padding:0px 32px;word-break:break-word;">` +
		`<div style="font-family:Arial;font-size:14px;color:#737163;">If you did not request this email, you can safely ignore it.</div></td></tr>` +
		`</tbody></table></div></td></tr></tbody></table></div></body>` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	for _, kept := range []string{
		"get you signed in", "Sign in with the secure link below",
		"Sign in to Claude.ai", "you can safely ignore it",
	} {
		if !strings.Contains(parsed.HTML, kept) {
			t.Errorf("font-size:0 wrapper content should survive, missing %q: %s", kept, parsed.HTML)
		}
	}
}

func TestParseBodyKeepsMsoHideOnlyContent(t *testing.T) {
	// mso-hide:all hides content in Outlook only; every other client is meant to show it. An element that
	// carries only mso-hide:all (a fallback block for non-Outlook clients) must therefore survive.
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<div style="mso-hide:all">Shown everywhere except Outlook</div>` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if !strings.Contains(parsed.HTML, "Shown everywhere except Outlook") {
		t.Errorf("mso-hide:all content should survive, got: %s", parsed.HTML)
	}
}

func TestPrepareHTMLParksPictureSource(t *testing.T) {
	out := prepareHTML(`<picture><source srcset="http://tracker.example/2x.webp">`+
		`<img src="http://tracker.example/pixel.gif"></picture>`, nil)
	if strings.Contains(strings.ToLower(out), "srcset") {
		t.Errorf("a <source> srcset should be dropped, got: %s", out)
	}
	if strings.Contains(out, ` src="http`) {
		t.Errorf("no element should keep a live remote src, got: %s", out)
	}
	if !strings.Contains(out, `data-pp-src="http://tracker.example/pixel.gif"`) {
		t.Errorf("the <img> source should be parked, got: %s", out)
	}
}

// parkedCSSTarget decodes the target of the first parked CSS reference in some prepared HTML, so a test can
// check both that nothing is fetchable and that the original URL survived for the reader to ask for later.
func parkedCSSTarget(t *testing.T, prepared string) string {
	t.Helper()
	const open = "url(" + parkedCSSURLScheme
	start := strings.Index(prepared, open)
	if start < 0 {
		t.Fatalf("expected a parked CSS reference in: %s", prepared)
	}
	rest := prepared[start+len(open):]
	end := strings.Index(rest, ")")
	if end < 0 {
		t.Fatalf("parked CSS reference was not closed in: %s", prepared)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(rest[:end])
	if err != nil {
		t.Fatalf("parked CSS target did not decode: %v", err)
	}
	return string(decoded)
}

// assertNoLiveRemoteURL checks that nothing in the prepared HTML is a reference a browser would fetch. The
// parked form is deliberately not a fetchable URL: the scheme has no handler, so it never leaves the machine.
func assertNoLiveRemoteURL(t *testing.T, prepared string) {
	t.Helper()
	for _, live := range []string{"url(http", "url('http", `url("http`} {
		if strings.Contains(prepared, live) {
			t.Errorf("a remote CSS url must not stay live, got: %s", prepared)
		}
	}
}

func TestPrepareHTMLParksRemoteCSSBackgroundInStyleAttr(t *testing.T) {
	out := prepareHTML(`<div style="color:red;background:url('http://tracker.example/bg.png')">hi</div>`, nil)
	assertNoLiveRemoteURL(t, out)
	// The target is kept, parked, rather than discarded. Discarding it stranded any text the sender coloured
	// for the image: with the background gone for good, a heading set white to sit on a dark photo fell back
	// to the sender's pale colour and could never be recovered by loading images.
	if got := parkedCSSTarget(t, out); got != "http://tracker.example/bg.png" {
		t.Errorf("the parked target should decode to the original url, got: %s", got)
	}
	if !strings.Contains(out, "color:red") {
		t.Errorf("an unrelated style declaration should be preserved, got: %s", out)
	}
}

func TestPrepareHTMLParksRemoteURLInStyleElement(t *testing.T) {
	out := prepareHTML(`<style>.hero{background:url(https://tracker.example/hero.jpg)}</style>`, nil)
	assertNoLiveRemoteURL(t, out)
	if got := parkedCSSTarget(t, out); got != "https://tracker.example/hero.jpg" {
		t.Errorf("the parked target should decode to the original url, got: %s", got)
	}
}

func TestPrepareHTMLEmptiesFontSourceRatherThanParkingIt(t *testing.T) {
	// Both senders in the reported cases carry @font-face blocks. Parking those would mean a press of Load
	// images fires requests at the sender's CDN for resources the image proxy rejects on content type anyway,
	// so a font source is emptied exactly as before and only image references become askable.
	out := prepareHTML(`<style>@font-face{font-family:'X';`+
		`src:local('X'), url('https://cdn.example/x.woff2') format('woff2')}`+
		`.hero{background-image:url('https://cdn.example/hero.jpg')}</style>`, nil)
	assertNoLiveRemoteURL(t, out)
	if strings.Contains(out, "x.woff2") {
		t.Errorf("a font source should not survive in readable form, got: %s", out)
	}
	if got := parkedCSSTarget(t, out); got != "https://cdn.example/hero.jpg" {
		t.Errorf("the background should be the parked reference, got: %s", got)
	}
	if n := strings.Count(out, parkedCSSURLScheme); n != 1 {
		t.Errorf("expected exactly the background parked, got %d parked references: %s", n, out)
	}
}

func TestPrepareHTMLParksURLWithQueryStringThroughSanitising(t *testing.T) {
	// The sanitiser's CSS value check rejects characters no ordinary value uses, "?" and "&" among them, and
	// drops the whole declaration when it sees one. Encoding the parked target keeps it inside the accepted
	// set, so a background URL carrying a query string cannot take its own background-colour down with it.
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<div style="background-color:#123456;background-image:url('https://cdn.example/a.jpg?t=1&v=2')">hi</div>` + "\r\n"
	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if !strings.Contains(parsed.HTML, "#123456") {
		t.Errorf("the background colour should survive sanitising, got: %s", parsed.HTML)
	}
	if !strings.Contains(parsed.HTML, parkedCSSURLScheme) {
		t.Errorf("the parked background should survive sanitising, got: %s", parsed.HTML)
	}
	assertNoLiveRemoteURL(t, parsed.HTML)
}

func TestPrepareHTMLKeepsEmbeddedDataURI(t *testing.T) {
	const dataURI = "data:image/png;base64,iVBORw0KGgo="
	out := prepareHTML(`<div style="background:url(`+dataURI+`)">x</div>`, nil)
	if !strings.Contains(out, dataURI) {
		t.Errorf("an embedded data URI should be kept, got: %s", out)
	}
}

func TestParseBodyBlocksRemoteImages(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<p>Hello</p><img src="http://tracker.example/pixel.gif" srcset="http://tracker.example/2x.gif 2x" alt="pic">` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	// The original source is parked in the data attribute, not left where the browser would fetch it.
	if !strings.Contains(html, `data-pp-src="http://tracker.example/pixel.gif"`) {
		t.Errorf("expected image source parked in data-pp-src, got: %s", html)
	}
	// A genuine (space-delimited) src attribute must be gone; the data-pp-src attribute is expected.
	if strings.Contains(html, ` src="http`) || strings.Contains(html, `<img src=`) {
		t.Errorf("remote image src should not auto-load, got: %s", html)
	}
	if strings.Contains(strings.ToLower(html), "srcset") {
		t.Errorf("srcset should be dropped, got: %s", html)
	}
	// The alt text and surrounding content survive.
	if !strings.Contains(html, "Hello") || !strings.Contains(html, `alt="pic"`) {
		t.Errorf("expected alt and content preserved, got: %s", html)
	}
}

func TestPrepareHTMLResolvesCidImageToDataURI(t *testing.T) {
	inline := map[string]inlineImage{"logo": {mediaType: "image/png", content: []byte{0x89, 0x50, 0x4e, 0x47}}}
	out := prepareHTML(`<img src="cid:logo" alt="logo">`, inline)
	if !strings.Contains(out, "data:image/png;base64,") {
		t.Errorf("a resolvable cid image should be inlined as a data URI, got: %s", out)
	}
	if strings.Contains(out, "cid:logo") || strings.Contains(out, blockedImageAttr) {
		t.Errorf("a resolved cid image should be neither a cid reference nor parked, got: %s", out)
	}
}

func TestPrepareHTMLLeavesEmbeddedDataURIImageUnparked(t *testing.T) {
	const dataURI = "data:image/png;base64,iVBORw0KGgo="
	out := prepareHTML(`<img src="`+dataURI+`" alt="x">`, nil)
	if !strings.Contains(out, `src="`+dataURI+`"`) {
		t.Errorf("an embedded data: image should stay loadable, got: %s", out)
	}
	if strings.Contains(out, blockedImageAttr) {
		t.Errorf("an embedded data: image should not be parked, got: %s", out)
	}
}

func TestPrepareHTMLLeavesUnresolvedCidUnparked(t *testing.T) {
	out := prepareHTML(`<img src="cid:missing" alt="x">`, nil)
	if strings.Contains(out, blockedImageAttr) {
		t.Errorf("an unresolved cid image should not be parked as remote, got: %s", out)
	}
	if !strings.Contains(out, "cid:missing") {
		t.Errorf("an unresolved cid image should be left in place, got: %s", out)
	}
}

func TestParseBodyResolvesInlineCidImage(t *testing.T) {
	// A 1x1 transparent GIF carried inline and referenced by the HTML through its Content-ID.
	const gifBase64 = "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/related; boundary=b\r\n" +
		"\r\n" +
		"--b\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<p>Logo:</p><img src="cid:logo" alt="logo">` + "\r\n" +
		"--b\r\n" +
		"Content-Type: image/gif\r\n" +
		"Content-Disposition: inline\r\n" +
		"Content-Id: <logo>\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		gifBase64 + "\r\n" +
		"--b--\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	// The embedded image is inlined as a data: URI and survives sanitising, so it renders with no fetch.
	if !strings.Contains(html, "data:image/gif;base64,"+gifBase64) {
		t.Errorf("expected the cid image inlined as its data URI, got: %s", html)
	}
	// It is neither left as an unloadable cid: reference nor parked as if it were a remote tracker.
	if strings.Contains(html, "cid:logo") {
		t.Errorf("the embedded image should not stay a cid: reference, got: %s", html)
	}
	if strings.Contains(html, blockedImageAttr) {
		t.Errorf("a message whose only image is embedded should have nothing parked, got: %s", html)
	}
	// The alt text survives.
	if !strings.Contains(html, `alt="logo"`) {
		t.Errorf("expected alt text preserved, got: %s", html)
	}
}

func TestParseBodyKeepsBackgroundColourButDropsRemoteImageURL(t *testing.T) {
	// The relaxed sanitiser keeps background styling, but the remote-url stripping in prepareHTML must still
	// neutralise a CSS url() tracker end to end: the colour survives, the tracker host does not.
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<div style="background-color:#123456;background-image:url('http://tracker.example/bg.png')">hi</div>` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	if !strings.Contains(html, "#123456") {
		t.Errorf("background colour should survive: %s", html)
	}
	if strings.Contains(html, "tracker.example") {
		t.Errorf("remote CSS url should not survive sanitising: %s", html)
	}
}
