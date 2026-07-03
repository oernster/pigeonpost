package main

import (
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// ComposeRequest is the front-end payload for sending a message. Recipients are comma-free single
// addresses; the front end splits any user input into this list.
type ComposeRequest struct {
	AccountID string   `json:"accountId"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
	Bcc       []string `json:"bcc"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	HTMLBody  string   `json:"htmlBody"`
}

// SendMessage parses the request's addresses and sends the message through the compose use case.
func (a *App) SendMessage(req ComposeRequest) error {
	to, err := parseAddresses(req.To)
	if err != nil {
		return err
	}
	cc, err := parseAddresses(req.Cc)
	if err != nil {
		return err
	}
	bcc, err := parseAddresses(req.Bcc)
	if err != nil {
		return err
	}
	return a.compose.Send(a.ctx, req.AccountID, application.Draft{
		To:       to,
		Cc:       cc,
		Bcc:      bcc,
		Subject:  req.Subject,
		Body:     req.Body,
		HTMLBody: req.HTMLBody,
	})
}

// SaveDraft stores an in-progress message in the account's Drafts mailbox. The message may be
// incomplete: unlike SendMessage it does not require recipients.
func (a *App) SaveDraft(req ComposeRequest) error {
	to, err := parseAddresses(req.To)
	if err != nil {
		return err
	}
	cc, err := parseAddresses(req.Cc)
	if err != nil {
		return err
	}
	bcc, err := parseAddresses(req.Bcc)
	if err != nil {
		return err
	}
	return a.compose.SaveDraft(a.ctx, req.AccountID, application.Draft{
		To:       to,
		Cc:       cc,
		Bcc:      bcc,
		Subject:  req.Subject,
		Body:     req.Body,
		HTMLBody: req.HTMLBody,
	})
}

// OutboxCount returns the number of outgoing operations queued while the server was offline, awaiting
// replay. The front end shows this so the user knows mail is waiting to be sent.
func (a *App) OutboxCount() (int, error) {
	return a.compose.PendingOutbox(a.ctx)
}

// ReplayOutbox attempts to deliver every queued outgoing operation, oldest first, and returns how many
// succeeded. It is called after a successful sync, when connectivity has returned.
func (a *App) ReplayOutbox() (int, error) {
	return a.compose.ReplayOutbox(a.ctx)
}

func parseAddresses(values []string) ([]domain.EmailAddress, error) {
	out := make([]domain.EmailAddress, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		addr, err := domain.NewEmailAddress("", trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q: %w", trimmed, err)
		}
		out = append(out, addr)
	}
	return out, nil
}
