package mailparse

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// parseHTMLBody runs an HTML fragment through the whole body pipeline (prepare plus sanitise), so a test
// sees exactly what the reader, the print path and a reply would receive.
func parseHTMLBody(t *testing.T, fragment string) string {
	t.Helper()
	raw := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" + fragment + "\r\n"
	parsed, err := ParseBody([]byte(raw))
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	return parsed.HTML
}

// imageSources returns every live src and every parked data-pp-src value on the <img> elements of a
// rendered body, read from the parsed tree rather than by string matching so attribute quoting cannot hide
// a live source.
func imageSources(t *testing.T, body string) (live, parked []string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for _, attr := range n.Attr {
				switch attr.Key {
				case "src":
					live = append(live, attr.Val)
				case blockedImageAttr:
					parked = append(parked, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return live, parked
}

// A browser fetches far more spellings than the http://, https:// and // prefixes: it folds backslashes into
// slashes, accepts a single slash or none after an http(s) scheme, ignores case and strips tab and newline
// anywhere in the value. Each of these must be parked, as must any other non-embedded source.
func TestParseBodyParksEveryNonEmbeddedImageSource(t *testing.T) {
	sources := []string{
		`http:\\tracker.example\x.png`,
		`http:/tracker.example/x`,
		`https:tracker.example/x`,
		`HTTP:/tracker.example/x`,
		`HtTpS:tracker.example/x`,
		`  https:tracker.example/x  `,
		"\thttp:\\\\tracker.example\\x.png",
		"ht\ntp://tracker.example/x",
		`//tracker.example/x`,
		`\\tracker.example\x`,
		`ftp://tracker.example/x`,
		`relative/x.png`,
	}
	for _, src := range sources {
		body := parseHTMLBody(t, `<p>Hi</p><img src="`+src+`" alt="pic">`)
		live, parked := imageSources(t, body)
		if len(live) != 0 {
			t.Errorf("src %q left live: %v (body %s)", src, live, body)
		}
		if len(parked) != 1 {
			t.Errorf("src %q not parked: %v (body %s)", src, parked, body)
		}
	}
}

// An embedded source is shown at once whatever its case or surrounding whitespace. An empty source
// (which no browser fetches) is not parked, so it raises no Load images offer for nothing.
func TestParseBodyLeavesEmbeddedAndEmptyImageSourcesUnparked(t *testing.T) {
	dataURI := "data:image/png;base64,iVBORw0KGgo="
	for _, src := range []string{dataURI, "  DATA:image/png;base64,iVBORw0KGgo=", "CID:missing", ""} {
		body := parseHTMLBody(t, `<img src="`+src+`" alt="pic">`)
		if _, parked := imageSources(t, body); len(parked) != 0 {
			t.Errorf("src %q parked, want left in place: %s", src, body)
		}
	}
}

// remoteCSSCases are style blocks a browser would fetch from without a url( token the old parser could see.
var remoteCSSCases = map[string]string{
	"import string":          `<style>@import "http://tracker.example/a.css";.k{color:red}</style>`,
	"import url":             `<style>@import url(http://tracker.example/a.css) screen;.k{color:red}</style>`,
	"escaped import keyword": `<style>@\69mport "http://tracker.example/a.css";.k{color:red}</style>`,
	"escaped url hex":        `<style>.k{color:red;background:u\72l(http://tracker.example/a.png)}</style>`,
	"escaped url hex spaced": `<style>.k{color:red;background:u\000072 l(http://tracker.example/a.png)}</style>`,
	"escaped url plain char": `<style>.k{color:red;background:\u\r\l(http://tracker.example/a.png)}</style>`,
	"escaped paren":          `<style>.k{color:red;background:url\28 http://tracker.example/a.png)}</style>`,
	"image-set":              `<style>.k{color:red;background-image:image-set("http://tracker.example/a.png" 1x,'https://tracker.example/b.png' 2x)}</style>`,
	"webkit image-set":       `<style>.k{color:red;background-image:-webkit-image-set("http://tracker.example/a.png" 1x)}</style>`,
	"image-set style attr":   `<div style="color:red;background-image:image-set('http://tracker.example/a.png' 1x)">x</div>`,
	"url other quote inside": `<style>.k{color:red;background:url("http://tracker.example/a'b.png")}</style>`,
	"url paren inside":       `<style>.k{color:red;background:url('http://tracker.example/a)b.png')}</style>`,
	"url escaped quote":      `<style>.k{color:red;background:url("http://tracker.example/a\"b.png")}</style>`,
	"url escaped space":      `<style>.k{color:red;background:url(http://tracker.example/a\ b.png)}</style>`,
}

// Every remote reference in CSS is neutralised, whether spelt with an escape, reached through @import or
// given as a bare string to image-set, while the rest of the stylesheet survives.
func TestParseBodyNeutralisesRemoteCSSWithoutURLToken(t *testing.T) {
	for name, fragment := range remoteCSSCases {
		body := parseHTMLBody(t, fragment)
		if strings.Contains(body, "tracker.example") {
			t.Errorf("%s: remote target survived: %s", name, body)
		}
		if strings.Contains(strings.ToLower(body), "@import") {
			t.Errorf("%s: @import survived: %s", name, body)
		}
		// The sanitiser reserialises an inline style with a space after each colon, so spaces are ignored.
		if !strings.Contains(strings.ReplaceAll(body, " ", ""), "color:red") {
			t.Errorf("%s: surrounding CSS lost: %s", name, body)
		}
	}
}

// A remote image-set candidate is parked rather than discarded, so Load images can still bring it back.
func TestParseBodyParksImageSetCandidates(t *testing.T) {
	body := parseHTMLBody(t, `<style>.k{background-image:image-set("https://tracker.example/a.png" 1x)}</style>`)
	if !strings.Contains(body, "image-set(url("+parkedCSSURLScheme) {
		t.Errorf("image-set candidate not parked: %s", body)
	}
}

// Escapes that do not spell a fetch are left exactly as written: a quote glyph in generated content, a
// class name that starts with a digit, a CJK font name and an escaped backslash before hex digits.
func TestParseBodyKeepsHarmlessCSSEscapes(t *testing.T) {
	for _, css := range []string{
		`.q:before{content:"\201C"}`,
		`.\31 0{color:red}`,
		`.f{font-family:"\5FAE\8F6F\96C5\9ED1"}`,
		`.b:after{content:"\\72"}`,
		`.i{background-image:image-set("a.avif" type("image/avif") 1x)}`,
	} {
		body := parseHTMLBody(t, `<style>`+css+`</style>`)
		want := css
		if strings.Contains(css, "image-set") {
			want = `type("image/avif")`
		}
		if !strings.Contains(body, want) {
			t.Errorf("CSS %q altered: %s", css, body)
		}
	}
}
