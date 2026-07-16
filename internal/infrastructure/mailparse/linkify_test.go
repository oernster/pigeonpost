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

func TestLinkifyRendersMarkdownLabelledLinks(t *testing.T) {
	out := LinkifyHTML("<p>go [Open the thing](https://example.org/very/long/path) now</p>")
	want := `<a href="https://example.org/very/long/path">Open the thing</a>`
	if !strings.Contains(out, want) {
		t.Errorf("missing labelled anchor %q in:\n%s", want, out)
	}
	if strings.Contains(out, "[Open the thing]") {
		t.Errorf("markdown syntax leaked into output:\n%s", out)
	}
}

func TestLinkifyMarkdownWWWTargetGetsScheme(t *testing.T) {
	out := LinkifyHTML("<p>[Site](www.example.org)</p>")
	want := `<a href="http://www.example.org">Site</a>`
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
}

func TestDisplayLinkifyMarksSoloParagraphLink(t *testing.T) {
	out := linkifyForDisplay("<p>https://example.org/page</p>")
	if !strings.Contains(out, `class="pp-solo-link"`) {
		t.Errorf("solo paragraph link not marked:\n%s", out)
	}
}

func TestDisplayLinkifyMarksSoloBRSeparatedLine(t *testing.T) {
	out := linkifyForDisplay(
		"<p>hello<br>[Open it](https://example.org/x)<br>bye</p>",
	)
	if !strings.Contains(out, `class="pp-solo-link"`) {
		t.Errorf("solo br-separated link not marked:\n%s", out)
	}
}

func TestDisplayLinkifyLeavesInlineLinksUnmarked(t *testing.T) {
	out := linkifyForDisplay("<p>see https://example.org/page for details</p>")
	if strings.Contains(out, "pp-solo-link") {
		t.Errorf("inline link wrongly marked solo:\n%s", out)
	}
}

func TestOutgoingLinkifyNeverAddsSoloClass(t *testing.T) {
	out := LinkifyHTML("<p>https://example.org/page</p>")
	if strings.Contains(out, "pp-solo-link") {
		t.Errorf("outgoing linkify added a display class:\n%s", out)
	}
}

func TestParsedMessageMarksSoloMarkdownLinkAsButton(t *testing.T) {
	raw := "From: a@example.com\r\n" +
		"To: b@example.com\r\n" +
		"Subject: button\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>hi<br>[Open the move](https://example.org/open)<br>bye</p>\r\n"
	body, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(body.HTML, `href="https://example.org/open"`) {
		t.Errorf("labelled link lost its target:\n%s", body.HTML)
	}
	if !strings.Contains(body.HTML, ">Open the move</a>") {
		t.Errorf("labelled link lost its label:\n%s", body.HTML)
	}
	if !strings.Contains(body.HTML, "pp-solo-link") {
		t.Errorf("solo link class did not survive sanitising:\n%s", body.HTML)
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

func TestDisplayLinkifyMarksExistingSoloAnchor(t *testing.T) {
	// The two-hop path: an outgoing message already carries a labelled
	// anchor (no class); the receiving side must still mark it solo.
	out := linkifyForDisplay(
		`<p>hi<br><a href="https://example.org/x">Open</a><br>bye</p>`,
	)
	if !strings.Contains(out, "pp-solo-link") {
		t.Errorf("existing solo anchor not marked:\n%s", out)
	}
}
