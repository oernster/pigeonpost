package main

import (
	"context"

	"github.com/oernster/pigeonpost/internal/application"
)

// UnreadNotifier reflects the total unread count onto an out-of-window surface, namely the Windows
// taskbar overlay badge. It is injected so the facade stays decoupled from the OS-specific
// implementation; on platforms without a taskbar badge it is a no-op.
type UnreadNotifier interface {
	SetUnread(total int)
}

// ReminderAlerter draws attention to a due calendar reminder from outside the window, by flashing the
// taskbar button when the window is not in the foreground. It is injected so the facade stays decoupled
// from the OS-specific implementation; on platforms without a taskbar it is a no-op.
type ReminderAlerter interface {
	Flash()
}

// App is the Wails facade: the single boundary the React front end talks to. It holds no business
// logic, delegating every call to an application use case and mapping domain results to DTOs.
type App struct {
	ctx      context.Context
	closer   func() error
	notifier UnreadNotifier
	alerter  ReminderAlerter
	accounts *application.AccountService
	setup    *application.AccountSetupService
	mailbox  *application.MailboxService
	sync     *application.SyncService
	compose  *application.ComposeService
	tags     *application.TagService
	body     *application.MessageBodyService
	actions  *application.MessageActionService
	folders  *application.FolderService
	rules    *application.RuleService
	contacts *application.ContactService
	calendar *application.CalendarService
}

// NewApp constructs the facade with its injected use-case services and a closer for shutdown.
func NewApp(
	closer func() error,
	notifier UnreadNotifier,
	alerter ReminderAlerter,
	accounts *application.AccountService,
	setup *application.AccountSetupService,
	mailbox *application.MailboxService,
	sync *application.SyncService,
	compose *application.ComposeService,
	tags *application.TagService,
	body *application.MessageBodyService,
	actions *application.MessageActionService,
	folders *application.FolderService,
	rules *application.RuleService,
	contacts *application.ContactService,
	calendar *application.CalendarService,
) *App {
	return &App{
		closer:   closer,
		notifier: notifier,
		alerter:  alerter,
		accounts: accounts,
		setup:    setup,
		mailbox:  mailbox,
		sync:     sync,
		compose:  compose,
		tags:     tags,
		body:     body,
		actions:  actions,
		folders:  folders,
		rules:    rules,
		contacts: contacts,
		calendar: calendar,
	}
}

// startup captures the Wails runtime context and starts the reminder scheduler.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.runReminderScheduler()
}

// shutdown releases infrastructure resources when the window closes.
func (a *App) shutdown(context.Context) {
	if a.closer != nil {
		_ = a.closer()
	}
}

// ListAccounts returns all configured accounts.
func (a *App) ListAccounts() ([]AccountDTO, error) {
	accounts, err := a.accounts.List(a.ctx)
	if err != nil {
		return nil, err
	}
	return toAccountDTOs(accounts), nil
}

// RemoveAccount deletes an account together with its cached mail and its keychain secret.
func (a *App) RemoveAccount(accountID string) error {
	return a.accounts.Remove(a.ctx, accountID)
}

// ListFolders returns the cached folders for an account.
func (a *App) ListFolders(accountID string) ([]FolderDTO, error) {
	folders, err := a.mailbox.Folders(a.ctx, accountID)
	if err != nil {
		return nil, err
	}
	return toFolderDTOs(folders), nil
}

// UnreadCounts returns the unread message count per account and the total across all accounts, for the
// sidebar per-account badges and the cross-account total badge in the titlebar.
func (a *App) UnreadCounts() (UnreadCountsDTO, error) {
	totals, err := a.mailbox.UnreadCounts(a.ctx)
	if err != nil {
		return UnreadCountsDTO{}, err
	}
	byAccount := totals.ByAccount
	if byAccount == nil {
		byAccount = map[string]int{}
	}
	// This is the single derived-total choke point the front end refreshes after every read-state
	// change, so reflecting the total onto the taskbar badge here keeps the badge correct without a
	// separate trigger at each call site.
	if a.notifier != nil {
		a.notifier.SetUnread(totals.Total)
	}
	return UnreadCountsDTO{Total: totals.Total, ByAccount: byAccount}, nil
}

// ListMessages returns the cached message summaries for a folder.
func (a *App) ListMessages(folderID string) ([]MessageDTO, error) {
	messages, err := a.mailbox.Messages(a.ctx, folderID)
	if err != nil {
		return nil, err
	}
	return toMessageDTOs(messages), nil
}

// SearchMessages returns cached messages matching a free-text query, most relevant first.
func (a *App) SearchMessages(query string) ([]MessageDTO, error) {
	messages, err := a.mailbox.Search(a.ctx, query)
	if err != nil {
		return nil, err
	}
	return toMessageDTOs(messages), nil
}

// GetMessageBody returns a message's full body, fetching and caching it on the first open.
func (a *App) GetMessageBody(messageID string) (MessageBodyDTO, error) {
	body, err := a.body.Body(a.ctx, messageID)
	if err != nil {
		return MessageBodyDTO{}, err
	}
	return toMessageBodyDTO(body), nil
}

// SyncAccount pulls folders and messages from the server into the local cache.
func (a *App) SyncAccount(accountID string) error {
	return a.sync.SyncAccount(a.ctx, accountID)
}

// SyncFolder refreshes a single folder's messages from the server, the light path used when a folder
// is opened rather than syncing the whole account.
func (a *App) SyncFolder(folderID string) error {
	return a.sync.SyncFolder(a.ctx, folderID)
}

// MarkRead sets or clears a message's read (Seen) state on the server and in the local cache.
func (a *App) MarkRead(messageID string, read bool) error {
	return a.actions.MarkRead(a.ctx, messageID, read)
}

// MarkFlagged sets or clears a message's flagged (starred) state on the server and in the local cache.
func (a *App) MarkFlagged(messageID string, flagged bool) error {
	return a.actions.MarkFlagged(a.ctx, messageID, flagged)
}

// DeleteMessage removes a message: it is moved to Trash where one exists, otherwise deleted
// permanently. The local cache is updated to match.
func (a *App) DeleteMessage(messageID string) error {
	return a.actions.Delete(a.ctx, messageID)
}

// DeleteMessagePermanent removes a message immediately and irreversibly, without moving it to Trash,
// regardless of which folder it lives in. The local cache is updated to match.
func (a *App) DeleteMessagePermanent(messageID string) error {
	return a.actions.DeletePermanent(a.ctx, messageID)
}

// MoveMessage relocates a message to another folder in the same account.
func (a *App) MoveMessage(messageID, destFolderID string) error {
	return a.actions.Move(a.ctx, messageID, destFolderID)
}

// CopyMessage duplicates a message into another folder in the same account, leaving the original.
func (a *App) CopyMessage(messageID, destFolderID string) error {
	return a.actions.Copy(a.ctx, messageID, destFolderID)
}

// CreateFolder creates a new mailbox on the account's server and refreshes the cached folder list.
func (a *App) CreateFolder(accountID, name string) error {
	return a.folders.Create(a.ctx, accountID, name)
}

// RenameFolder renames a folder on the server and refreshes the cached folder list.
func (a *App) RenameFolder(folderID, newName string) error {
	return a.folders.Rename(a.ctx, folderID, newName)
}

// DeleteFolder deletes a folder on the server, clears its cached messages and refreshes the list.
func (a *App) DeleteFolder(folderID string) error {
	return a.folders.Delete(a.ctx, folderID)
}
