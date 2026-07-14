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
	// SetAccountPositions writes the sidebar order: the account at index i in orderedIDs gets position i.
	SetAccountPositions(ctx context.Context, orderedIDs []string) error
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
// offline; the sync service writes to it. The Visible listing variants exclude messages hidden by an
// unexpired snooze and back the reading views; the plain variants see everything and back the sync
// (whose known-message sets and flag carry-over must include snoozed rows) and search.
type MailStore interface {
	ListFolders(ctx context.Context, accountID string) ([]domain.Folder, error)
	SaveFolders(ctx context.Context, accountID string, folders []domain.Folder) error
	ListMessages(ctx context.Context, folderID string) ([]domain.MessageSummary, error)
	ListMessagesPage(ctx context.Context, folderID string, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool) ([]domain.MessageSummary, error)
	ListMessagesVisible(ctx context.Context, folderID string, visibleAt time.Time) ([]domain.MessageSummary, error)
	ListMessagesPageVisible(ctx context.Context, folderID string, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool, visibleAt time.Time) ([]domain.MessageSummary, error)
	SaveMessages(ctx context.Context, folderID string, messages []domain.MessageSummary) error
	SetSeen(ctx context.Context, messageID string, seen bool) error
	SetFlagged(ctx context.Context, messageID string, flagged bool) error
	SetAnswered(ctx context.Context, messageID string, answered bool) error
	SetForwarded(ctx context.Context, messageID string, forwarded bool) error
	DeleteAccountData(ctx context.Context, accountID string) error
	GetMessage(ctx context.Context, messageID string) (domain.MessageSummary, error)
	GetFolder(ctx context.Context, folderID string) (domain.Folder, error)
	// UnreadByAccount counts only messages visible at the given instant, so a snoozed unread message
	// does not badge the folder it is hidden from until it resurfaces.
	UnreadByAccount(ctx context.Context, visibleAt time.Time) (map[string]int, error)
	GetMessageBody(ctx context.Context, messageID string) (domain.MessageBody, error)
	SaveMessageBody(ctx context.Context, body domain.MessageBody) error
	// SearchMessages returns the cached messages matching the modelled query, most relevant first,
	// capped at limit. Each hit's snippet wraps matched terms in SearchMatchStart/SearchMatchEnd.
	// Snoozed messages stay searchable: hiding is a folder-view concern, not an existence one.
	SearchMessages(ctx context.Context, query domain.SearchQuery, limit int) ([]SearchHit, error)
	DeleteMessage(ctx context.Context, messageID string) error
}

// SnoozedMessage pairs a hidden message with the instant it resurfaces and its owning account (the
// Snoozed view spans accounts, so each row must say whose it is), for the Snoozed view.
type SnoozedMessage struct {
	Summary   domain.MessageSummary
	Until     time.Time
	AccountID string
}

// SnoozeStore is the local snooze state: a message with a snooze row is hidden from the visible
// listings until its instant passes, then reappears untouched. It is local-only (nothing reaches the
// server) and implemented by the same SQLite store as MailStore; it is a separate contract so the
// snooze service depends only on what it uses.
type SnoozeStore interface {
	SetSnooze(ctx context.Context, messageID string, until time.Time) error
	ClearSnooze(ctx context.Context, messageID string) error
	// ListSnoozed returns every snoozed message with its due instant, soonest first.
	ListSnoozed(ctx context.Context) ([]SnoozedMessage, error)
	// PopDueSnoozed removes every snooze whose instant has passed and returns the messages that just
	// resurfaced, for the caller to announce. A snooze orphaned by its message's deletion is removed
	// without being returned.
	PopDueSnoozed(ctx context.Context, now time.Time) ([]domain.MessageSummary, error)
	// NextSnooze returns the earliest pending snooze instant and whether one exists, so the resurface
	// scheduler only does work when something is actually due.
	NextSnooze(ctx context.Context) (time.Time, bool, error)
}

// SearchMatchStart and SearchMatchEnd delimit each matched term inside a SearchHit's snippet. They are
// control characters that cannot appear in message text, so the UI can split on them and highlight the
// matches without ever interpreting message content as markup.
const (
	SearchMatchStart = "\x01"
	SearchMatchEnd   = "\x02"
)

// SearchHit is one search result: the matched message and a snippet of the matched text with each
// matched term wrapped in the match markers. The snippet is empty for a purely structural query (one
// with no search text, such as "is:unread in:INBOX").
type SearchHit struct {
	Summary domain.MessageSummary
	Snippet string
}

