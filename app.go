package main

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/taskbar"
)

// singleInstanceID names the per-user lock that keeps only one PigeonPost running; a second launch
// signals this instance to reveal itself instead of starting its own window.
const singleInstanceID = "uk.codecrafter.pigeonpost"

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

// MailWatcher maintains a live server connection that invokes a callback the instant an account's inbox
// changes, for push new-mail detection. It is injected so the facade stays decoupled from the IMAP
// implementation; a nil watcher just leaves the poll as the only mechanism.
type MailWatcher interface {
	Watch(ctx context.Context, account domain.Account, onChange func())
}

// App is the Wails facade: the single boundary the React front end talks to. It holds no business
// logic, delegating every call to an application use case and mapping domain results to DTOs.
type App struct {
	ctx        context.Context
	closer     func() error
	notifier   UnreadNotifier
	alerter    ReminderAlerter
	tray       *taskbar.Tray
	watcher    MailWatcher
	watchers   map[string]context.CancelFunc // per-account IDLE watcher cancels, keyed by account id
	watchersMu sync.Mutex                    // guards watchers
	mailCheck  sync.Mutex                    // serialises checkMail so the poll and IDLE pushes do not detect concurrently
	quitting   atomic.Bool                   // set when an explicit Quit is under way, so the close prompt is skipped
	accounts   *application.AccountService
	setup      *application.AccountSetupService
	msSetup    *application.MicrosoftSetupService
	mailbox    *application.MailboxService
	sync       *application.SyncService
	compose    *application.ComposeService
	tags       *application.TagService
	body       *application.MessageBodyService
	actions    *application.MessageActionService
	folders    *application.FolderService
	rules      *application.RuleService
	contacts   *application.ContactService
	calendar   *application.CalendarService
	scheduling *application.SchedulingService
}

// NewApp constructs the facade with its injected use-case services and a closer for shutdown.
func NewApp(
	closer func() error,
	notifier UnreadNotifier,
	alerter ReminderAlerter,
	tray *taskbar.Tray,
	watcher MailWatcher,
	accounts *application.AccountService,
	setup *application.AccountSetupService,
	microsoftSetup *application.MicrosoftSetupService,
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
	scheduling *application.SchedulingService,
) *App {
	return &App{
		closer:     closer,
		notifier:   notifier,
		alerter:    alerter,
		tray:       tray,
		watcher:    watcher,
		watchers:   make(map[string]context.CancelFunc),
		accounts:   accounts,
		setup:      setup,
		msSetup:    microsoftSetup,
		mailbox:    mailbox,
		sync:       sync,
		compose:    compose,
		tags:       tags,
		body:       body,
		actions:    actions,
		folders:    folders,
		rules:      rules,
		contacts:   contacts,
		calendar:   calendar,
		scheduling: scheduling,
	}
}

// startup captures the Wails runtime context, starts the tray (its menu items emit Wails events the
// front end turns into the matching dialogs, so this layer owns the callbacks and the taskbar package
// stays free of Wails), and starts the reminder scheduler and the new-mail notifier.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.tray != nil {
		a.tray.Start(taskbar.TrayActions{
			// Open must go through the Wails runtime, not a Win32 window search: when the window is hidden
			// to the tray it is no longer a findable visible window.
			Open:         func() { a.revealWindow() },
			About:        func() { runtime.EventsEmit(ctx, "menu:about") },
			Licence:      func() { runtime.EventsEmit(ctx, "menu:licence") },
			CheckUpdates: func() { runtime.EventsEmit(ctx, "menu:check-updates") },
			Quit:         func() { a.quit() },
		})
	}
	go a.runReminderScheduler()
	go a.runMailNotifier()
}

// beforeClose runs when the user clicks the window's close button. Rather than quit, it asks the front
// end to show the close-choice dialog (the app's own dark-themed dialog, not a native one) and keeps the
// window open by returning true; the dialog then calls MinimiseToTray or RequestQuit. An explicit Quit
// already under way, or a platform without a restorable tray icon (every platform but Windows, where
// hiding the window would strand it), skips the prompt and lets the close proceed.
func (a *App) beforeClose(ctx context.Context) bool {
	if a.quitting.Load() {
		return false
	}
	if a.tray == nil || !a.tray.CanHideToTray() {
		return false
	}
	runtime.EventsEmit(ctx, "app:close-request")
	return true
}

// MinimiseToTray hides the window so PigeonPost keeps running in the tray. The close-choice dialog calls
// this for its Minimise option.
func (a *App) MinimiseToTray() {
	runtime.WindowHide(a.ctx)
}

