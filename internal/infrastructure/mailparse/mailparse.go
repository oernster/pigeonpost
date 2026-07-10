// Package mailparse turns a raw RFC 5322 message into the plain-text and sanitised HTML bodies the UI
// renders. It is shared by the mail source adapters (IMAP and POP3) so the message-body handling,
// including HTML sanitising and remote-image blocking, lives in one place rather than being duplicated
// per protocol.
package mailparse

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"io"
	"mime"
	"regexp"
	"strings"

	// Its init registers a CharsetReader on go-message so bodies in legacy charsets (iso-8859-1,
	// windows-1252, and the rest) decode instead of failing with "unhandled charset"; charset.Reader is
	// also used directly by the header-word decoder below.
	"github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"

	"github.com/oernster/pigeonpost/internal/domain"
)

// htmlSanitizer strips anything unsafe (scripts, event handlers, javascript: URLs, style/iframe/form
// elements) from message HTML while keeping common formatting and links. It allows the data-pp-src
// attribute on images, where parkElementSource parks a remote image's original source so it does not
// load until the reader asks. It also allows data: image URIs so an embedded image resolved from a
// cid: reference renders inline. It is built once and safe for concurrent use.
var htmlSanitizer = buildSanitizer()

// blockedImageAttr holds a remote image's original src while it is prevented from loading.
const blockedImageAttr = "data-pp-src"

func buildSanitizer() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs(blockedImageAttr).OnElements("img")
	policy.AllowDataURIImages()
	return policy
}

// blankLines collapses three or more consecutive newlines down to two.
var blankLines = regexp.MustCompile(`\n{3,}`)

// ParsedAttachment is one file carried by a message: its display filename, MIME content type and raw
// bytes. Filename and ContentType are always set (a missing name falls back to a generic one) so a caller
// can build a domain Attachment without further validation.
type ParsedAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// ParsedBody is the result of parsing a raw message: its plain-text and HTML bodies, the raw
// text/calendar payload when the message carried one (an iMIP scheduling object such as a meeting
// invite), and any attachment parts. Invite is nil when the message carried no calendar part; Attachments
// is empty when it carried no files.
type ParsedBody struct {
	Plain       string
	HTML        string
	Invite      []byte
	Attachments []ParsedAttachment
}

// fallbackAttachmentName names an attachment part that carried no filename, so it can still be listed and
// saved.
const fallbackAttachmentName = "attachment"

// fallbackAttachmentType is the generic MIME type used when an attachment part has no readable
// Content-Type.
const fallbackAttachmentType = "application/octet-stream"

// ParseBody parses a raw RFC 5322 message into its plain-text and HTML bodies plus any text/calendar
// scheduling payload. When the message has only an HTML body, a plain-text rendering is derived from it
// so the message is always readable. A text/calendar part (whether sent inline or as an attachment) is
// captured as the invite rather than folded into the readable body; the first such part wins.
func ParseBody(raw []byte) (ParsedBody, error) {
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return ParsedBody{}, fmt.Errorf("mailparse: parse message: %w", err)
	}
	var plainBuf, htmlBuf strings.Builder
	var invite []byte
	var attachments []ParsedAttachment
	inlineImages := map[string]inlineImage{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParsedBody{}, fmt.Errorf("mailparse: read part: %w", err)
		}
		mediaType := partMediaType(part.Header)
		if mediaType == "text/calendar" {
			content, err := io.ReadAll(part.Body)
			if err != nil {
				return ParsedBody{}, fmt.Errorf("mailparse: read part body: %w", err)
			}
			if invite == nil {
				invite = content
			}
			continue
		}
		content, err := io.ReadAll(part.Body)
		if err != nil {
			return ParsedBody{}, fmt.Errorf("mailparse: read part body: %w", err)
		}
		// A part with a Content-ID and an image payload is an embedded image the HTML references by a cid:
		// URL; collect it (whether the sender marked it inline or as an attachment) so the HTML can show it
		// inline. An attachment-dispositioned one still lists as a saveable attachment below as well.
		if id := contentID(part.Header); id != "" && strings.HasPrefix(mediaType, "image/") {
			inlineImages[id] = inlineImage{mediaType: mediaType, content: content}
		}
		switch header := part.Header.(type) {
		case *mail.AttachmentHeader:
			attachments = append(attachments, attachmentPart(header, mediaType, content))
		case *mail.InlineHeader:
			switch {
			case mediaType == "text/html":
				htmlBuf.Write(content)
			case mediaType == "text/plain" || mediaType == "":
				plainBuf.Write(content)
			default:
				// An inline non-text part with no usable Content-ID is neither readable text nor a
				// referenced embedded image (a cid: image is collected above), so it is skipped rather than
				// written into the body as raw bytes.
			}
		}
	}
	plain := plainBuf.String()
	htmlBody := htmlBuf.String()
	if htmlBody != "" {
		htmlBody = htmlSanitizer.Sanitize(prepareHTML(htmlBody, inlineImages))
	}
	if strings.TrimSpace(plain) == "" && htmlBody != "" {
		plain = htmlToText(htmlBody)
	}
	return ParsedBody{Plain: plain, HTML: htmlBody, Invite: invite, Attachments: attachments}, nil
}

