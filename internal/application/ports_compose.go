package application

import (
	"context"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The outgoing-mail ports: sending, the server-side Drafts and Sent copies, the offline outbox and the
// local compose recovery snapshot.

// MailTransport sends an outgoing message via an account's outgoing (SMTP) server.
type MailTransport interface {
	Send(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) error
}

// DraftSaver appends a message to an account's Drafts mailbox on the server, flagged \Draft, so the
// draft is available from any device. It is separate from MailTransport because saving a draft does
// not send anything.
type DraftSaver interface {
	SaveDraft(ctx context.Context, account domain.Account, draftsPath string, msg domain.OutgoingMessage) error
}

// SentSaver appends a copy of a sent message to an account's Sent mailbox, so the user keeps a record of
// what they sent on providers that do not save sent mail server-side. It is separate from DraftSaver
// because a sent copy is flagged \Seen, not \Draft.
type SentSaver interface {
	SaveSent(ctx context.Context, account domain.Account, sentPath string, msg domain.OutgoingMessage) error
}

// OutboxStore persists outgoing operations that could not reach the server because it was offline, so
// they survive a restart and can be replayed on reconnect. Items are listed oldest first.
type OutboxStore interface {
	EnqueueOutbox(ctx context.Context, item domain.OutboxItem) error
	ListOutbox(ctx context.Context) ([]domain.OutboxItem, error)
	// DeleteOutbox removes an item and reports whether one was actually removed: false means it was
	// already gone, which the cancel path surfaces as "the message had already left".
	DeleteOutbox(ctx context.Context, id string) (bool, error)
	MarkOutboxFailed(ctx context.Context, id, reason string) error
	// ClearOutboxHold removes an item's hold so it degrades to an ordinary queued operation.
	ClearOutboxHold(ctx context.Context, id string) error
	// NextOutboxHold returns the earliest hold among unfailed items and whether one exists.
	NextOutboxHold(ctx context.Context) (time.Time, bool, error)

	// The claim makes a send happen once. Before sending, a replay claims the item: one conditional write
	// moves it from queued to sending, so of every replay racing for it (two account syncs, a sync and the
	// send-later dispatcher) only the one whose claim succeeded sends. A cancel removes only an unclaimed
	// item, so it cannot report a message stopped that is already on the wire.

	// ClaimOutbox moves a queued, unfailed item to sending and reports whether this call did so.
	ClaimOutbox(ctx context.Context, id string) (bool, error)
	// ReleaseOutbox returns a claimed item to queued after a send that did not deliver.
	ReleaseOutbox(ctx context.Context, id string) error
	// CancelQueuedOutbox deletes an item only while it is queued and reports whether it did.
	CancelQueuedOutbox(ctx context.Context, id string) (bool, error)
	// RecoverOutboxClaims returns to queued the items an earlier run left sending, reporting how many.
	RecoverOutboxClaims(ctx context.Context) (int, error)
}

// DraftRecoveryStore persists a single local snapshot of an in-progress compose window, so a message
// still being written survives an accidental close or a crash. It never touches the server; it is the
// local-recovery counterpart to the server-side draft that DraftSaver appends. Only the most recent
// snapshot is kept: SaveDraftRecovery replaces any existing one; GetDraftRecovery reports whether a
// snapshot is present.
type DraftRecoveryStore interface {
	SaveDraftRecovery(ctx context.Context, recovery domain.DraftRecovery) error
	GetDraftRecovery(ctx context.Context) (domain.DraftRecovery, bool, error)
	ClearDraftRecovery(ctx context.Context) error
}
