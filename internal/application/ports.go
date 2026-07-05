package application

import (
	"context"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// AccountStore persists and retrieves accounts. Credentials are not part of this contract; they are
// held in the OS keychain and referenced separately.
type AccountStore interface {
	ListAccounts(ctx context.Context) ([]domain.Account, error)
	GetAccount(ctx context.Context, id string) (domain.Account, error)
	SaveAccount(ctx context.Context, account domain.Account) error
	DeleteAccount(ctx context.Context, id string) error
}

// CredentialStore reads, persists and removes an account's secret in the OS keychain. It is kept
// separate from AccountStore so secrets never travel through the account database.
type CredentialStore interface {
	Password(ctx context.Context, account domain.Account) (string, error)
	SetPassword(ctx context.Context, account domain.Account, secret string) error
	DeletePassword(ctx context.Context, account domain.Account) error
}

// AccountVerifier proves a candidate password against an account's incoming server before the account
// is persisted, so a misconfigured account fails at setup time rather than silently on first sync. The
// password is passed explicitly (not read from the keychain) so verification can run before anything
// is written, leaving a working account untouched when an edit is verified with a bad password.
type AccountVerifier interface {
	Verify(ctx context.Context, account domain.Account, password string) error
}

// MailStore is the local cache of folders and message summaries. The UI reads from here so it works
// offline; the sync service writes to it.
type MailStore interface {
	ListFolders(ctx context.Context, accountID string) ([]domain.Folder, error)
	SaveFolders(ctx context.Context, accountID string, folders []domain.Folder) error
	ListMessages(ctx context.Context, folderID string) ([]domain.MessageSummary, error)
	SaveMessages(ctx context.Context, folderID string, messages []domain.MessageSummary) error
	SetSeen(ctx context.Context, messageID string, seen bool) error
	SetFlagged(ctx context.Context, messageID string, flagged bool) error
	DeleteAccountData(ctx context.Context, accountID string) error
	GetMessage(ctx context.Context, messageID string) (domain.MessageSummary, error)
	GetFolder(ctx context.Context, folderID string) (domain.Folder, error)
	UnreadByAccount(ctx context.Context) (map[string]int, error)
	GetMessageBody(ctx context.Context, messageID string) (domain.MessageBody, error)
	SaveMessageBody(ctx context.Context, body domain.MessageBody) error
	SearchMessages(ctx context.Context, query string) ([]domain.MessageSummary, error)
	DeleteMessage(ctx context.Context, messageID string) error
}

// TagStore persists user-defined coloured tags and their many-to-many association with messages.
type TagStore interface {
	ListTags(ctx context.Context) ([]domain.Tag, error)
	SaveTag(ctx context.Context, tag domain.Tag) error
	DeleteTag(ctx context.Context, id string) error
	TagsForMessage(ctx context.Context, messageID string) ([]domain.Tag, error)
	AddMessageTag(ctx context.Context, messageID, tagID string) error
	RemoveMessageTag(ctx context.Context, messageID, tagID string) error
}

// MailSource is a remote mail server (IMAP/POP3) from which folders and message summaries are pulled.
type MailSource interface {
	FetchFolders(ctx context.Context, account domain.Account) ([]domain.Folder, error)
	FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error)
	FetchBody(ctx context.Context, account domain.Account, folder domain.Folder, uid string) (plain, html string, err error)
	// FetchRaw returns the full raw RFC822 bytes of a message by its opaque handle, used for export
	// (.eml) and for attaching an existing message to a new one.
	FetchRaw(ctx context.Context, account domain.Account, folder domain.Folder, uid string) ([]byte, error)
}

// MailActions performs write operations against a remote mailbox, such as changing message flags. It
// is separate from MailSource so read paths cannot accidentally mutate the server.
type MailActions interface {
	SetSeen(ctx context.Context, account domain.Account, folder domain.Folder, uid string, seen bool) error
	SetFlagged(ctx context.Context, account domain.Account, folder domain.Folder, uid string, flagged bool) error
	// Delete removes a message by its opaque handle. A non-empty trashPath moves it to that mailbox; an
	// empty trashPath deletes it permanently (mark \Deleted and expunge).
	Delete(ctx context.Context, account domain.Account, folder domain.Folder, uid string, trashPath string) error
	// Move relocates a message by its opaque handle from its folder to the destination mailbox.
	Move(ctx context.Context, account domain.Account, folder domain.Folder, uid string, destPath string) error
	// Copy duplicates a message by its opaque handle into the destination mailbox, leaving the original in place.
	Copy(ctx context.Context, account domain.Account, folder domain.Folder, uid string, destPath string) error
}

