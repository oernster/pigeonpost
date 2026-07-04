package imap

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

func TestMakeIdentifiers(t *testing.T) {
	fid := makeFolderID("a1", "INBOX/Work")
	if !strings.HasPrefix(fid, "a1") || !strings.Contains(fid, "INBOX/Work") {
		t.Errorf("folder id = %q", fid)
	}
	mid := makeMessageID(fid, "42")
	if !strings.HasPrefix(mid, fid) || !strings.HasSuffix(mid, "42") {
		t.Errorf("message id = %q", mid)
	}
}

func TestFolderKindFor(t *testing.T) {
	cases := []struct {
		name    string
		mailbox string
		attrs   []imap.MailboxAttr
		want    domain.FolderKind
	}{
		{"inbox by name", "INBOX", nil, domain.FolderInbox},
		{"inbox case-insensitive", "inbox", nil, domain.FolderInbox},
		{"sent", "Sent", []imap.MailboxAttr{imap.MailboxAttrSent}, domain.FolderSent},
		{"drafts", "Drafts", []imap.MailboxAttr{imap.MailboxAttrDrafts}, domain.FolderDrafts},
		{"trash", "Trash", []imap.MailboxAttr{imap.MailboxAttrTrash}, domain.FolderTrash},
		{"junk", "Spam", []imap.MailboxAttr{imap.MailboxAttrJunk}, domain.FolderJunk},
		{"archive", "Archive", []imap.MailboxAttr{imap.MailboxAttrArchive}, domain.FolderArchive},
		{"custom no attrs", "Work", nil, domain.FolderCustom},
		{"custom other attr", "Work", []imap.MailboxAttr{imap.MailboxAttrHasChildren}, domain.FolderCustom},
		{"trash by name", "Trash", nil, domain.FolderTrash},
		{"deleted items by name", "Deleted Items", nil, domain.FolderTrash},
		{"spam by name", "spam", nil, domain.FolderJunk},
		{"sent by name", "Sent Items", nil, domain.FolderSent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := folderKindFor(tc.mailbox, tc.mailbox, tc.attrs); got != tc.want {
				t.Errorf("kind = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildFolder(t *testing.T) {
	data := &imap.ListData{Mailbox: "INBOX/Projects", Attrs: []imap.MailboxAttr{imap.MailboxAttrArchive}}
	folder, err := buildFolder("a1", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.Path() != "INBOX/Projects" {
		t.Errorf("path = %q", folder.Path())
	}
	if folder.Kind() != domain.FolderArchive {
		t.Errorf("kind = %v", folder.Kind())
	}
	if folder.AccountID() != "a1" {
		t.Errorf("accountID = %q", folder.AccountID())
	}
}

func TestMapFlags(t *testing.T) {
	flags := mapFlags([]imap.Flag{imap.FlagSeen, imap.FlagFlagged, imap.FlagAnswered, imap.FlagDraft, imap.FlagDeleted, imap.FlagJunk})
	for _, want := range []domain.Flag{domain.FlagSeen, domain.FlagFlagged, domain.FlagAnswered, domain.FlagDraft, domain.FlagDeleted} {
		if !flags.Has(want) {
			t.Errorf("expected flag %d set", want)
		}
	}
	// Unknown flags (Junk) are ignored, not mapped onto an unrelated bit.
	empty := mapFlags(nil)
	if empty.Raw() != 0 {
		t.Errorf("empty flags = %d", empty.Raw())
	}
}

func TestFirstAddress(t *testing.T) {
	if !firstAddress(nil).IsZero() {
		t.Error("empty address list should map to zero")
	}
	valid := firstAddress([]imap.Address{{Name: "Alice", Mailbox: "alice", Host: "example.com"}})
	if valid.Address() != "alice@example.com" || valid.Display() != "Alice" {
		t.Errorf("valid address = %q display %q", valid.Address(), valid.Display())
	}
	invalid := firstAddress([]imap.Address{{Name: "Bad", Mailbox: "bad", Host: ""}})
	if !invalid.IsZero() {
		t.Error("unparseable address should map to zero, not error")
	}
}

func TestBuildMessageWithEnvelope(t *testing.T) {
	when := time.Date(2026, time.July, 3, 10, 0, 0, 0, time.UTC)
	buf := &imapclient.FetchMessageBuffer{
		UID:        imap.UID(77),
		RFC822Size: 4096,
		Flags:      []imap.Flag{imap.FlagSeen},
		Envelope: &imap.Envelope{
			Subject:   "Weekly update",
			Date:      when,
			MessageID: "abc@example.com",
			From:      []imap.Address{{Name: "Bob", Mailbox: "bob", Host: "example.com"}},
		},
	}
	msg, err := buildMessage("f1", buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.UID() != "77" || msg.Subject() != "Weekly update" || msg.Size() != 4096 {
		t.Errorf("unexpected message: %+v", msg)
	}
	if msg.From().Address() != "bob@example.com" {
		t.Errorf("from = %q", msg.From().Address())
	}
	if !msg.IsRead() || !msg.Date().Equal(when) || msg.MessageID() != "abc@example.com" {
		t.Errorf("envelope fields lost: read=%v date=%v id=%q", msg.IsRead(), msg.Date(), msg.MessageID())
	}
}

func TestBuildMessageWithoutEnvelope(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{UID: imap.UID(5), RFC822Size: 10}
	msg, err := buildMessage("f1", buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Subject() != "" || !msg.From().IsZero() || msg.IsRead() {
		t.Errorf("expected empty summary, got %+v", msg)
	}
}

func TestHasAttr(t *testing.T) {
	attrs := []imap.MailboxAttr{imap.MailboxAttrNoSelect, imap.MailboxAttrHasChildren}
	if !hasAttr(attrs, imap.MailboxAttrNoSelect) {
		t.Error("expected NoSelect found")
	}
	if hasAttr(attrs, imap.MailboxAttrSent) {
		t.Error("did not expect Sent")
	}
}

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

	plain, html, err := parseBody([]byte(raw))
	if err != nil {
		t.Fatalf("parseBody: %v", err)
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

	plain, html, err := parseBody([]byte(raw))
	if err != nil {
		t.Fatalf("parseBody: %v", err)
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

	_, html, err := parseBody([]byte(raw))
	if err != nil {
		t.Fatalf("parseBody: %v", err)
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

func TestParseBodyBlocksRemoteImages(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		`<p>Hello</p><img src="http://tracker.example/pixel.gif" srcset="http://tracker.example/2x.gif 2x" alt="pic">` + "\r\n"

	_, html, err := parseBody([]byte(raw))
	if err != nil {
		t.Fatalf("parseBody: %v", err)
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
