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

func TestBuildMessageDecodesEncodedWordSubjectAndName(t *testing.T) {
	// A real Windows-1252 Q-encoded subject and sender name (the =A3 is a pound sign) must decode to
	// readable text rather than show their raw =?...?= encoding in the list.
	buf := &imapclient.FetchMessageBuffer{
		UID:        imap.UID(1),
		RFC822Size: 100,
		Envelope: &imap.Envelope{
			Subject: "=?Windows-1252?Q?Re:_Systems_Developer_opportunity_-_circa_=A390k_-_Hybrid?=",
			From: []imap.Address{{
				Name:    "=?utf-8?Q?Andr=C3=A9_Mercer?=",
				Mailbox: "andre", Host: "example.com",
			}},
		},
	}
	msg, err := buildMessage("f1", buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Subject() != "Re: Systems Developer opportunity - circa £90k - Hybrid" {
		t.Errorf("subject not decoded: %q", msg.Subject())
	}
	if msg.From().Display() != "André Mercer" {
		t.Errorf("sender name not decoded: %q", msg.From().Display())
	}
}

func attachmentPartFixture(mediaType string, value string) *imap.BodyStructureSinglePart {
	slash := strings.IndexByte(mediaType, '/')
	part := &imap.BodyStructureSinglePart{Type: mediaType[:slash], Subtype: mediaType[slash+1:]}
	if value != "" {
		part.Extended = &imap.BodyStructureSinglePartExt{
			Disposition: &imap.BodyStructureDisposition{Value: value, Params: map[string]string{"filename": "file"}},
		}
	}
	return part
}

func TestHasAttachment(t *testing.T) {
	textPart := attachmentPartFixture("text/plain", "")
	pdfPart := attachmentPartFixture("application/pdf", "attachment")
	inlineImage := attachmentPartFixture("image/png", "inline")
	calendarPart := attachmentPartFixture("text/calendar", "attachment")

	cases := map[string]struct {
		bs   imap.BodyStructure
		want bool
	}{
		"nil structure":         {nil, false},
		"plain single part":     {textPart, false},
		"attachment in mixed":   {&imap.BodyStructureMultiPart{Subtype: "mixed", Children: []imap.BodyStructure{textPart, pdfPart}}, true},
		"text only alternative": {&imap.BodyStructureMultiPart{Subtype: "alternative", Children: []imap.BodyStructure{textPart}}, false},
		"inline image only":     {&imap.BodyStructureMultiPart{Subtype: "related", Children: []imap.BodyStructure{textPart, inlineImage}}, false},
		"calendar not counted":  {&imap.BodyStructureMultiPart{Subtype: "mixed", Children: []imap.BodyStructure{textPart, calendarPart}}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := hasAttachment(tc.bs); got != tc.want {
				t.Errorf("hasAttachment = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildMessageSetsHasAttachments(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		UID:        imap.UID(3),
		RFC822Size: 100,
		Envelope:   &imap.Envelope{Subject: "With file"},
		BodyStructure: &imap.BodyStructureMultiPart{Subtype: "mixed", Children: []imap.BodyStructure{
			attachmentPartFixture("text/plain", ""),
			attachmentPartFixture("application/pdf", "attachment"),
		}},
	}
	msg, err := buildMessage("f1", buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !msg.HasAttachments() {
		t.Error("a message with an attachment part should report HasAttachments")
	}
}

func TestBuildMessageUnescapesSubjectEntities(t *testing.T) {
	// A sender that builds the subject from an HTML template leaves entities in it; they must show as the
	// real characters rather than "&amp;" in the list.
	buf := &imapclient.FetchMessageBuffer{
		UID:        imap.UID(2),
		RFC822Size: 100,
		Envelope: &imap.Envelope{
			Subject: "We've sent your application to Harnham - Data &amp; Analytics Recruitment",
		},
	}
	msg, err := buildMessage("f1", buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Subject() != "We've sent your application to Harnham - Data & Analytics Recruitment" {
		t.Errorf("subject entity not unescaped: %q", msg.Subject())
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

func TestBuildMessageUIDIsDecimalString(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{UID: imap.UID(123), RFC822Size: 1}
	msg, err := buildMessage("f1", buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.UID() != "123" {
		t.Errorf("UID = %q, want the decimal string 123", msg.UID())
	}
}
