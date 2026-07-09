package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Draft is the user-supplied content of a message to send. From is the chosen sender address: empty means
// the account's primary address, otherwise it must be one of the account's identities (an alias it may
// send as), validated at send time.
type Draft struct {
	From        string
	To          []domain.EmailAddress
	Cc          []domain.EmailAddress
	Bcc         []domain.EmailAddress
	Subject     string
	Body        string
	HTMLBody    string
	Attachments []domain.Attachment
}

// IDGenerator produces a unique identifier for a queued outbox item.
type IDGenerator func() string

// ComposeService is the use-case boundary for outgoing mail: sending messages, saving drafts, the
// offline outbox that holds those operations when the server is unreachable and replays them later, and
// the local draft-recovery snapshot that guards an in-progress compose against an accidental close.
type ComposeService struct {
	accounts  AccountStore
	store     MailStore
	transport MailTransport
	drafts    DraftSaver
	sent      SentSaver
	outbox    OutboxStore
	recovery  DraftRecoveryStore
	clock     domain.Clock
	newID     IDGenerator
}

// NewComposeService constructs the service with its injected dependencies. The clock and id generator
// stamp queued outbox items; the outbox store persists them across restarts; the recovery store holds
// the single local snapshot of the compose window.
func NewComposeService(
	accounts AccountStore,
	store MailStore,
	transport MailTransport,
	drafts DraftSaver,
	sent SentSaver,
	outbox OutboxStore,
	recovery DraftRecoveryStore,
	clock domain.Clock,
	newID IDGenerator,
) *ComposeService {
	return &ComposeService{
		accounts:  accounts,
		store:     store,
		transport: transport,
		drafts:    drafts,
		sent:      sent,
		outbox:    outbox,
		recovery:  recovery,
		clock:     clock,
		newID:     newID,
	}
}

// DraftSnapshot is the raw content of an in-progress compose window, captured for local recovery. Its
// recipient fields are the text as typed, not parsed addresses, so an incomplete message is preserved.
type DraftSnapshot struct {
	AccountID string
	To        string
	Cc        string
	Bcc       string
	Subject   string
	BodyHTML  string
}

// SaveDraftRecovery stores a local snapshot of the in-progress compose, replacing any previous one, so
// the message survives an accidental close or a crash. It never touches the server.
func (s *ComposeService) SaveDraftRecovery(ctx context.Context, snapshot DraftSnapshot) error {
	recovery, err := domain.NewDraftRecovery(domain.DraftRecoveryInput{
		AccountID: snapshot.AccountID,
		To:        snapshot.To,
		Cc:        snapshot.Cc,
		Bcc:       snapshot.Bcc,
		Subject:   snapshot.Subject,
		BodyHTML:  snapshot.BodyHTML,
	}, s.clock.Now())
	if err != nil {
		return fmt.Errorf("compose: build draft recovery: %w", err)
	}
	if err := s.recovery.SaveDraftRecovery(ctx, recovery); err != nil {
		return fmt.Errorf("compose: save draft recovery: %w", err)
	}
	return nil
}

// DraftRecovery returns the locally held compose snapshot and whether one exists, so the front end can
// offer to restore an unsent message after a restart or an accidental close.
func (s *ComposeService) DraftRecovery(ctx context.Context) (domain.DraftRecovery, bool, error) {
	recovery, ok, err := s.recovery.GetDraftRecovery(ctx)
	if err != nil {
		return domain.DraftRecovery{}, false, fmt.Errorf("compose: get draft recovery: %w", err)
	}
	return recovery, ok, nil
}

// ClearDraftRecovery discards the local compose snapshot, called once the message is sent, saved to the
// server, or the user chooses not to restore it.
func (s *ComposeService) ClearDraftRecovery(ctx context.Context) error {
	if err := s.recovery.ClearDraftRecovery(ctx); err != nil {
		return fmt.Errorf("compose: clear draft recovery: %w", err)
	}
	return nil
}

