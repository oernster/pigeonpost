package pop3

import "testing"

func TestBuildSummaryFullHeader(t *testing.T) {
	header := "From: Bob <bob@example.com>\r\n" +
		"To: me@example.com\r\n" +
		"Cc: team@example.com\r\n" +
		"Subject: Weekly update\r\n" +
		"Message-Id: <abc@example.com>\r\n" +
		"Date: Fri, 03 Jul 2026 10:00:00 +0000\r\n" +
		"\r\n"

	msg, err := buildSummary("f1", "uidl-1", []byte(header), 2048)
	if err != nil {
		t.Fatalf("buildSummary: %v", err)
	}
	if msg.UID() != "uidl-1" {
		t.Errorf("UID = %q, want uidl-1", msg.UID())
	}
	if msg.ID() != "f1"+idSeparator+"uidl-1" {
		t.Errorf("ID = %q", msg.ID())
	}
	if msg.Subject() != "Weekly update" {
		t.Errorf("Subject = %q", msg.Subject())
	}
	if msg.From().Address() != "bob@example.com" {
		t.Errorf("From = %q", msg.From().Address())
	}
	if len(msg.To()) != 1 || msg.To()[0].Address() != "me@example.com" {
		t.Errorf("To = %+v", msg.To())
	}
	if len(msg.Cc()) != 1 || msg.Cc()[0].Address() != "team@example.com" {
		t.Errorf("Cc = %+v", msg.Cc())
	}
	if msg.Size() != 2048 {
		t.Errorf("Size = %d", msg.Size())
	}
	if msg.IsRead() {
		t.Error("a freshly fetched POP3 message should be unread")
	}
}

func TestBuildSummaryEncodedSubject(t *testing.T) {
	header := "From: a@b.com\r\n" +
		"Subject: =?utf-8?B?SGVsbG8gd29ybGQ=?=\r\n" +
		"\r\n"

	msg, err := buildSummary("f1", "uidl-2", []byte(header), 10)
	if err != nil {
		t.Fatalf("buildSummary: %v", err)
	}
	if msg.Subject() != "Hello world" {
		t.Errorf("decoded subject = %q, want Hello world", msg.Subject())
	}
}

func TestBuildSummaryMinimalHeader(t *testing.T) {
	msg, err := buildSummary("f1", "uidl-3", []byte("Subject: Bare\r\n\r\n"), 5)
	if err != nil {
		t.Fatalf("buildSummary: %v", err)
	}
	if !msg.From().IsZero() {
		t.Errorf("expected zero sender, got %q", msg.From().Address())
	}
	if len(msg.To()) != 0 || len(msg.Cc()) != 0 {
		t.Errorf("expected no recipients, got To=%+v Cc=%+v", msg.To(), msg.Cc())
	}
}

func TestNumberForUID(t *testing.T) {
	items := []UIDItem{{Number: 1, UID: "a"}, {Number: 7, UID: "b"}}
	if n, ok := numberForUID(items, "b"); !ok || n != 7 {
		t.Errorf("numberForUID(b) = %d, %v; want 7, true", n, ok)
	}
	if _, ok := numberForUID(items, "missing"); ok {
		t.Error("numberForUID(missing) should report not found")
	}
}

func TestInboxAndMessageIDs(t *testing.T) {
	folder := inboxID("acct")
	if folder != "acct"+idSeparator+inboxPath {
		t.Errorf("inboxID = %q", folder)
	}
	if got := makeMessageID(folder, "uidl"); got != folder+idSeparator+"uidl" {
		t.Errorf("makeMessageID = %q", got)
	}
}
