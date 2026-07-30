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

func TestParseBodyPreservesInlineStyle(t *testing.T) {
	// The sanitiser keeps a comprehensive set of visual CSS so an email renders faithfully inside the
	// reader's sandboxed iframe. A representative inline style (size, colour, background, radius, shorthand
	// padding) must survive ParseBody with its properties intact.
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<div style="font-size:32px;color:#0F0F0D;background-color:#FFFFFF;border-radius:12px;padding:18px 24px">Styled</div>` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	for _, want := range []string{
		"font-size", "32px", "color", "#0F0F0D", "background-color", "#FFFFFF",
		"border-radius", "12px", "padding", "18px 24px",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("inline style should survive ParseBody, missing %q: %s", want, html)
		}
	}
}

func TestParseBodyKeepsStyleBlockAndClassButStripsScriptAndHandlers(t *testing.T) {
	// Preserving styling must not weaken the script or event-handler protections. A <style> block and a
	// class hook survive so class-based styling renders, while an inline <script> and an onclick handler are
	// still removed even though AllowUnsafe lets the <style> CSS text through.
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<style>.btn{color:#ffffff;background:#000000}</style>` +
		`<a class="btn" href="https://example.com" onclick="evil()">Go</a>` +
		`<script>alert(1)</script>` + "\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	html := parsed.HTML
	for _, want := range []string{"<style>", ".btn", "#000000", `class="btn"`} {
		if !strings.Contains(html, want) {
			t.Errorf("style block and class hook should survive, missing %q: %s", want, html)
		}
	}
	for _, banned := range []string{"<script", "alert(", "onclick"} {
		if strings.Contains(strings.ToLower(html), banned) {
			t.Errorf("dangerous content should still be removed, found %q: %s", banned, html)
		}
	}
}

func TestParseBodyNormalisesWrappedHrefsSoLinksSurvive(t *testing.T) {
	// Bulk-mail senders wrap long href values across source lines, leaving tabs, newlines or encoded
	// line breaks inside the URL. The URL standard strips those characters and browsers follow the link,
	// but Go's url.Parse rejects them, so without normalisation the sanitiser deleted the whole anchor
	// and every button in such an email rendered styled but dead. Each wrapped form must survive
	// ParseBody as a working link with the joined target.
	cases := []struct {
		name string
		body string
		want string
	}{
		{"newline inside href", "<a href=\"https://example.com/re\nset?token=abc\">Go</a>", "https://example.com/reset?token=abc"},
		{"crlf inside href", "<a href=\"https://example.com/re\r\nset?token=abc\">Go</a>", "https://example.com/reset?token=abc"},
		{"tab inside href", "<a href=\"https://example.com/re\tset?token=abc\">Go</a>", "https://example.com/reset?token=abc"},
		{"encoded newline inside href", `<a href="https://example.com/re&#10;set?token=abc">Go</a>`, "https://example.com/reset?token=abc"},
		{"padded href", `<a href="   https://example.com/reset?token=abc  ">Go</a>`, "https://example.com/reset?token=abc"},
		{"interior space percent-encoded", `<a href="https://example.com/a b/c">Go</a>`, "https://example.com/a%20b/c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := "MIME-Version: 1.0\r\n" +
				"Content-Type: text/html; charset=utf-8\r\n" +
				"\r\n" +
				"<html><body>" + tc.body + "</body></html>\r\n"
			parsed, err := ParseBody([]byte(raw))
			if err != nil {
				t.Fatalf("ParseBody: %v", err)
			}
			if !strings.Contains(parsed.HTML, `href="`+tc.want+`"`) {
				t.Errorf("wrapped href should survive as %q: %s", tc.want, parsed.HTML)
			}
		})
	}
}

func TestParseBodyStillRejectsObfuscatedJavascriptHref(t *testing.T) {
	// Href normalisation runs before the sanitiser, so a javascript: URL obfuscated with an embedded
	// newline (the classic filter-evasion form) is made legible first and then rejected by the scheme
	// policy. The de-obfuscated scheme must never survive into the rendered HTML.
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<a href=\"java\nscript:alert(1)\">Go</a>\r\n"

	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	if strings.Contains(strings.ToLower(parsed.HTML), "javascript") {
		t.Errorf("obfuscated javascript: href must not survive sanitising: %s", parsed.HTML)
	}
}