// Send builds a validated message from the draft, using the account's address as the sender, and
// hands it to the transport. When the server is unreachable the message is queued in the outbox
// instead of failing, and Send returns nil: the message will be delivered on the next replay.
func (s *ComposeService) Send(ctx context.Context, accountID string, draft Draft) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("compose: load account %q: %w", accountID, err)
	}
	from, ok := account.ResolveSender(draft.From)
	if !ok {
		return fmt.Errorf("compose: account %q send as %q: %w", accountID, draft.From, ErrUnknownSender)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From:        from,
		To:          draft.To,
		Cc:          draft.Cc,
		Bcc:         draft.Bcc,
		Subject:     draft.Subject,
		Body:        draft.Body,
		HTMLBody:    draft.HTMLBody,
		Attachments: draft.Attachments,
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
	s.saveToSent(ctx, account, msg)
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
	from, ok := account.ResolveSender(draft.From)
	if !ok {
		return fmt.Errorf("compose: account %q send as %q: %w", accountID, draft.From, ErrUnknownSender)
	}
	msg, err := domain.NewDraftMessage(domain.OutgoingMessageInput{
		From:        from,
		To:          draft.To,
		Cc:          draft.Cc,
		Bcc:         draft.Bcc,
		Subject:     draft.Subject,
		Body:        draft.Body,
		HTMLBody:    draft.HTMLBody,
		Attachments: draft.Attachments,
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

// OutboxItems returns the queued outgoing operations, oldest first, so the user can review or cancel
// mail waiting to be sent.
func (s *ComposeService) OutboxItems(ctx context.Context) ([]domain.OutboxItem, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return nil, fmt.Errorf("compose: list outbox: %w", err)
	}
	return items, nil
}

// CancelOutbox removes a queued operation before it is sent, discarding it.
func (s *ComposeService) CancelOutbox(ctx context.Context, id string) error {
	if err := s.outbox.DeleteOutbox(ctx, id); err != nil {
		return fmt.Errorf("compose: cancel outbox item %q: %w", id, err)
	}
	return nil
}

// ReplayOutbox attempts every queued operation, oldest first. A successful operation is removed from
// the queue. If the server is still unreachable, replay stops and the remaining items stay queued. An
// operation that fails for any other reason (the account is gone, the message is rejected) is kept in
// the queue and stamped with its failure reason, so it surfaces in the outbox for the user to see and
// act on rather than vanishing. An item already marked failed is skipped, not retried. Its error is
// also collected and returned. It returns how many operations succeeded.
func (s *ComposeService) ReplayOutbox(ctx context.Context) (int, error) {
	items, err := s.outbox.ListOutbox(ctx)
	if err != nil {
		return 0, fmt.Errorf("compose: list outbox: %w", err)
	}
	replayed := 0
	var failures []error
	for _, item := range items {
		if item.Failed() {
			continue
		}
		err := s.replayItem(ctx, item)
		if errors.Is(err, domain.ErrOffline) {
			return replayed, nil
		}
		if err != nil {
			if markErr := s.outbox.MarkOutboxFailed(ctx, item.ID(), err.Error()); markErr != nil {
				return replayed, fmt.Errorf("compose: mark outbox item %q failed: %w", item.ID(), markErr)
			}
			failures = append(failures, fmt.Errorf("compose: outbox item %q failed: %w", item.ID(), err))
			continue
		}
		replayed++
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

// gmailSMTPHost is Gmail's outgoing server. Gmail saves sent mail to its Sent Mail folder server-side
// automatically, so PigeonPost must not also append a copy or the message would appear in Sent twice.
const gmailSMTPHost = "smtp.gmail.com"

// autoSavesSent reports whether the account's provider saves sent mail server-side, so the client must
// not append its own copy. Gmail is the provider PigeonPost offers that does this.
func autoSavesSent(account domain.Account) bool {
	return strings.EqualFold(account.Outgoing().Host(), gmailSMTPHost)
}

// saveToSent appends a copy of a just-sent message to the account's Sent mailbox, so the user keeps a
// record of what they sent. It is best-effort: the message has already been delivered, so a provider
// that saves sent mail itself (skipped), a missing Sent folder or an append failure must never turn a
// successful send into a failure.
func (s *ComposeService) saveToSent(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) {
	if autoSavesSent(account) {
		return
	}
	sentPath := s.sentPath(ctx, account.ID())
	if sentPath == "" {
		return
	}
	_ = s.sent.SaveSent(ctx, account, sentPath, msg)
}

// sentPath returns the path of the account's Sent mailbox. It returns an empty string when the folder
// list cannot be read or the account has no Sent folder (both meaning: skip the best-effort Sent copy).
func (s *ComposeService) sentPath(ctx context.Context, accountID string) string {
	folders, err := s.store.ListFolders(ctx, accountID)
	if err != nil {
		return ""
	}
	for _, folder := range folders {
		if folder.Kind() == domain.FolderSent {
			return folder.Path()
		}
	}
	return ""
}
