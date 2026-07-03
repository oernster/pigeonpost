// Package imap implements the application MailSource interface against a live IMAP server using
// emersion/go-imap v2. The pure mapping between IMAP wire types and domain types lives in this file
// so it can be unit-tested without a network connection.
package imap

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"

	"github.com/oernster/pigeonpost/internal/domain"
)

// htmlSanitizer strips anything unsafe (scripts, event handlers, javascript: URLs, style/iframe/form
// elements) from message HTML while keeping common formatting and links. It also allows the
// data-pp-src attribute on images, which is where blockRemoteImages parks a remote image's original
// source so it does not load until the reader asks for it. It is built once and safe for concurrent use.
var htmlSanitizer = buildSanitizer()

// blockedImageAttr holds a remote image's original src while it is prevented from loading.
const blockedImageAttr = "data-pp-src"

func buildSanitizer() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs(blockedImageAttr).OnElements("img")
	return policy
}

// folderIDSeparator joins account, mailbox and uid into stable local identifiers. It is a control
// character that does not appear in mailbox names or email addresses.
const folderIDSeparator = "\x1f"

func makeFolderID(accountID, mailbox string) string {
	return accountID + folderIDSeparator + mailbox
}

func makeMessageID(folderID string, uid uint32) string {
	return fmt.Sprintf("%s%s%d", folderID, folderIDSeparator, uid)
}

// folderKindFor classifies a mailbox from its name and RFC 6154 special-use attributes.
func folderKindFor(mailbox string, attrs []imap.MailboxAttr) domain.FolderKind {
	if strings.EqualFold(mailbox, "INBOX") {
		return domain.FolderInbox
	}
	for _, attr := range attrs {
		switch attr {
		case imap.MailboxAttrSent:
			return domain.FolderSent
		case imap.MailboxAttrDrafts:
			return domain.FolderDrafts
		case imap.MailboxAttrTrash:
			return domain.FolderTrash
		case imap.MailboxAttrJunk:
			return domain.FolderJunk
		case imap.MailboxAttrArchive:
			return domain.FolderArchive
		}
	}
	return domain.FolderCustom
}

// buildFolder maps a LIST response into a domain folder. Counts are unknown from LIST alone and
// default to zero until a later STATUS/SELECT sync fills them in.
func buildFolder(accountID string, data *imap.ListData) (domain.Folder, error) {
	kind := folderKindFor(data.Mailbox, data.Attrs)
	separator := ""
	if data.Delim != 0 {
		separator = string(data.Delim)
	}
	return domain.NewFolderWithSeparator(
		makeFolderID(accountID, data.Mailbox), accountID, data.Mailbox, separator, kind, 0, 0)
}

// mapFlags converts IMAP flags into the domain flag set, ignoring flags the domain does not model.
func mapFlags(flags []imap.Flag) domain.Flags {
	out := domain.NewFlags(0)
	for _, flag := range flags {
		switch flag {
		case imap.FlagSeen:
			out = out.With(domain.FlagSeen)
		case imap.FlagAnswered:
			out = out.With(domain.FlagAnswered)
		case imap.FlagFlagged:
			out = out.With(domain.FlagFlagged)
		case imap.FlagDraft:
			out = out.With(domain.FlagDraft)
		case imap.FlagDeleted:
			out = out.With(domain.FlagDeleted)
		}
	}
	return out
}

// firstAddress maps the first envelope address into a domain address. An empty list or an
// unparseable address yields the zero address rather than an error, because a missing sender must
// not fail a whole sync.
func firstAddress(addrs []imap.Address) domain.EmailAddress {
	if len(addrs) == 0 {
		return domain.EmailAddress{}
	}
	a := addrs[0]
	email, err := domain.NewEmailAddress(a.Name, a.Mailbox+"@"+a.Host)
	if err != nil {
		return domain.EmailAddress{}
	}
	return email
}

