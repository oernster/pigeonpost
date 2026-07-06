package message

import (
	"mime"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TestCalendarMessageWireIsValid renders a meeting-invite message and re-parses the wire bytes, asserting
// the multipart/mixed body carries a well-formed text/calendar part with the iTIP method parameter. A
// malformed part would be accepted by an SMTP server then dropped, which reads as "never received".
func TestCalendarMessageWireIsValid(t *testing.T) {
	from, _ := domain.NewEmailAddress("", "organizer@example.com")
	to, _ := domain.NewEmailAddress("", "guest@example.com")
	part, err := domain.NewCalendarPart(domain.MethodRequest,
		[]byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:uid-1\r\nSUMMARY:Sync\r\nATTENDEE:mailto:guest@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"))
	if err != nil {
		t.Fatalf("calendar part: %v", err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From: from, To: []domain.EmailAddress{to}, Subject: "Invitation: Sync",
		Body: "You are invited.", Calendar: part,
	})
	if err != nil {
		t.Fatalf("outgoing: %v", err)
	}

	wire := BuildMIME(msg, time.Unix(0, 0).UTC(), "mid-1")
	t.Logf("---- WIRE ----\n%s\n---- END ----", wire)

	// Split headers from body and parse the top-level multipart to confirm the calendar part is present.
	raw := string(wire)
	sep := strings.Index(raw, "\r\n\r\n")
	if sep < 0 {
		t.Fatal("no header/body separator")
	}
	headers := raw[:sep]
	ctLine := ""
	for _, line := range strings.Split(headers, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "content-type:") {
			ctLine = strings.TrimSpace(line[len("content-type:"):])
		}
	}
	mediaType, params, err := mime.ParseMediaType(ctLine)
	if err != nil {
		t.Fatalf("parse top content-type %q: %v", ctLine, err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("top media type = %q, want multipart/mixed", mediaType)
	}
	body := raw[sep+4:]
	mr := multipart.NewReader(strings.NewReader(body), params["boundary"])
	foundCalendar := false
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		pType, pParams, perr := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if perr != nil {
			t.Errorf("part content-type parse: %v (raw %q)", perr, p.Header.Get("Content-Type"))
			continue
		}
		if pType == "text/calendar" {
			foundCalendar = true
			if pParams["method"] != "REQUEST" {
				t.Errorf("text/calendar method param = %q, want REQUEST", pParams["method"])
			}
		}
	}
	if !foundCalendar {
		t.Error("no text/calendar part found in the rendered message")
	}
}
