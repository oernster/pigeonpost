package mailparse

import (
	"strings"
	"testing"
)

func TestParseBodyMultipartAlternative(t *testing.T) {
	raw := "From: a@b.com\r\n" +
		"To: c@d.com\r\n" +
		"Subject: Test\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"bd\"\r\n" +
		"\r\n" +
		"--bd\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Hello plain\r\n" +
		"--bd\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>Hello <b>html</b></p>\r\n" +
		"--bd--\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	plain, html := parsed.Plain, parsed.HTML
	if !strings.Contains(plain, "Hello plain") {
		t.Errorf("plain = %q, want it to contain Hello plain", plain)
	}
	if !strings.Contains(html, "<b>html</b>") {
		t.Errorf("html = %q, want it to contain the html part", html)
	}
}

func TestParseBodyExtractsAttachments(t *testing.T) {
	raw := "From: a@b.com\r\n" +
		"To: c@d.com\r\n" +
		"Subject: Test\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"bd\"\r\n" +
		"\r\n" +
		"--bd\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"See the file\r\n" +
		"--bd\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Disposition: attachment; filename=\"notes.txt\"\r\n" +
		"\r\n" +
		"file bytes\r\n" +
		"--bd\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment\r\n" +
		"\r\n" +
		"nameless\r\n" +
		"--bd--\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if !strings.Contains(parsed.Plain, "See the file") {
		t.Errorf("plain = %q, want the readable text", parsed.Plain)
	}
	if len(parsed.Attachments) != 2 {
		t.Fatalf("got %d attachments, want 2", len(parsed.Attachments))
	}
	if parsed.Attachments[0].Filename != "notes.txt" || !strings.Contains(string(parsed.Attachments[0].Content), "file bytes") {
		t.Errorf("first attachment = %+v", parsed.Attachments[0])
	}
	// A nameless attachment gets a fallback filename so it can still be saved.
	if parsed.Attachments[1].Filename != fallbackAttachmentName {
		t.Errorf("nameless attachment filename = %q, want fallback", parsed.Attachments[1].Filename)
	}

	converted, err := DomainAttachments(parsed.Attachments)
	if err != nil {
		t.Fatalf("DomainAttachments: %v", err)
	}
	if len(converted) != 2 || converted[0].Filename() != "notes.txt" {
		t.Errorf("converted = %+v", converted)
	}
}

func TestParseBodySkipsInlineNonTextParts(t *testing.T) {
	// An inline image (a cid: embedded part) must not be written into the readable plain body as raw
	// bytes, and it is not a saveable attachment either.
	raw := "From: a@b.com\r\n" +
		"To: c@d.com\r\n" +
		"Subject: Test\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/related; boundary=\"bd\"\r\n" +
		"\r\n" +
		"--bd\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Body text\r\n" +
		"--bd\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-Disposition: inline\r\n" +
		"Content-ID: <logo>\r\n" +
		"\r\n" +
		"PNGBYTES\r\n" +
		"--bd--\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if strings.Contains(parsed.Plain, "PNGBYTES") {
		t.Errorf("inline image bytes leaked into the plain body: %q", parsed.Plain)
	}
	if len(parsed.Attachments) != 0 {
		t.Errorf("inline image should not be a saveable attachment, got %d", len(parsed.Attachments))
	}
}

func TestParseBodyHTMLOnlyDerivesPlain(t *testing.T) {
	raw := "From: a@b.com\r\n" +
		"To: c@d.com\r\n" +
		"Subject: Test\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>Line one</p><p>Line two</p>\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	plain, html := parsed.Plain, parsed.HTML
	if !strings.Contains(html, "Line one") {
		t.Errorf("html = %q", html)
	}
	if !strings.Contains(plain, "Line one") || !strings.Contains(plain, "Line two") {
		t.Errorf("derived plain = %q, want both lines", plain)
	}
}