// TagStore persists user-defined coloured tags and their many-to-many association with messages.
type TagStore interface {
	ListTags(ctx context.Context) ([]domain.Tag, error)
	SaveTag(ctx context.Context, tag domain.Tag) error
	DeleteTag(ctx context.Context, id string) error
	TagsForMessage(ctx context.Context, messageID string) ([]domain.Tag, error)
	// TagColoursForMessages returns the hex tag colours of each of the given message ids in one query,
	// keyed by message id, so the message list can show tag colours without a query per row.
	TagColoursForMessages(ctx context.Context, messageIDs []string) (map[string][]string, error)
	AddMessageTag(ctx context.Context, messageID, tagID string) error
	RemoveMessageTag(ctx context.Context, messageID, tagID string) error
	// AssignMessageTag attaches a tag to a message and (when recordPending is true) records the pending
	// intent to sync it, in one transaction so the local change and its intent cannot drift apart.
	AssignMessageTag(ctx context.Context, messageID, tagID string, recordPending bool) error
	// UnassignMessageTag detaches a tag from a message and (when recordPending is true) records the pending
	// intent to remove it on the server, in one transaction.
	UnassignMessageTag(ctx context.Context, messageID, tagID string, recordPending bool) error
	// SetPendingTagOp records the intended state of a (message, tag) pair not yet confirmed on the server
	// (assigned true for an assignment, false for a removal), replacing any existing intent for that pair.
	SetPendingTagOp(ctx context.Context, messageID, tagID string, assigned bool) error
	// ClearPendingTagOp removes the pending intent for a (message, tag) pair, called once the server agrees.
	ClearPendingTagOp(ctx context.Context, messageID, tagID string) error
	// PendingTagOps returns the pending intents for one message keyed by tag id (true for a pending
	// assignment, false for a pending removal), read during a reconcile to guard unsynced local changes.
	PendingTagOps(ctx context.Context, messageID string) (map[string]bool, error)
	// ListPendingTagOps returns every pending tag operation across all messages, used to replay unsynced
	// intents to the server on a sync.
	ListPendingTagOps(ctx context.Context) ([]domain.PendingTagOp, error)
}

// MailSource is a remote mail server (IMAP/POP3) from which folders and message summaries are pulled.
type MailSource interface {
	FetchFolders(ctx context.Context, account domain.Account) ([]domain.Folder, error)
	FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error)
	// FetchBody returns a message's plain-text and HTML bodies, any raw text/calendar scheduling payload
	// (an iMIP invite or reply, nil when the message carried none), and its attachments (empty when it
	// carried none).
	FetchBody(ctx context.Context, account domain.Account, folder domain.Folder, uid string) (plain, html string, invite []byte, attachments []domain.Attachment, err error)
	// FetchRaw returns the full raw RFC822 bytes of a message by its opaque handle, used for export
	// (.eml) and for attaching an existing message to a new one.
	FetchRaw(ctx context.Context, account domain.Account, folder domain.Folder, uid string) ([]byte, error)
}

// MailActions performs write operations against a remote mailbox, such as changing message flags. It
// is separate from MailSource so read paths cannot accidentally mutate the server.
type MailActions interface {
	SetSeen(ctx context.Context, account domain.Account, folder domain.Folder, uid string, seen bool) error
	SetFlagged(ctx context.Context, account domain.Account, folder domain.Folder, uid string, flagged bool) error
	// SetAnswered marks a message replied-to (\Answered) on the server; SetForwarded marks it forwarded
	// ($Forwarded keyword). Both are set after the corresponding message is sent.
	SetAnswered(ctx context.Context, account domain.Account, folder domain.Folder, uid string, answered bool) error
	SetForwarded(ctx context.Context, account domain.Account, folder domain.Folder, uid string, forwarded bool) error
	// SetKeyword adds or removes an arbitrary IMAP keyword on a message by its opaque handle, used to round
	// a user tag onto the server as a keyword. It is separate from the fixed system-flag setters because the
	// keyword is chosen by the caller.
	SetKeyword(ctx context.Context, account domain.Account, folder domain.Folder, uid string, keyword string, set bool) error
	// Delete removes a message by its opaque handle. A non-empty trashPath moves it to that mailbox; an
	// empty trashPath deletes it permanently (mark \Deleted and expunge).
	Delete(ctx context.Context, account domain.Account, folder domain.Folder, uid string, trashPath string) error
	// Move relocates a message by its opaque handle from its folder to the destination mailbox.
	Move(ctx context.Context, account domain.Account, folder domain.Folder, uid string, destPath string) error
	// Copy duplicates a message by its opaque handle into the destination mailbox, leaving the original in place.
	Copy(ctx context.Context, account domain.Account, folder domain.Folder, uid string, destPath string) error
	// DeleteMany removes several messages that live in the same folder in one server round trip: it moves
	// them to trashPath or deletes them permanently when trashPath is empty. It is the batched form of
	// Delete, so a bulk delete opens one connection for the whole folder instead of one per message.
	DeleteMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, trashPath string) error
	// MoveMany relocates several messages from one folder to destPath in one server round trip. It is the
	// batched form of Move, so a bulk move or a drag-and-drop of a selection opens one connection for the
	// whole folder instead of one per message.
	MoveMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, destPath string) error
}

