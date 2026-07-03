package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Draft is the user-supplied content of a message to send. The sender is taken from the account, so
// it is not part of the draft.
type Draft struct {
	To       []domain.EmailAddress
	Cc       []domain.EmailAddress
	Bcc      []domain.EmailAddress
	Subject  string
	Body     string
	HTMLBody string
}

// IDGenerator produces a unique identifier for a queued outbox item.
type IDGenerator func() string

// ComposeService is the use-case boundary for outgoing mail: sending messages, saving drafts, and the
// offline outbox that holds those operations when the server is unreachable and replays them later.
type ComposeService struct {
	accounts  AccountStore
	store     MailStore
	transport MailTransport
	drafts    DraftSaver
	outbox    OutboxStore
	clock     domain.Clock
	newID     IDGenerator
}

// NewComposeService constructs the service with its injected dependencies. The clock and id generator
// stamp queued outbox items; the outbox store persists them across restarts.
func NewComposeService(
	accounts AccountStore,
	store MailStore,
	transport MailTransport,
	drafts DraftSaver,
	outbox OutboxStore,
	clock domain.Clock,
	newID IDGenerator,
) *ComposeService {
	return &ComposeService{
		accounts:  accounts,
		store:     store,
		transport: transport,
		drafts:    drafts,
		outbox:    outbox,
		clock:     clock,
		newID:     newID,
	}
}

// Send builds a validated message from the draft, using the account's address as the sender, and
// hands it to the transport. When the server is unreachable the message is queued in the outbox
// instead of failing, and Send returns nil: the message will be delivered on the next replay.
func (s *ComposeService) Send(ctx context.Context, accountID string, draft Draft) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("compose: load account %q: %w", accountID, err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:     account.Address(),
		To:       draft.To,
		Cc:       draft.Cc,
		Bcc:      draft.Bcc,
		Subject:  draft.Subject,
		Body:     draft.Body,
		HTMLBody: draft.HTMLBody,
	})
	if err != nil {
		return fmt.Errorf("compose: build message: %w", err)
	}
	if err := s.transport.Send(ctx, account, msg); err != nil {
		if errors.Is(err, domain.ErrOffline) {
			return s.enqueue(ctx, accountID, domain.OutboxSend, msg)
		}
		return fmt.Errorf("compose: send: %w", err)
	}
	return nil
}

// SaveDraft stores an in-progress message in the account's Drafts mailbox on the server. Unlike Send,
// it accepts an incomplete message (no recipients, empty body). When the server is unreachable the
// draft is queued in the outbox and SaveDraft returns nil. It fails with ErrNoDraftsFolder when the
// account has no Drafts mailbox.
func (s *ComposeService) SaveDraft(ctx context.Context, accountID string, draft Draft) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("compose: load account %q: %w", accountID, err)
	}
	draftsPath, err := s.draftsPath(ctx, accountID)
	if err != nil {
		return err
	}
	msg, err := domain.NewDraftMessage(domain.OutgoingMessageInput{
		From:     account.Address(),
		To:       draft.To,
		Cc:       draft.Cc,
		Bcc:      draft.Bcc,
		Subject:  draft.Subject,
		Body:     draft.Body,
		HTMLBody: draft.HTMLBody,
	})
	if err != nil {
		return fmt.Errorf("compose: build draft: %w", err)
	}
	if err := s.drafts.SaveDraft(ctx, account, draftsPath, msg); err != nil {
		if errors.Is(err, domain.ErrOffline) {
			return s.enqueue(ctx, accountID, domain.OutboxDraft, msg)
		}
		return fmt.Errorf("compose: save draft: %w", err)
	}
	return nil
}

// PendingOutbox returns the number of operations currently queued for replay.
func (s *ComposeService) PendingOutbox(ctx context.Context) (int, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return 0, fmt.Errorf("compose: list outbox: %w", err)
	}
	return len(items), nil
}

// ReplayOutbox attempts every queued operation, oldest first. A successful operation is removed from
// the queue. If the server is still unreachable, replay stops and the remaining items stay queued. An
// operation that fails for any other reason (the account is gone, the message is rejected) is removed
// so it cannot wedge the queue, and its error is collected and returned. It returns how many
// operations succeeded.
func (s *ComposeService) ReplayOutbox(ctx context.Context) (int, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return 0, fmt.Errorf("compose: list outbox: %w", err)
	}
	replayed := 0
	var failures []error
	for _, item := range items {
		err := s.replayItem(ctx, item)
		if errors.Is(err, domain.ErrOffline) {
			return replayed, nil
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("compose: drop outbox item %q: %w", item.ID(), err))
		} else {
			replayed++
		}
		if delErr := s.outbox.DeleteOutbox(ctx, item.ID()); delErr != nil {
			return replayed, fmt.Errorf("compose: remove replayed item %q: %w", item.ID(), delErr)
		}
	}
	return replayed, errors.Join(failures...)
}

// replayItem performs one queued operation against the server, dispatching on its kind.
func (s *ComposeService) replayItem(ctx context.Context, item domain.OutboxItem) error {
	account, err := s.accounts.GetAccount(ctx, item.AccountID())
	if err != nil {
		return fmt.Errorf("load account %q: %w", item.AccountID(), err)
	}
	switch item.Kind() {
	case domain.OutboxDraft:
		draftsPath, err := s.draftsPath(ctx, item.AccountID())
		if err != nil {
			return err
		}
		return s.drafts.SaveDraft(ctx, account, draftsPath, item.Message())
	default:
		return s.transport.Send(ctx, account, item.Message())
	}
}

// enqueue records an outgoing operation in the outbox, stamped with a fresh id and the current time.
func (s *ComposeService) enqueue(ctx context.Context, accountID string, kind domain.OutboxKind, msg domain.OutgoingMessage) error {
	item, err := domain.NewOutboxItem(s.newID(), accountID, kind, msg, s.clock.Now())
	if err != nil {
		return fmt.Errorf("compose: build outbox item: %w", err)
	}
	if err := s.outbox.EnqueueOutbox(ctx, item); err != nil {
		return fmt.Errorf("compose: queue outbox item: %w", err)
	}
	return nil
}

// draftsPath returns the path of the account's Drafts mailbox, or ErrNoDraftsFolder when none exists.
func (s *ComposeService) draftsPath(ctx context.Context, accountID string) (string, error) {
	folders, err := s.store.ListFolders(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("compose: list folders for %q: %w", accountID, err)
	}
	for _, folder := range folders {
		if folder.Kind() == domain.FolderDrafts {
			return folder.Path(), nil
		}
	}
	return "", ErrNoDraftsFolder
}
