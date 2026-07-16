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

// visualStyleProperties are the inline-CSS properties the sanitiser keeps so an email renders with its
// intended fonts, colours, spacing, borders and layout. bluemonday drops any property not listed here, so
// the set is deliberately comprehensive. It is safe to keep this much styling because the reader shows the
// result inside a sandboxed, CSP-locked iframe (see the frontend EmailHtmlFrame), which is the real
// security boundary: CSS there cannot run scripts, cannot fetch remote resources and cannot reach the app.
var visualStyleProperties = []string{
	"color", "opacity", "visibility", "cursor", "outline",
	"background", "background-color", "background-image", "background-position",
	"background-repeat", "background-size", "background-clip", "background-origin", "background-attachment",
	"font", "font-family", "font-size", "font-weight", "font-style", "font-variant",
	"line-height", "letter-spacing", "word-spacing", "text-align", "text-decoration",
	"text-transform", "text-indent", "text-overflow", "text-shadow", "white-space",
	"vertical-align", "direction", "word-break", "overflow-wrap",
	"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
	"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
	"border", "border-width", "border-style", "border-color",
	"border-top", "border-right", "border-bottom", "border-left",
	"border-top-width", "border-top-style", "border-top-color",
	"border-right-width", "border-right-style", "border-right-color",
	"border-bottom-width", "border-bottom-style", "border-bottom-color",
	"border-left-width", "border-left-style", "border-left-color",
	"border-radius", "border-top-left-radius", "border-top-right-radius",
	"border-bottom-right-radius", "border-bottom-left-radius", "border-collapse", "border-spacing",
	"width", "min-width", "max-width", "height", "min-height", "max-height",
	"display", "float", "clear", "overflow", "overflow-x", "overflow-y",
	"box-sizing", "box-shadow", "table-layout",
	"list-style", "list-style-type", "list-style-position", "list-style-image",
	"flex", "flex-direction", "flex-wrap", "flex-flow", "align-items", "align-content",
	"justify-content", "gap", "row-gap", "column-gap",
}

// presentationalAlignElements are the elements an align attribute is kept on. Legacy email layouts lean on
// align (and the table attributes allowed in buildSanitizer) rather than CSS, so keeping them preserves the
// intended column and cell alignment.
var presentationalAlignElements = []string{
	"table", "thead", "tbody", "tfoot", "tr", "td", "th", "col", "colgroup",
	"div", "p", "img", "h1", "h2", "h3", "h4", "h5", "h6", "center", "caption", "font",
}

// cssValueMatcher accepts the characters that make up ordinary CSS values (words, whitespace, hex colours,
// percentages, decimals, value lists, functions such as rgb()/calc()/url(), quoted font stacks, data: URIs
// and !important). It exists only so bluemonday keeps a declaration rather than applying its strict
// built-in per-property handler, which rejects common email values like the shorthand "18px 24px".
var cssValueMatcher = regexp.MustCompile(`^[\w\s#%.,:;!+*/()'"=-]*$`)

// keepCSSValue is the permissive value handler bluemonday runs for every allowed visual property. Strict
// validation is unnecessary because the CSP-locked iframe (not this sanitiser) is the security boundary, so
// it keeps any value made of ordinary CSS characters and rejects only ones no real value uses. Remote
// url() targets are already neutralised by prepareHTML before sanitising, so a surviving url() is a data:
// or cid: reference the CSP still permits.
func keepCSSValue(value string) bool {
	return cssValueMatcher.MatchString(value)
}

