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
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO outbox (id, account_id, kind, from_display, from_address, to_json, cc_json,
		        subject, body, html_body, created_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		item.ID(), item.AccountID(), int(item.Kind()), display, address, toJSON, ccJSON,
		msg.Subject(), msg.Body(), msg.HTMLBody(), item.CreatedAt().UnixMilli()); err != nil {
		return fmt.Errorf("insert outbox item %q: %w", item.ID(), err)
	}
	return nil
}

// ListOutbox returns the queued operations, oldest first.
func (s *Store) ListOutbox(ctx context.Context) ([]domain.OutboxItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, kind, from_display, from_address, to_json, cc_json,
		        subject, body, html_body, created_ms
		 FROM outbox ORDER BY created_ms ASC, id ASC;`)
	if err != nil {
		return nil, fmt.Errorf("query outbox: %w", err)
	}
	defer rows.Close()

	var items []domain.OutboxItem
	for rows.Next() {
		item, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox: %w", err)
	}
	return items, nil
}

// DeleteOutbox removes a queued operation by id, after it has been replayed or dropped.
func (s *Store) DeleteOutbox(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM outbox WHERE id = ?;", id); err != nil {
		return fmt.Errorf("delete outbox item %q: %w", id, err)
	}
	return nil
}

func scanOutbox(row scanner) (domain.OutboxItem, error) {
	var (
		id, accountID            string
		kind                     int
		fromDisplay, fromAddress string
		toJSON, ccJSON           string
		subject, body, htmlBody  string
		createdMS                int64
	)
	if err := row.Scan(&id, &accountID, &kind, &fromDisplay, &fromAddress, &toJSON, &ccJSON,
		&subject, &body, &htmlBody, &createdMS); err != nil {
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

	in := domain.OutgoingMessageInput{
		From: from, To: to, Cc: cc, Subject: subject, Body: body, HTMLBody: htmlBody,
	}
	msg, err := buildOutboxMessage(domain.OutboxKind(kind), in)
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox message for %q: %w", id, err)
	}
	item, err := domain.NewOutboxItem(id, accountID, domain.OutboxKind(kind), msg, time.UnixMilli(createdMS).UTC())
	if err != nil {
		return domain.OutboxItem{}, fmt.Errorf("rebuild outbox item %q: %w", id, err)
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
