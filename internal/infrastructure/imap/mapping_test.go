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
