package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// outboxItemWith builds a queued send carrying the given attachments.
func outboxItemWith(t *testing.T, attachments ...domain.Attachment) domain.OutboxItem {
	t.Helper()
	from, err := domain.NewEmailAddress("", "me@example.com")
	if err != nil {
		t.Fatalf("from: %v", err)
	}
	to, err := domain.NewEmailAddress("", "peter@example.com")
	if err != nil {
		t.Fatalf("to: %v", err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From: from, To: []domain.EmailAddress{to}, Subject: "amusement", Body: "See attached",
		Attachments: attachments,
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	item, err := domain.NewOutboxItem("o1", "acct", domain.OutboxSend, msg, time.UnixMilli(0).UTC())
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	return item
}

// The Outbox row once always said a queued message had no attachments, so a scheduled send that had
// lost its file looked no different from one that still carried it.
func TestOutboxItemDTONamesTheQueuedAttachments(t *testing.T) {
	t.Parallel()
	photo, err := domain.NewAttachment("photo.jpg", "image/jpeg", []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	dto := toOutboxItemDTO(outboxItemWith(t, photo))
	if !reflect.DeepEqual(dto.Attachments, []string{"photo.jpg"}) {
		t.Errorf("attachments = %#v, want [photo.jpg]", dto.Attachments)
	}
	if !reflect.DeepEqual(dto.To, []string{"peter@example.com"}) || dto.Subject != "amusement" {
		t.Errorf("dto = %+v, want the recipient and subject carried through", dto)
	}
}

func TestOutboxItemDTOWithoutAttachmentsIsAnEmptyList(t *testing.T) {
	t.Parallel()
	// Empty rather than nil, so the front end receives [] and never has to coalesce a null.
	if got := toOutboxItemDTO(outboxItemWith(t)).Attachments; got == nil || len(got) != 0 {
		t.Errorf("attachments = %#v, want an empty list", got)
	}
}
