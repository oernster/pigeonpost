package domain

import (
	"strings"
	"time"
)

// OutboxKind classifies a queued outgoing operation: a message to be sent, or a draft to be appended
// to the Drafts mailbox. Both carry an OutgoingMessage; the kind decides how it is replayed.
type OutboxKind int

const (
	// OutboxSend is a message queued for delivery via the outgoing (SMTP) server.
	OutboxSend OutboxKind = iota
	// OutboxDraft is a message queued for saving to the account's Drafts mailbox.
	OutboxDraft
)

// String returns a stable identifier for the kind.
func (k OutboxKind) String() string {
	switch k {
	case OutboxSend:
		return "send"
	case OutboxDraft:
		return "draft"
	default:
		return "unknown"
	}
}

// Valid reports whether the kind is one the outbox understands.
func (k OutboxKind) Valid() bool {
	return k == OutboxSend || k == OutboxDraft
}

// OutboxItem is a single outgoing operation held in the local queue: because the server was
// unreachable when it was requested (replayed, oldest first, once connectivity returns), or because it
// is inside its undo-send hold window (sent once the hold elapses, unless the user cancels it first).
type OutboxItem struct {
	id        string
	accountID string
	kind      OutboxKind
	message   OutgoingMessage
	createdAt time.Time
	holdUntil time.Time
	failure   string
}

// NewOutboxItem validates and constructs a queued item. The created time is passed in (via the
// injected clock at the call site) rather than read here, so the domain stays free of the wall clock.
func NewOutboxItem(id, accountID string, kind OutboxKind, message OutgoingMessage, createdAt time.Time) (OutboxItem, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return OutboxItem{}, ErrEmptyOutboxID
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return OutboxItem{}, ErrEmptyAccountID
	}
	if !kind.Valid() {
		return OutboxItem{}, ErrInvalidOutboxKind
	}
	if message.From().IsZero() {
		return OutboxItem{}, ErrNoSender
	}
	return OutboxItem{
		id:        id,
		accountID: accountID,
		kind:      kind,
		message:   message,
		createdAt: createdAt,
	}, nil
}

// ID returns the queue item identifier.
func (i OutboxItem) ID() string { return i.id }

// AccountID returns the owning account identifier.
func (i OutboxItem) AccountID() string { return i.accountID }

// Kind returns the operation kind.
func (i OutboxItem) Kind() OutboxKind { return i.kind }

// Message returns the queued outgoing message.
func (i OutboxItem) Message() OutgoingMessage { return i.message }

// CreatedAt returns the time the item was queued.
func (i OutboxItem) CreatedAt() time.Time { return i.createdAt }

// HoldUntil returns the end of the item's undo-send hold window, or the zero time when the item
// carries no hold (an offline-queued item, sent on the next replay).
func (i OutboxItem) HoldUntil() time.Time { return i.holdUntil }

// HeldAt reports whether the item is still inside its hold window at the given instant, meaning it
// must not be sent yet: the user may still undo it.
func (i OutboxItem) HeldAt(now time.Time) bool {
	return !i.holdUntil.IsZero() && now.Before(i.holdUntil)
}

// WithHoldUntil returns a copy of the item holding it until the given instant (the undo-send window).
// The zero time clears the hold, degrading the item to an ordinary queued operation.
func (i OutboxItem) WithHoldUntil(holdUntil time.Time) OutboxItem {
	i.holdUntil = holdUntil
	return i
}

// Failure returns the reason a permanent replay failure kept the item in the queue, or an empty string
// when the item has not failed. A permanently failed item (the account is gone, the message was
// rejected) is retained rather than dropped so the user can see it and act on it.
func (i OutboxItem) Failure() string { return i.failure }

// Failed reports whether the item carries a permanent-failure reason.
func (i OutboxItem) Failed() bool { return i.failure != "" }

// WithFailure returns a copy of the item marked with a permanent-failure reason. The receiver is
// unchanged, keeping the value immutable.
func (i OutboxItem) WithFailure(reason string) OutboxItem {
	i.failure = reason
	return i
}