// allAddresses maps every parseable envelope address into domain addresses, skipping any that do not
// parse. Used for the To and Cc lists so reply-all can address the whole conversation.
func allAddresses(addrs []imap.Address) []domain.EmailAddress {
	out := make([]domain.EmailAddress, 0, len(addrs))
	for _, a := range addrs {
		email, err := domain.NewEmailAddress(a.Name, a.Mailbox+"@"+a.Host)
		if err != nil {
			continue
		}
		out = append(out, email)
	}
	return out
}

// buildMessage maps a FETCH buffer into a domain message summary.
func buildMessage(folderID string, buf *imapclient.FetchMessageBuffer) (domain.MessageSummary, error) {
	uid := uint32(buf.UID)
	in := domain.MessageSummaryInput{
		ID:       makeMessageID(folderID, uid),
		FolderID: folderID,
		UID:      uid,
		Size:     int(buf.RFC822Size),
		Flags:    mapFlags(buf.Flags),
	}
	if buf.Envelope != nil {
		in.Subject = buf.Envelope.Subject
		in.Date = buf.Envelope.Date
		in.MessageID = buf.Envelope.MessageID
		in.From = firstAddress(buf.Envelope.From)
		in.To = allAddresses(buf.Envelope.To)
		in.Cc = allAddresses(buf.Envelope.Cc)
	}
	return domain.NewMessageSummary(in)
}

// blankLines collapses three or more consecutive newlines down to two.
var blankLines = regexp.MustCompile(`\n{3,}`)

// parseBody parses a raw RFC 5322 message into its plain-text and HTML bodies. When the message has
// only an HTML body, a plain-text rendering is derived from it so the message is always readable.
func parseBody(raw []byte) (plain, htmlBody string, err error) {
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return "", "", fmt.Errorf("imap: parse message: %w", err)
	}
	var plainBuf, htmlBuf strings.Builder
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", fmt.Errorf("imap: read part: %w", err)
		}
		inline, ok := part.Header.(*mail.InlineHeader)
		if !ok {
			continue // an attachment; not part of the readable body
		}
		content, err := io.ReadAll(part.Body)
		if err != nil {
			return "", "", fmt.Errorf("imap: read part body: %w", err)
		}
		contentType, _, _ := inline.ContentType()
		if contentType == "text/html" {
			htmlBuf.Write(content)
		} else {
			plainBuf.Write(content)
		}
	}
	plain = plainBuf.String()
	htmlBody = htmlBuf.String()
	if htmlBody != "" {
		htmlBody = htmlSanitizer.Sanitize(blockRemoteImages(htmlBody))
	}
	if strings.TrimSpace(plain) == "" && htmlBody != "" {
		plain = htmlToText(htmlBody)
	}
	return plain, htmlBody, nil
}

// blockRemoteImages rewrites every <img src> to a data attribute so remote images do not load
// automatically. Auto-loading a remote image leaks that the reader opened the message (and their IP)
// to the sender, so the source is parked until the reader explicitly asks to load images. srcset is
// dropped for the same reason. On a parse or render failure the original HTML is returned unchanged;
// the sanitizer still runs over it afterwards.
func blockRemoteImages(source string) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			parkImageSource(n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
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

// parkImageSource renames an image's src attribute to the blocked-image data attribute and drops
// srcset, so the browser has nothing to fetch until the source is restored.
func parkImageSource(n *html.Node) {
	kept := n.Attr[:0]
	for _, attr := range n.Attr {
		switch strings.ToLower(attr.Key) {
		case "src":
			attr.Key = blockedImageAttr
			kept = append(kept, attr)
		case "srcset":
			// Dropped: srcset is another way to trigger a remote fetch.
		default:
			kept = append(kept, attr)
		}
	}
	n.Attr = kept
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

func hasAttr(attrs []imap.MailboxAttr, want imap.MailboxAttr) bool {
	for _, attr := range attrs {
		if attr == want {
			return true
		}
	}
	return false
}