// MailTransport sends an outgoing message via an account's outgoing (SMTP) server.
type MailTransport interface {
	Send(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) error
}

// FolderActions creates, renames and deletes mailboxes on a remote server. It is separate from the
// message-level MailActions because it changes the folder structure rather than messages.
// MoveAllMessages relocates every message in one mailbox into another, used when merging a stray sent
// folder into the canonical one; an empty source mailbox is a no-op.
type FolderActions interface {
	CreateFolder(ctx context.Context, account domain.Account, path string) error
	RenameFolder(ctx context.Context, account domain.Account, oldPath, newPath string) error
	DeleteFolder(ctx context.Context, account domain.Account, path string) error
	MoveAllMessages(ctx context.Context, account domain.Account, fromPath, toPath string) error
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
	// already gone, which the undo-send path surfaces as "the message had already left".
	DeleteOutbox(ctx context.Context, id string) (bool, error)
	MarkOutboxFailed(ctx context.Context, id, reason string) error
	// ClearOutboxHold removes an item's undo-send hold so it degrades to an ordinary queued operation.
	ClearOutboxHold(ctx context.Context, id string) error
	// NextOutboxHold returns the earliest undo-send hold among unfailed items and whether one exists.
	NextOutboxHold(ctx context.Context) (time.Time, bool, error)
}

// DraftRecoveryStore persists a single local snapshot of an in-progress compose window, so a message
// still being written survives an accidental close or a crash. It never touches the server; it is the
// local-recovery counterpart to the server-side draft that DraftSaver appends. Only the most recent
// snapshot is kept: SaveDraftRecovery replaces any existing one, and GetDraftRecovery reports whether a
// snapshot is present.
type DraftRecoveryStore interface {
	SaveDraftRecovery(ctx context.Context, recovery domain.DraftRecovery) error
	GetDraftRecovery(ctx context.Context) (domain.DraftRecovery, bool, error)
	ClearDraftRecovery(ctx context.Context) error
}

// RuleStore persists user-defined filter rules, applied to messages as they are synced.
type RuleStore interface {
	ListRules(ctx context.Context) ([]domain.Rule, error)
	SaveRule(ctx context.Context, rule domain.Rule) error
	DeleteRule(ctx context.Context, id string) error
}