func TestParseBodyExtractsCalendarInvite(t *testing.T) {
	raw := "From: chair@example.com\r\n" +
		"To: guest@example.com\r\n" +
		"Subject: Invite\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"bd\"\r\n" +
		"\r\n" +
		"--bd\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Please come to the sync.\r\n" +
		"--bd\r\n" +
		"Content-Type: text/calendar; method=REQUEST; charset=utf-8\r\n" +
		"Content-Disposition: attachment; filename=\"invite.ics\"\r\n" +
		"\r\n" +
		"BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nEND:VCALENDAR\r\n" +
		"--bd--\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if !strings.Contains(string(parsed.Invite), "METHOD:REQUEST") {
		t.Errorf("calendar part not captured as invite: %q", parsed.Invite)
	}
	if !strings.Contains(parsed.Plain, "Please come to the sync") {
		t.Errorf("plain body lost: %q", parsed.Plain)
	}
	// The calendar payload must not leak into the readable body.
	if strings.Contains(parsed.Plain, "VCALENDAR") {
		t.Errorf("calendar payload leaked into the plain body: %q", parsed.Plain)
	}
}

func TestParseBodyNoCalendarYieldsNilInvite(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Just a note.\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if parsed.Invite != nil {
		t.Errorf("a message with no calendar part should yield a nil invite, got %q", parsed.Invite)
	}
}

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

func TestParseBodySanitizesHTML(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<p>Safe <b>text</b></p><script>alert('xss')</script>` +
		`<a href="javascript:evil()">bad</a><img src="http://x/pixel.gif" onerror="steal()">` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	for _, banned := range []string{"<script", "javascript:", "onerror", "alert("} {
		if strings.Contains(strings.ToLower(html), banned) {
			t.Errorf("sanitised html still contains %q: %s", banned, html)
		}
	}
	if !strings.Contains(html, "Safe") || !strings.Contains(html, "<b>text</b>") {
		t.Errorf("sanitiser removed safe formatting: %s", html)
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
		`<h1 style="opacity:0.9">Visible headline</h1>` +
		`<p style="line-height:0">Line height kept</p>` +
		`<p style="font-size:0.9em">Visible body text</p>` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	for _, gone := range []string{"Hidden preheader duplicate", "Zero opacity teaser", "Zero height preheader", "Hidden attribute snippet"} {
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

func TestPrepareHTMLParksPictureSource(t *testing.T) {
	out := prepareHTML(`<picture><source srcset="http://tracker.example/2x.webp">` +
		`<img src="http://tracker.example/pixel.gif"></picture>`)
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

func TestPrepareHTMLStripsRemoteCSSBackgroundInStyleAttr(t *testing.T) {
	out := prepareHTML(`<div style="color:red;background:url('http://tracker.example/bg.png')">hi</div>`)
	if strings.Contains(out, "tracker.example") {
		t.Errorf("a remote CSS url should be stripped, got: %s", out)
	}
	if !strings.Contains(out, "color:red") {
		t.Errorf("an unrelated style declaration should be preserved, got: %s", out)
	}
}

func TestPrepareHTMLStripsRemoteURLInStyleElement(t *testing.T) {
	out := prepareHTML(`<style>.hero{background:url(https://tracker.example/hero.jpg)}</style>`)
	if strings.Contains(out, "tracker.example") {
		t.Errorf("a remote url inside a <style> element should be stripped, got: %s", out)
	}
}

func TestPrepareHTMLKeepsEmbeddedDataURI(t *testing.T) {
	const dataURI = "data:image/png;base64,iVBORw0KGgo="
	out := prepareHTML(`<div style="background:url(` + dataURI + `)">x</div>`)
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

func TestDecodeHeader(t *testing.T) {
	cases := map[string]string{
		// RFC 2047 encoded-word in a non-UTF-8 charset (the =A3 is a pound sign).
		"=?Windows-1252?Q?circa_=A390k?=": "circa £90k",
		// HTML entities from a template-built subject are unescaped.
		"Data &amp; Analytics":      "Data & Analytics",
		"a &lt;b&gt; c &#39;d&#39;": "a <b> c 'd'",
		// A plain value is unchanged, and bare ampersands are left alone.
		"Fish & Chips at AT&T": "Fish & Chips at AT&T",
		"Plain subject":        "Plain subject",
		// A malformed encoded-word is not dropped; it is returned (unescaped) as-is.
		"=?utf-8?Q?broken": "=?utf-8?Q?broken",
	}
	for in, want := range cases {
		if got := DecodeHeader(in); got != want {
			t.Errorf("DecodeHeader(%q) = %q, want %q", in, got, want)
		}
	}
}