// headerWordDecoder decodes RFC 2047 encoded-words in header text (subjects, display names, attachment
// filenames). The go-message charset reader lets it handle the non-UTF-8 charsets real senders use, such
// as windows-1252 and iso-8859-1, which the standard library's decoder rejects on its own.
var headerWordDecoder = mime.WordDecoder{CharsetReader: charset.Reader}

// DecodeHeader prepares a header value (a subject, display name or attachment filename) for display. It
// turns any RFC 2047 encoded-words into plain UTF-8, so "=?Windows-1252?Q?Re:_...?=" reads as text, and
// then unescapes HTML entities, so a subject a sender built from an HTML template ("Data &amp; Analytics")
// shows the real character. A value that carries neither is returned unchanged, and one whose
// encoded-words fail to decode is unescaped as-is, so a header quirk never drops a message.
func DecodeHeader(value string) string {
	decoded := value
	if strings.Contains(value, "=?") {
		if d, err := headerWordDecoder.DecodeHeader(value); err == nil {
			decoded = d
		}
	}
	return stdhtml.UnescapeString(decoded)
}

// DomainAttachments converts parsed attachment parts into domain Attachments, applying the domain's own
// validation and byte-copying. ParseBody guarantees a non-empty filename, so it does not fail on that.
func DomainAttachments(parsed []ParsedAttachment) ([]domain.Attachment, error) {
	out := make([]domain.Attachment, 0, len(parsed))
	for _, p := range parsed {
		att, err := domain.NewAttachment(p.Filename, p.ContentType, p.Content)
		if err != nil {
			return nil, fmt.Errorf("mailparse: build attachment %q: %w", p.Filename, err)
		}
		out = append(out, att)
	}
	return out, nil
}

// attachmentPart builds a ParsedAttachment from an attachment part, falling back to a generic name and
// media type when the part supplies none, so every attachment is listable and saveable.
func attachmentPart(header *mail.AttachmentHeader, mediaType string, content []byte) ParsedAttachment {
	filename, err := header.Filename()
	// Some senders (Outlook) wrap the filename in an RFC 2047 encoded-word, which the media-type parser
	// leaves raw, so decode it here as well as the header-level subjects and names.
	filename = DecodeHeader(filename)
	if err != nil || strings.TrimSpace(filename) == "" {
		filename = fallbackAttachmentName
	}
	if mediaType == "" {
		mediaType = fallbackAttachmentType
	}
	return ParsedAttachment{Filename: filename, ContentType: mediaType, Content: content}
}

// partMediaType returns a part's media type (lowercased, without parameters), or an empty string when
// the part has no readable Content-Type. It reads the raw header so it works for an attachment part as
// well as an inline one, since the PartHeader interface exposes no typed ContentType accessor.
func partMediaType(header mail.PartHeader) string {
	mediaType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return mediaType
}

// prepareHTML walks the parsed message HTML before sanitising to do two things the sanitiser cannot.
// First it removes nodes the sender deliberately hid with inline CSS (a preheader / preview-text block,
// the snippet a mail client shows in the message list). Those nodes are meant to stay invisible, but
// the sanitiser strips the style attribute that hides them, so left in place they would surface and
// duplicate the visible content; they are dropped here while their hiding style is still readable.
// Second it stops the message from auto-loading any remote resource, which would leak that the reader
// opened it (and their IP) to the sender. It parks a remote <img> or <picture> <source> src into a
// data attribute and drops srcset; it also neutralises remote url(...) references in inline style
// attributes and <style> elements, so a CSS background cannot be used as a tracking pixel either. An
// embedded image is shown at once: a cid: reference is resolved to the message's own image part as a
// data: URI, while an inline data: URI is kept. On a parse or render failure the original HTML is returned
// unchanged; the sanitizer still runs over it afterwards.
func prepareHTML(source string, inline map[string]inlineImage) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img", "source":
				parkElementSource(n, inline)
			case "style":
				stripStyleElementURLs(n)
			}
			stripStyleAttrURLs(n)
		}
		var next *html.Node
		for c := n.FirstChild; c != nil; c = next {
			next = c.NextSibling
			if c.Type == html.ElementNode && isHiddenBySender(c) {
				n.RemoveChild(c)
				continue
			}
			walk(c)
		}
	}
	walk(doc)
	var b strings.Builder
	if err := html.Render(&b, doc); err != nil {
		return source
	}
	return b.String()
}