// buildSanitizer builds the policy that cleans message HTML. It starts from bluemonday's UGCPolicy (which
// strips <script>, <iframe>, <object>, event-handler attributes and javascript: URLs) and then relaxes it
// to preserve visual styling, since the sandboxed, CSP-locked iframe the reader renders into is the real
// security boundary. The script, frame, object and handler protections are all kept.
func buildSanitizer() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs(blockedImageAttr).OnElements("img")
	policy.AllowDataURIImages()

	// Keep inline styles, the class attribute and the presentational table attributes real emails use for
	// layout. Style values pass through keepCSSValue so faithful (lax) email CSS survives.
	policy.AllowStyling()
	policy.AllowStyles(visualStyleProperties...).MatchingHandler(keepCSSValue).Globally()
	policy.AllowAttrs("bgcolor").OnElements("body", "table", "tr", "td", "th")
	policy.AllowAttrs("align").OnElements(presentationalAlignElements...)
	policy.AllowAttrs("valign").OnElements("table", "thead", "tbody", "tfoot", "tr", "td", "th", "col", "colgroup")
	policy.AllowAttrs("width", "height").OnElements("table", "tr", "td", "th", "img", "col", "colgroup")
	policy.AllowAttrs("cellpadding", "cellspacing", "border").OnElements("table")

	// Keep <style> blocks so class-based and media-query styling renders. AllowUnsafe lets the CSS text of
	// an allowed <style> pass through unescaped; it does NOT re-admit <script>, <iframe> or <object>, which
	// stay in bluemonday's skip-content set and are never allowed elements, so they and their content are
	// still removed (verified against bluemonday v1.0.27 and pinned by the sanitiser tests).
	policy.AllowElements("style")
	policy.AllowUnsafe(true)
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
	acc := bodyAccumulator{inlineImages: map[string]inlineImage{}}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParsedBody{}, fmt.Errorf("mailparse: read part: %w", err)
		}
		if err := acc.handlePart(part); err != nil {
			return ParsedBody{}, err
		}
	}
	plain := acc.plain.String()
	htmlBody := acc.html.String()
	if htmlBody != "" {
		prepared := LinkifyHTML(prepareHTML(htmlBody, acc.inlineImages))
		htmlBody = htmlSanitizer.Sanitize(prepared)
	}
	if strings.TrimSpace(plain) == "" && htmlBody != "" {
		plain = htmlToText(htmlBody)
	}
	return ParsedBody{Plain: plain, HTML: htmlBody, Invite: acc.invite, Attachments: acc.attachments}, nil
}

// bodyAccumulator gathers the parts of a MIME message as ParseBody walks them: the plain and HTML
// bodies, an inline-image table keyed by Content-ID, the saveable attachments and the first calendar
// invite.
type bodyAccumulator struct {
	plain        strings.Builder
	html         strings.Builder
	invite       []byte
	attachments  []ParsedAttachment
	inlineImages map[string]inlineImage
}

// handlePart folds a single MIME part into the accumulator: a text/calendar part becomes the invite, an
// image carrying a Content-ID is remembered for inline display, and an attachment- or inline-dispositioned
// part is stored or written into the matching body buffer.
func (acc *bodyAccumulator) handlePart(part *mail.Part) error {
	mediaType := partMediaType(part.Header)
	if mediaType == "text/calendar" {
		content, err := io.ReadAll(part.Body)
		if err != nil {
			return fmt.Errorf("mailparse: read part body: %w", err)
		}
		if acc.invite == nil {
			acc.invite = content
		}
		return nil
	}
	content, err := io.ReadAll(part.Body)
	if err != nil {
		return fmt.Errorf("mailparse: read part body: %w", err)
	}
	// A part with a Content-ID and an image payload is an embedded image the HTML references by a cid:
	// URL; collect it (whether the sender marked it inline or as an attachment) so the HTML can show it
	// inline. An attachment-dispositioned one still lists as a saveable attachment below as well.
	if id := contentID(part.Header); id != "" && strings.HasPrefix(mediaType, "image/") {
		acc.inlineImages[id] = inlineImage{mediaType: mediaType, content: content}
	}
	switch header := part.Header.(type) {
	case *mail.AttachmentHeader:
		acc.attachments = append(acc.attachments, attachmentPart(header, mediaType, content))
	case *mail.InlineHeader:
		switch {
		case mediaType == "text/html":
			acc.html.Write(content)
		case mediaType == "text/plain" || mediaType == "":
			acc.plain.Write(content)
		default:
			// An inline non-text part with no usable Content-ID is neither readable text nor a
			// referenced embedded image (a cid: image is collected above), so it is skipped rather than
			// written into the body as raw bytes.
		}
	}
	return nil
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
