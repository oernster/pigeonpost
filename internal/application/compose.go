package application

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	account, msg, err := s.buildOutgoing(ctx, accountID, draft)
	if err != nil {
		return err
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

// HoldSend is Send behind an undo-send window: the validated message is queued in the outbox with a
// hold of holdFor from now and the queued item's id is returned, so the front end can offer Undo
// (which is CancelOutbox) until the hold elapses and the dispatcher sends it. A holdFor of zero or
// less means no window: the message sends immediately through Send and the returned id is empty.
func (s *ComposeService) HoldSend(ctx context.Context, accountID string, draft Draft, holdFor time.Duration) (string, error) {
	if holdFor <= 0 {
		return "", s.Send(ctx, accountID, draft)
	}
	_, msg, err := s.buildOutgoing(ctx, accountID, draft)
	if err != nil {
		return "", err
	}
	now := s.clock.Now()
	item, err := domain.NewOutboxItem(s.newID(), accountID, domain.OutboxSend, msg, now)
	if err != nil {
		return "", fmt.Errorf("compose: build held outbox item: %w", err)
	}
	item = item.WithHoldUntil(now.Add(holdFor))
	if err := s.outbox.EnqueueOutbox(ctx, item); err != nil {
		return "", fmt.Errorf("compose: queue held send: %w", err)
	}
	return item.ID(), nil
}

// buildOutgoing loads the account, resolves the chosen sender identity and builds the validated
// outgoing message, shared by the immediate and the held send paths.
func (s *ComposeService) buildOutgoing(ctx context.Context, accountID string, draft Draft) (domain.Account, domain.OutgoingMessage, error) {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return domain.Account{}, domain.OutgoingMessage{}, fmt.Errorf("compose: load account %q: %w", accountID, err)
	}
	from, ok := account.ResolveSender(draft.From)
	if !ok {
		return domain.Account{}, domain.OutgoingMessage{}, fmt.Errorf("compose: account %q send as %q: %w", accountID, draft.From, ErrUnknownSender)
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
		return domain.Account{}, domain.OutgoingMessage{}, fmt.Errorf("compose: build message: %w", err)
	}
	return account, msg, nil
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

// draftsPath returns the path of the account's Drafts mailbox, or ErrNoDraftsFolder when none exists.
func (s *ComposeService) draftsPath(ctx context.Context, accountID string) (string, error) {
	path, found, err := folderPathByKind(ctx, s.store, accountID, domain.FolderDrafts)
	if err != nil {
		return "", fmt.Errorf("compose: list folders for %q: %w", accountID, err)
	}
	if !found {
		return "", ErrNoDraftsFolder
	}
	return path, nil
}

// saveToSent appends a copy of a just-sent message to the account's Sent mailbox, so the user keeps a
// record of what they sent. It is best-effort: the message has already been delivered, so a provider
// that saves sent mail itself (skipped), a missing Sent folder or an append failure must never turn a
// successful send into a failure.
func (s *ComposeService) saveToSent(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) {
	if account.SavesSentServerSide() {
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
	path, _, err := folderPathByKind(ctx, s.store, accountID, domain.FolderSent)
	if err != nil {
		return ""
	}
	return path
}
