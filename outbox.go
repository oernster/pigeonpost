package main

// OutboxItemDTO is the front-end view of one queued outgoing operation waiting to be sent.
type OutboxItemDTO struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Subject   string   `json:"subject"`
	To        []string `json:"to"`
	CreatedMs int64    `json:"createdMs"`
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
			Kind:      item.Kind().String(),
			Subject:   msg.Subject(),
			To:        to,
			CreatedMs: item.CreatedAt().UnixMilli(),
		})
	}
	return out, nil
}

// CancelOutboxItem discards a queued outgoing operation before it is sent.
func (a *App) CancelOutboxItem(id string) error {
	return a.compose.CancelOutbox(a.ctx, id)
}
