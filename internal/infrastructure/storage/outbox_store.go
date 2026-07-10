package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// addrJSON is the persisted form of one recipient in an outbox row's to/cc list.
type addrJSON struct {
	Display string `json:"display"`
	Address string `json:"address"`
}

// attachmentJSON is the persisted form of one attachment on a queued outbox item. Content is a byte
// slice, which encoding/json stores as base64, so the row is self-contained across a restart.
type attachmentJSON struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Content     []byte `json:"content"`
}

// EnqueueOutbox stores a queued outgoing operation. The message's recipient lists are serialised to
// JSON so the row is self-contained and can be replayed after a restart.
func (s *Store) EnqueueOutbox(ctx context.Context, item domain.OutboxItem) error {
	msg := item.Message()
	display, address := senderColumns(msg.From())
	toJSON, err := marshalAddrs(msg.To())
	if err != nil {
		return fmt.Errorf("encode outbox recipients: %w", err)
	}
	ccJSON, err := marshalAddrs(msg.Cc())
	if err != nil {
		return fmt.Errorf("encode outbox cc: %w", err)
	}
	bccJSON, err := marshalAddrs(msg.Bcc())
	if err != nil {
		return fmt.Errorf("encode outbox bcc: %w", err)
	}
	attachmentsJSON, err := marshalAttachments(msg.Attachments())
	if err != nil {
		return fmt.Errorf("encode outbox attachments: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO outbox (id, account_id, kind, from_display, from_address, to_json, cc_json,
		        bcc_json, subject, body, html_body, attachments_json, created_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		item.ID(), item.AccountID(), int(item.Kind()), display, address, toJSON, ccJSON,
		bccJSON, msg.Subject(), msg.Body(), msg.HTMLBody(), attachmentsJSON, item.CreatedAt().UnixMilli()); err != nil {
		return fmt.Errorf("insert outbox item %q: %w", item.ID(), err)
	}
	return nil
}

// ListOutbox returns the queued operations, oldest first.
func (s *Store) ListOutbox(ctx context.Context) ([]domain.OutboxItem, error) {
	return queryRows(ctx, s.db, "outbox",
		`SELECT id, account_id, kind, from_display, from_address, to_json, cc_json,
		        bcc_json, subject, body, html_body, attachments_json, created_ms, failure
		 FROM outbox ORDER BY created_ms ASC, id ASC;`, scanOutbox)
}

// DeleteOutbox removes a queued operation by id, after it has been replayed or dropped.
func (s *Store) DeleteOutbox(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM outbox WHERE id = ?;", id); err != nil {
		return fmt.Errorf("delete outbox item %q: %w", id, err)
	}
	return nil
}

// MarkOutboxFailed stamps a permanent-failure reason on a queued operation so it is kept in the outbox
// for the user to see rather than dropped after a replay that cannot succeed.
func (s *Store) MarkOutboxFailed(ctx context.Context, id, reason string) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE outbox SET failure = ? WHERE id = ?;", reason, id); err != nil {
		return fmt.Errorf("mark outbox item %q failed: %w", id, err)
	}
	return nil
}

func scanOutbox(row scanner) (domain.OutboxItem, error) {
	var (
		id, accountID            string
		kind                     int
		fromDisplay, fromAddress string
		toJSON, ccJSON, bccJSON  string
		subject, body, htmlBody  string
		attachmentsJSON          string
		createdMS                int64
		failure                  string
	)
	if err := row.Scan(&id, &accountID, &kind, &fromDisplay, &fromAddress, &toJSON, &ccJSON,
		&bccJSON, &subject, &body, &htmlBody, &attachmentsJSON, &createdMS, &failure); err != nil {
		return domain.OutboxItem{}, fmt.Errorf("scan outbox item: %w", err)
	}

	var from domain.EmailAddress
	if fromAddress != "" {
		parsed, err := domain.NewEmailAddress(fromDisplay, fromAddress)
		if err != nil {
			return domain.OutboxItem{}, fmt.Errorf("rebuild outbox sender for %q: %w", id, err)
		}
		from = parsed
	}
	to, err := unmarshalAddrs(toJSON)
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox recipients for %q: %w", id, err)
	}
	cc, err := unmarshalAddrs(ccJSON)
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox cc for %q: %w", id, err)
	}
	bcc, err := unmarshalAddrs(bccJSON)
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox bcc for %q: %w", id, err)
	}
	attachments, err := unmarshalAttachments(attachmentsJSON)
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox attachments for %q: %w", id, err)
	}

	in := domain.OutgoingMessageInput{
		From: from, To: to, Cc: cc, Bcc: bcc, Subject: subject, Body: body, HTMLBody: htmlBody,
		Attachments: attachments,
	}
	msg, err := buildOutboxMessage(domain.OutboxKind(kind), in)
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox message for %q: %w", id, err)
	}
	item, err := domain.NewOutboxItem(id, accountID, domain.OutboxKind(kind), msg, time.UnixMilli(createdMS).UTC())
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox item %q: %w", id, err)
	}
	if failure != "" {
		item = item.WithFailure(failure)
	}
	return item, nil
}

// buildOutboxMessage reconstructs the queued message. A draft may be incomplete (no recipients) so it
// uses the lenient constructor; a send is rebuilt with the strict one.
func buildOutboxMessage(kind domain.OutboxKind, in domain.OutgoingMessageInput) (domain.OutgoingMessage, error) {
	if kind == domain.OutboxDraft {
		return domain.NewDraftMessage(in)
	}
	return domain.NewOutgoingMessage(in)
}

func marshalAddrs(addrs []domain.EmailAddress) (string, error) {
	out := make([]addrJSON, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, addrJSON{Display: a.Display(), Address: a.Address()})
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalAddrs(raw string) ([]domain.EmailAddress, error) {
	var stored []addrJSON
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	out := make([]domain.EmailAddress, 0, len(stored))
	for _, a := range stored {
		addr, err := domain.NewEmailAddress(a.Display, a.Address)
		if err != nil {
			return nil, err
		}
		out = append(out, addr)
	}
	return out, nil
}

func marshalAttachments(attachments []domain.Attachment) (string, error) {
	out := make([]attachmentJSON, 0, len(attachments))
	for _, a := range attachments {
		out = append(out, attachmentJSON{Filename: a.Filename(), ContentType: a.ContentType(), Content: a.Content()})
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalAttachments(raw string) ([]domain.Attachment, error) {
	var stored []attachmentJSON
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	out := make([]domain.Attachment, 0, len(stored))
	for _, a := range stored {
		attachment, err := domain.NewAttachment(a.Filename, a.ContentType, a.Content)
		if err != nil {
			return nil, err
		}
		out = append(out, attachment)
	}
	return out, nil
}
