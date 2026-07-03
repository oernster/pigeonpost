package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Draft is the user-supplied content of a message to send. The sender is taken from the account, so
// it is not part of the draft.
type Draft struct {
	To      []domain.EmailAddress
	Cc      []domain.EmailAddress
	Subject string
	Body    string
}

// ComposeService is the use-case boundary for sending mail.
type ComposeService struct {
	accounts  AccountStore
	transport MailTransport
}

// NewComposeService constructs the service with its injected dependencies.
func NewComposeService(accounts AccountStore, transport MailTransport) *ComposeService {
	return &ComposeService{accounts: accounts, transport: transport}
}

// Send builds a validated message from the draft, using the account's address as the sender, and
// hands it to the transport.
func (s *ComposeService) Send(ctx context.Context, accountID string, draft Draft) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("compose: load account %q: %w", accountID, err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:    account.Address(),
		To:      draft.To,
		Cc:      draft.Cc,
		Subject: draft.Subject,
		Body:    draft.Body,
	})
	if err != nil {
		return fmt.Errorf("compose: build message: %w", err)
	}
	if err := s.transport.Send(ctx, account, msg); err != nil {
		return fmt.Errorf("compose: send: %w", err)
	}
	return nil
}
