package domain

import (
	"strings"
	"time"
)

// DraftRecovery is a verbatim snapshot of an in-progress compose window, held locally so a message
// still being written survives an accidental close or a crash. Unlike an OutboxItem it is not bound for
// the server: its recipients are kept as the raw, possibly incomplete text the user has typed rather
// than validated addresses, so a half-finished address is preserved instead of rejected. Only one
// snapshot is kept at a time, the most recently autosaved compose.
type DraftRecovery struct {
	accountID string
	to        string
	cc        string
	bcc       string
	subject   string
	bodyHTML  string
	savedAt   time.Time
}

// DraftRecoveryInput carries the raw compose fields to snapshot. Each recipient field is the text as
// typed (comma-separated addresses), not a parsed list.
type DraftRecoveryInput struct {
	AccountID string
	To        string
	Cc        string
	Bcc       string
	Subject   string
	BodyHTML  string
}

// NewDraftRecovery validates and constructs a snapshot. It requires the owning account, so the restored
// draft is composed from the right identity; every content field is optional, because the point is to
// preserve whatever the user had typed, however incomplete. The saved time is passed in (from the
// injected clock at the call site) rather than read here, keeping the domain free of the wall clock.
func NewDraftRecovery(in DraftRecoveryInput, savedAt time.Time) (DraftRecovery, error) {
	accountID := strings.TrimSpace(in.AccountID)
	if accountID == "" {
		return DraftRecovery{}, ErrEmptyAccountID
	}
	return DraftRecovery{
		accountID: accountID,
		to:        in.To,
		cc:        in.Cc,
		bcc:       in.Bcc,
		subject:   in.Subject,
		bodyHTML:  in.BodyHTML,
		savedAt:   savedAt,
	}, nil
}

// AccountID returns the identifier of the account the draft is composed from.
func (d DraftRecovery) AccountID() string { return d.accountID }

// To returns the raw To field text as the user typed it.
func (d DraftRecovery) To() string { return d.to }

// Cc returns the raw Cc field text.
func (d DraftRecovery) Cc() string { return d.cc }

// Bcc returns the raw Bcc field text.
func (d DraftRecovery) Bcc() string { return d.bcc }

// Subject returns the subject line.
func (d DraftRecovery) Subject() string { return d.subject }

// BodyHTML returns the rich-text body as HTML.
func (d DraftRecovery) BodyHTML() string { return d.bodyHTML }

// SavedAt returns the time the snapshot was taken.
func (d DraftRecovery) SavedAt() time.Time { return d.savedAt }