// TemplateStore persists user-defined message templates, inserted while composing.
type TemplateStore interface {
	ListTemplates(ctx context.Context) ([]domain.Template, error)
	SaveTemplate(ctx context.Context, template domain.Template) error
	DeleteTemplate(ctx context.Context, id string) error
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

// CalendarStore persists calendars, their events and any preserved non-event passthrough components.
type CalendarStore interface {
	ListCalendars(ctx context.Context) ([]domain.Calendar, error)
	SaveCalendar(ctx context.Context, calendar domain.Calendar) error
	DeleteCalendar(ctx context.Context, id string) error
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEvent(ctx context.Context, id string) (domain.Event, error)
	SaveEvent(ctx context.Context, event domain.Event) error
	DeleteEvent(ctx context.Context, id string) error
	// SavePassthrough stores or replaces (by UID) a preserved VTODO or VJOURNAL; ListPassthrough returns
	// them all for re-export.
	SavePassthrough(ctx context.Context, passthrough domain.CalendarPassthrough) error
	ListPassthrough(ctx context.Context) ([]domain.CalendarPassthrough, error)
}

// CalDAVSource reads calendars and their objects from a remote CalDAV server. It is the read half of the
// two-way DAV sync (the infrastructure go-webdav adapter implements it), the calendar counterpart to
// MailSource. RemoteCalendar and RemoteObject are defined with the sync service in caldav.go.
type CalDAVSource interface {
	ListCalendars(ctx context.Context) ([]RemoteCalendar, error)
	ListObjects(ctx context.Context, calendar RemoteCalendar) ([]RemoteObject, error)
	// CollectionCTag returns a collection's CTag (its calendarserver.org change tag), used to skip an
	// unchanged collection on a sync. An empty string means the server does not report one, so the caller
	// reconciles unconditionally; an error is a transport or parse failure, which the caller treats the same
	// way (it cannot skip, so it reconciles).
	CollectionCTag(ctx context.Context, collectionHref string) (string, error)
}

// CalendarAccountStore persists CalDAV/CardDAV accounts. Each account's password is not stored here; it
// lives in the OS keychain, keyed by the account id, as for a mail account.
type CalendarAccountStore interface {
	SaveCalendarAccount(ctx context.Context, account domain.CalendarAccount) error
	ListCalendarAccounts(ctx context.Context) ([]domain.CalendarAccount, error)
	GetCalendarAccount(ctx context.Context, id string) (domain.CalendarAccount, error)
	DeleteCalendarAccount(ctx context.Context, id string) error
}

// CalendarCredentialStore keeps a CalDAV/CardDAV account's password in the OS keychain, never the database.
type CalendarCredentialStore interface {
	CalendarPassword(ctx context.Context, account domain.CalendarAccount) (string, error)
	SetCalendarPassword(ctx context.Context, account domain.CalendarAccount, secret string) error
	DeleteCalendarPassword(ctx context.Context, account domain.CalendarAccount) error
}

// CalDAVSourceFactory builds a CalDAVSource for an account and password. It is the seam that keeps the
// application free of the go-webdav client: the infrastructure adapter implements it.
type CalDAVSourceFactory interface {
	NewSource(account domain.CalendarAccount, password string) (CalDAVSource, error)
}

// CalendarCodec converts events to and from a serialised calendar format (ICS). It is the import/export
// seam. A decoded event carries its own id (an ICS UID where present) so an import can reconcile against
// existing records. Non-event components PigeonPost does not model (to-dos and journal entries) are
// carried as passthrough so they survive a round-trip.
type CalendarCodec interface {
	Decode(data []byte) ([]domain.Event, []domain.CalendarPassthrough, error)
	Encode(events []domain.Event, passthrough []domain.CalendarPassthrough) ([]byte, error)
}

// SchedulingCodec converts iTIP (RFC 5546) scheduling messages to and from the text/calendar payload an
// email carries (RFC 6047 iMIP). It is the seam the scheduling service uses: DecodeScheduling reads an
// incoming invite or reply (the VCALENDAR METHOD and its events, each with its organizer and attendees),
// and the encode methods build the REQUEST, REPLY and CANCEL a two-way invite flow sends back out.
type SchedulingCodec interface {
	DecodeScheduling(data []byte) (domain.SchedulingMessage, error)
	// EncodeRequest builds a METHOD:REQUEST inviting the attendees carried on the events.
	EncodeRequest(events []domain.Event) ([]byte, error)
	// EncodeCancel builds a METHOD:CANCEL withdrawing the events.
	EncodeCancel(events []domain.Event) ([]byte, error)
	// EncodeReply builds a METHOD:REPLY carrying the responder as the single attendee with the status that
	// is their answer, so the organizer sees only the response that changed.
	EncodeReply(event domain.Event, responder domain.EmailAddress, status domain.ParticipationStatus) ([]byte, error)
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
	// SplitCountForward returns the master's RRULE for the forward half of a this-and-following split.
	// A COUNT-based rule has its COUNT reduced by the number of occurrences that precede at, so the split
	// keeps the series total instead of restarting the count; an open-ended or UNTIL-bounded rule is
	// returned unchanged.
	SplitCountForward(master domain.Event, at time.Time) (string, error)
}

// RemoteImageResolver fetches a message body's blocked remote images server-side and returns the HTML with
// each fetched image inlined as a data: URI. It is the seam that lets the reader show images a browser cannot
// load cross-origin (a sender's Cross-Origin-Resource-Policy, CORS or hotlink protection stops direct
// embedding); an image it cannot fetch is left parked. The fetch is hardened against server-side request
// forgery in the infrastructure implementation.
type RemoteImageResolver interface {
	Resolve(ctx context.Context, html string) (string, error)
}
