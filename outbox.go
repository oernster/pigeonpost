package main

import "github.com/oernster/pigeonpost/internal/domain"

// OutboxItemDTO is the front-end view of one queued outgoing operation waiting to be sent. The account
// id lets the front end show the queue as a per-account Outbox folder; the body lets it preview the
// queued message without a separate fetch. HoldMs is the end of the item's send hold in Unix
// milliseconds; 0 means the item carries no hold. Attachments names the files the queued message carries,
// so the Outbox row can show it has them before it goes.
type OutboxItemDTO struct {
	ID          string   `json:"id"`
	AccountID   string   `json:"accountId"`
	Kind        string   `json:"kind"`
	Subject     string   `json:"subject"`
	To          []string `json:"to"`
	Body        string   `json:"body"`
	CreatedMs   int64    `json:"createdMs"`
	HoldMs      int64    `json:"holdMs"`
	Failed      bool     `json:"failed"`
	Failure     string   `json:"failure"`
	Attachments []string `json:"attachments"`
}

// ListOutbox returns the queued outgoing operations, oldest first, for the outbox view.
func (a *App) ListOutbox() ([]OutboxItemDTO, error) {
	items, err := a.compose.OutboxItems(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OutboxItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toOutboxItemDTO(item))
	}
	return out, nil
}

// toOutboxItemDTO maps one queued operation to its wire form. The lists are never nil, so the front end
// always receives arrays.
func toOutboxItemDTO(item domain.OutboxItem) OutboxItemDTO {
	msg := item.Message()
	to := make([]string, 0, len(msg.To()))
	for _, addr := range msg.To() {
		to = append(to, addr.Address())
	}
	attachments := make([]string, 0, len(msg.Attachments()))
	for _, attachment := range msg.Attachments() {
		attachments = append(attachments, attachment.Filename())
	}
	return OutboxItemDTO{
		ID:          item.ID(),
		AccountID:   item.AccountID(),
		Kind:        item.Kind().String(),
		Subject:     msg.Subject(),
		To:          to,
		Body:        msg.Body(),
		CreatedMs:   item.CreatedAt().UnixMilli(),
		HoldMs:      holdUntilMillisDTO(item),
		Failed:      item.Failed(),
		Failure:     item.Failure(),
		Attachments: attachments,
	}
}

// holdUntilMillisDTO maps an item's send hold to its wire form: 0 for none.
func holdUntilMillisDTO(item domain.OutboxItem) int64 {
	if item.HoldUntil().IsZero() {
		return 0
	}
	return item.HoldUntil().UnixMilli()
}

// CancelOutboxItem discards a queued outgoing operation before it is sent. It reports whether the item
// was still queued: false means a send had already claimed it (it is on its way or has left), so the
// front end says the cancel came too late instead of pretending the send was stopped.
func (a *App) CancelOutboxItem(id string) (bool, error) {
	return a.compose.CancelOutbox(a.ctx, id)
}