// MailTransport sends an outgoing message via an account's outgoing (SMTP) server.
type MailTransport interface {
	Send(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) error
}

// FolderActions creates, renames and deletes mailboxes on a remote server. It is separate from the
// message-level MailActions because it changes the folder structure rather than messages.
type FolderActions interface {
	CreateFolder(ctx context.Context, account domain.Account, path string) error
	RenameFolder(ctx context.Context, account domain.Account, oldPath, newPath string) error
	DeleteFolder(ctx context.Context, account domain.Account, path string) error
}

// DraftSaver appends a message to an account's Drafts mailbox on the server, flagged \Draft, so the
// draft is available from any device. It is separate from MailTransport because saving a draft does
// not send anything.
type DraftSaver interface {
	SaveDraft(ctx context.Context, account domain.Account, draftsPath string, msg domain.OutgoingMessage) error
}

// OutboxStore persists outgoing operations that could not reach the server because it was offline, so
// they survive a restart and can be replayed on reconnect. Items are listed oldest first.
type OutboxStore interface {
	EnqueueOutbox(ctx context.Context, item domain.OutboxItem) error
	ListOutbox(ctx context.Context) ([]domain.OutboxItem, error)
	DeleteOutbox(ctx context.Context, id string) error
	MarkOutboxFailed(ctx context.Context, id, reason string) error
}

// RuleStore persists user-defined filter rules, applied to messages as they are synced.
type RuleStore interface {
	ListRules(ctx context.Context) ([]domain.Rule, error)
	SaveRule(ctx context.Context, rule domain.Rule) error
	DeleteRule(ctx context.Context, id string) error
}

// ContactStore persists address-book contacts and groups (mailing lists). Groups reference contacts by
// id; the store owns how that association is held.
type ContactStore interface {
	ListContacts(ctx context.Context) ([]domain.Contact, error)
	GetContact(ctx context.Context, id string) (domain.Contact, error)
	SaveContact(ctx context.Context, contact domain.Contact) error
	DeleteContact(ctx context.Context, id string) error
	ListContactGroups(ctx context.Context) ([]domain.ContactGroup, error)
	SaveContactGroup(ctx context.Context, group domain.ContactGroup) error
	DeleteContactGroup(ctx context.Context, id string) error
}

// ContactCodec converts contacts to and from a serialised address-book format (vCard, CSV). It is the
// import/export seam: one implementation per format, selected by the caller. A decoded contact carries
// its own id (a vCard UID where present) so an import can reconcile against existing records.
type ContactCodec interface {
	Decode(data []byte) ([]domain.Contact, error)
	Encode(contacts []domain.Contact) ([]byte, error)
}

// CalendarStore persists calendars and their events.
type CalendarStore interface {
	ListCalendars(ctx context.Context) ([]domain.Calendar, error)
	SaveCalendar(ctx context.Context, calendar domain.Calendar) error
	DeleteCalendar(ctx context.Context, id string) error
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEvent(ctx context.Context, id string) (domain.Event, error)
	SaveEvent(ctx context.Context, event domain.Event) error
	DeleteEvent(ctx context.Context, id string) error
}

// CalendarCodec converts events to and from a serialised calendar format (ICS). It is the import/export
// seam. A decoded event carries its own id (an ICS UID where present) so an import can reconcile
// against existing records.
type CalendarCodec interface {
	Decode(data []byte) ([]domain.Event, error)
	Encode(events []domain.Event) ([]byte, error)
}

// RecurrenceService performs the recurrence operations that need RRULE parsing, kept outside the domain
// because that parsing needs a dedicated library the domain must not depend on.
type RecurrenceService interface {
	// Expand turns a recurring event's rule and recurrence dates (RRULE, RDATE, EXDATE) into the concrete
	// occurrences whose start falls within the inclusive window [from, to]. Each returned instance carries
	// a RecurrenceID equal to its own start, which identifies the occurrence.
	Expand(event domain.Event, from, to time.Time) ([]domain.EventInstance, error)
	// TruncateBefore returns the given RRULE rewritten so the series ends before at, used when a
	// this-and-future edit or delete splits or shortens a series. Any COUNT is dropped in favour of an
	// UNTIL of one second before at, so the occurrence at at and all later ones are removed.
	TruncateBefore(rule string, at time.Time) (string, error)
}
