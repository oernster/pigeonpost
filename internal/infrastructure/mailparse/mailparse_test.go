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

	plain, html, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if !strings.Contains(plain, "Hello plain") {
		t.Errorf("plain = %q, want it to contain Hello plain", plain)
	}
	if !strings.Contains(html, "<b>html</b>") {
		t.Errorf("html = %q, want it to contain the html part", html)
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

	plain, html, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if !strings.Contains(html, "Line one") {
		t.Errorf("html = %q", html)
	}
	if !strings.Contains(plain, "Line one") || !strings.Contains(plain, "Line two") {
		t.Errorf("derived plain = %q, want both lines", plain)
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

	_, html, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
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

	_, html, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
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

func TestParseBodyBlocksRemoteImages(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<p>Hello</p><img src="http://tracker.example/pixel.gif" srcset="http://tracker.example/2x.gif 2x" alt="pic">` + "\r\n"

	_, html, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
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
