package mailparse

import (
	"strings"
	"testing"
)

func TestLinkifyAnchorsBareHTTPSURL(t *testing.T) {
	out := LinkifyHTML("<p>see https://example.org/page for details</p>")
	want := `<a href="https://example.org/page">https://example.org/page</a>`
	if !strings.Contains(out, want) {
		t.Errorf("missing anchor %q in:\n%s", want, out)
	}
}

func TestLinkifyLeavesTrailingPunctuationOutside(t *testing.T) {
	out := LinkifyHTML("<p>read https://example.org/a, then https://example.org/b.</p>")
	wants := []string{
		`<a href="https://example.org/a">https://example.org/a</a>,`,
		`<a href="https://example.org/b">https://example.org/b</a>.`,
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestLinkifyAddsSchemeForWWWHosts(t *testing.T) {
	out := LinkifyHTML("<p>visit www.example.org today</p>")
	want := `<a href="http://www.example.org">www.example.org</a>`
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
}

func TestLinkifyAnchorsMailtoAddresses(t *testing.T) {
	out := LinkifyHTML("<p>write to mailto:jane@example.org please</p>")
	want := `<a href="mailto:jane@example.org">mailto:jane@example.org</a>`
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
}

func TestLinkifyNeverNestsInsideExistingAnchors(t *testing.T) {
	source := `<p><a href="https://example.org">https://example.org/text</a></p>`
	out := LinkifyHTML(source)
	if strings.Count(out, "<a ") != 1 {
		t.Errorf("expected the existing anchor to be untouched:\n%s", out)
	}
}

func TestLinkifySkipsScriptStyleAndTextarea(t *testing.T) {
	source := "<style>.x{background:url(https://example.org/i)}</style>" +
		"<textarea>https://example.org/t</textarea>"
	out := LinkifyHTML(source)
	if strings.Contains(out, "<a ") {
		t.Errorf("expected no anchors in non-prose text:\n%s", out)
	}
}

func TestLinkifyHandlesSeveralURLsInOneTextNode(t *testing.T) {
	out := LinkifyHTML("<p>https://a.example and https://b.example end</p>")
	if strings.Count(out, "<a ") != 2 {
		t.Errorf("expected two anchors:\n%s", out)
	}
	if !strings.Contains(out, "> and <") && !strings.Contains(out, "</a> and <a") {
		t.Errorf("text between links lost:\n%s", out)
	}
}

func TestLinkifyLeavesPlainTextAlone(t *testing.T) {
	out := LinkifyHTML("<p>no links here at all</p>")
	if strings.Contains(out, "<a ") {
		t.Errorf("unexpected anchor:\n%s", out)
	}
	if !strings.Contains(out, "no links here at all") {
		t.Errorf("text mangled:\n%s", out)
	}
}

func TestParsedMessageLinkifiesBareURLsInHTML(t *testing.T) {
	raw := "From: a@example.com\r\n" +
		"To: b@example.com\r\n" +
		"Subject: link\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>click https://example.org/page now</p>\r\n"
	body, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(body.HTML, `href="https://example.org/page"`) {
		t.Errorf("sanitised HTML lost the injected anchor:\n%s", body.HTML)
	}
}