// RequestQuit quits the application from the close-choice dialog's Quit option, recording the intent so
// the close prompt is not shown again for this quit.
func (a *App) RequestQuit() {
	a.quit()
}

// quit records that an explicit Quit is under way, so beforeClose does not prompt, then quits.
func (a *App) quit() {
	a.quitting.Store(true)
	runtime.Quit(a.ctx)
}

// onSecondInstance runs in the already-running instance when the user launches PigeonPost again. Rather
// than start a second window, it reveals this one, which may be hidden in the tray or minimised.
func (a *App) onSecondInstance(options.SecondInstanceData) {
	a.revealWindow()
}

// revealWindow brings the window back into view: it un-hides it (it may be hidden to the tray) and
// un-minimises it. Used by the tray's Open action and by a second launch.
func (a *App) revealWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// shutdown releases infrastructure resources when the window closes, removing the tray icon first.
func (a *App) shutdown(context.Context) {
	if a.tray != nil {
		a.tray.Stop()
	}
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

// RemoveAccount deletes an account together with its cached mail and its keychain secret, and stops its
// IDLE watcher so a removed account leaves no stale server connection behind.
func (a *App) RemoveAccount(accountID string) error {
	if err := a.accounts.Remove(a.ctx, accountID); err != nil {
		return err
	}
	a.stopMailWatcher(accountID)
	return nil
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
	// change, so reflecting the total onto the taskbar badge and the tray icon here keeps both correct
	// without a separate trigger at each call site. The taskbar badge shows on the window's taskbar
	// button; the tray badge shows even when the window is hidden to the tray.
	if a.notifier != nil {
		a.notifier.SetUnread(totals.Total)
	}
	if a.tray != nil {
		a.tray.SetUnread(totals.Total)
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

// ListThreads returns the cached messages of a folder grouped into conversations, newest conversation
// first, for the reading list's conversation view.
func (a *App) ListThreads(folderID string) ([]ThreadDTO, error) {
	threads, err := a.mailbox.Threads(a.ctx, folderID)
	if err != nil {
		return nil, err
	}
	return toThreadDTOs(threads), nil
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

// DeleteMessages removes several messages in one batched pass per folder, far faster than a call per
// message: each is moved to Trash where the account has one, otherwise deleted permanently. It returns
// which ids were removed (so the UI drops exactly those) plus any error text, rather than failing the
// whole call, so a partial success still reports what went.
func (a *App) DeleteMessages(ids []string) BulkResultDTO {
	return a.bulkDelete(ids, false)
}

// DeleteMessagesPermanent removes several messages immediately and irreversibly, without moving them to
// Trash. It is the batched counterpart to DeleteMessagePermanent.
func (a *App) DeleteMessagesPermanent(ids []string) BulkResultDTO {
	return a.bulkDelete(ids, true)
}

// bulkDelete runs the batched delete and packs its outcome into the DTO the front end reads.
func (a *App) bulkDelete(ids []string, permanent bool) BulkResultDTO {
	deleted, err := a.actions.DeleteMany(a.ctx, ids, permanent)
	return bulkResult(ids, deleted, err)
}

// MoveMessages relocates several messages into one folder in a single batched pass per source folder,
// far faster than a call per message: this is what keeps a drag-and-drop or a bulk "Move to" of a large
// Gmail selection under Gmail's simultaneous-connection cap. It returns which ids moved so the UI drops
// exactly those, plus any error text.
func (a *App) MoveMessages(ids []string, destFolderID string) BulkResultDTO {
	moved, err := a.actions.MoveMany(a.ctx, ids, destFolderID)
	return bulkResult(ids, moved, err)
}

// bulkResult packs a batched action's outcome (the ids acted on plus any error) into the DTO the front
// end reads, reporting how many of the requested ids were not processed.
func bulkResult(requested, acted []string, err error) BulkResultDTO {
	result := BulkResultDTO{Ids: acted, Failed: len(requested) - len(acted)}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// MoveMessage relocates a message to another folder in the same account.
func (a *App) MoveMessage(messageID, destFolderID string) error {
	return a.actions.Move(a.ctx, messageID, destFolderID)
}

// MarkJunk moves a message to the account's Junk folder, filing it out of the inbox as spam. It returns
// an error when the account has no Junk folder or the message already lives there.
func (a *App) MarkJunk(messageID string) error {
	return a.actions.MarkJunk(a.ctx, messageID)
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