// hiddenStyleRe matches the inline CSS senders use to hide preheader / preview text: an off display, an
// invisible or zero-opacity box, a collapsed height, or a zero or 1px font. Numeric values are anchored
// to a declaration terminator (optionally through !important) so a visible opacity:0.9 or
// font-size:0.5em is not caught.
var hiddenStyleRe = regexp.MustCompile(`(?i)(?:display\s*:\s*none|visibility\s*:\s*hidden|mso-hide\s*:\s*all|opacity\s*:\s*0(?:\.0+)?(?:\s*!important)?\s*(?:;|$)|(?:^|[;{\s])(?:max-)?height\s*:\s*0(?:px)?(?:\s*!important)?\s*(?:;|$)|font-size\s*:\s*(?:0(?:px|pt|em|rem)?|1px)(?:\s*!important)?\s*(?:;|$))`)

// isHiddenBySender reports whether an element is one the sender hid from view, via the HTML hidden
// attribute or an inline style that makes it invisible. Such elements are preheader / preview text that
// must not surface once the sanitiser removes the style that hides them.
func isHiddenBySender(n *html.Node) bool {
	for _, attr := range n.Attr {
		switch {
		case strings.EqualFold(attr.Key, "hidden"):
			return true
		case strings.EqualFold(attr.Key, "style") && hiddenStyleRe.MatchString(attr.Val):
			return true
		}
	}
	return false
}

// parkElementSource rewrites an image element's src for safe display and drops srcset. An embedded
// image is shown at once: a cid: reference is swapped for the matching part's data: URI, while an
// inline data: URI is left as is. A remote src is parked into the blocked-image data attribute instead, so the
// browser fetches nothing until the reader asks. srcset is always dropped, being a second way to
// trigger a remote fetch. It covers <img> and the <source> children of a <picture>.
func parkElementSource(n *html.Node, inline map[string]inlineImage) {
	kept := n.Attr[:0]
	for _, attr := range n.Attr {
		switch strings.ToLower(attr.Key) {
		case "src":
			if resolved, ok := resolveInlineImage(attr.Val, inline); ok {
				attr.Val = resolved
			} else if isRemoteURL(attr.Val) {
				attr.Key = blockedImageAttr
			}
			kept = append(kept, attr)
		case "srcset":
			// Dropped: srcset is another way to trigger a remote fetch.
		default:
			kept = append(kept, attr)
		}
	}
	n.Attr = kept
}

// remoteCSSURLRe matches a CSS url(...) reference and captures its target, so a remote target can be
// told apart from an embedded one.
var remoteCSSURLRe = regexp.MustCompile(`(?i)url\(\s*['"]?([^)'"]*)['"]?\s*\)`)

// stripRemoteCSSURLs replaces every remote url(...) in a CSS fragment with an empty url(), leaving
// embedded data: and cid: references intact. A tracker can pull a remote file through a CSS background
// just as through an <img>, so this closes that vector in both inline styles and <style> elements.
func stripRemoteCSSURLs(css string) string {
	return remoteCSSURLRe.ReplaceAllStringFunc(css, func(match string) string {
		target := strings.ToLower(strings.TrimSpace(remoteCSSURLRe.FindStringSubmatch(match)[1]))
		if strings.HasPrefix(target, "data:") || strings.HasPrefix(target, "cid:") {
			return match
		}
		return "url()"
	})
}

// stripStyleAttrURLs neutralises remote url(...) references in an element's inline style attribute.
func stripStyleAttrURLs(n *html.Node) {
	for i, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "style") {
			n.Attr[i].Val = stripRemoteCSSURLs(attr.Val)
		}
	}
}

// stripStyleElementURLs neutralises remote url(...) references inside a <style> element's CSS text.
func stripStyleElementURLs(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			c.Data = stripRemoteCSSURLs(c.Data)
		}
	}
}

// htmlToText renders HTML into readable plain text: it drops script/style, turns <br> and the close of
// block elements into line breaks, and collapses runs of blank lines.
func htmlToText(source string) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && n.Data == "br" {
			b.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && isBlockElement(n.Data) {
			b.WriteByte('\n')
		}
	}
	walk(doc)
	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(strings.Join(strings.Fields(line), " "), " ")
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "section", "article":
		return true
	default:
		return false
	}
}
