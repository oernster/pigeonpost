package main

// OutboxItemDTO is the front-end view of one queued outgoing operation waiting to be sent. The account
// id lets the front end show the queue as a per-account Outbox folder; the body lets it preview the
// queued message without a separate fetch.
type OutboxItemDTO struct {
	ID        string   `json:"id"`
	AccountID string   `json:"accountId"`
	Kind      string   `json:"kind"`
	Subject   string   `json:"subject"`
	To        []string `json:"to"`
	Body      string   `json:"body"`
	CreatedMs int64    `json:"createdMs"`
	Failed    bool     `json:"failed"`
	Failure   string   `json:"failure"`
}

// ListOutbox returns the queued outgoing operations, oldest first, for the outbox view.
func (a *App) ListOutbox() ([]OutboxItemDTO, error) {
	items, err := a.compose.OutboxItems(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OutboxItemDTO, 0, len(items))
	for _, item := range items {
		msg := item.Message()
		to := make([]string, 0, len(msg.To()))
		for _, addr := range msg.To() {
			to = append(to, addr.Address())
		}
		out = append(out, OutboxItemDTO{
			ID:        item.ID(),
			AccountID: item.AccountID(),
			Kind:      item.Kind().String(),
			Subject:   msg.Subject(),
			To:        to,
			Body:      msg.Body(),
			CreatedMs: item.CreatedAt().UnixMilli(),
			Failed:    item.Failed(),
			Failure:   item.Failure(),
		})
	}
	return out, nil
}

// CancelOutboxItem discards a queued outgoing operation before it is sent.
func (a *App) CancelOutboxItem(id string) error {
	return a.compose.CancelOutbox(a.ctx, id)
}
