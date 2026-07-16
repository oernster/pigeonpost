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
	ctx           context.Context
	title         string // main window title, used to locate its HWND to focus the WebView on launch
	pendingEmail  string // a .eml path from a cold-launch argument, opened once the front end is ready (see lifecycle.go)
	pendingMailto string // a mailto: URI from a cold-launch argument, opened as a compose once the front end is ready
	closer        func() error
	notifier      UnreadNotifier
	alerter       ReminderAlerter
	tray          *taskbar.Tray
	watcher       MailWatcher
	watchers      map[string]context.CancelFunc // per-account IDLE watcher cancels, keyed by account id
	watchersMu    sync.Mutex                    // guards watchers
	mailCheck     sync.Mutex                    // serialises checkMail so the poll and IDLE pushes do not detect concurrently
	quitting      atomic.Bool                   // set when an explicit Quit is under way, so the close prompt is skipped
	accounts      *application.AccountService
	setup         *application.AccountSetupService
	msSetup       *application.MicrosoftSetupService
	mailbox       *application.MailboxService
	unified       *application.UnifiedMailboxService
	snooze        *application.SnoozeService
	sync          *application.SyncService
	compose       *application.ComposeService
	tags          *application.TagService
	tagSync       *application.TagSyncService
	body          *application.MessageBodyService
	actions       *application.MessageActionService
	folders       *application.FolderService
	rules         *application.RuleService
	templates     *application.TemplateService
	contacts      *application.ContactService
	calendar      *application.CalendarService
	calendarEdit  *application.CalendarEditService
	scheduling    *application.SchedulingService
	remoteImages  *application.RemoteImageService
	caldav        *application.CalDAVService
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
	unified *application.UnifiedMailboxService,
	snooze *application.SnoozeService,
	sync *application.SyncService,
	compose *application.ComposeService,
	tags *application.TagService,
	tagSync *application.TagSyncService,
	body *application.MessageBodyService,
	actions *application.MessageActionService,
	folders *application.FolderService,
	rules *application.RuleService,
	templates *application.TemplateService,
	contacts *application.ContactService,
	calendar *application.CalendarService,
	calendarEdit *application.CalendarEditService,
	scheduling *application.SchedulingService,
	remoteImages *application.RemoteImageService,
	caldav *application.CalDAVService,
) *App {
	return &App{
		closer:       closer,
		notifier:     notifier,
		alerter:      alerter,
		tray:         tray,
		watcher:      watcher,
		watchers:     make(map[string]context.CancelFunc),
		accounts:     accounts,
		setup:        setup,
		msSetup:      microsoftSetup,
		mailbox:      mailbox,
		unified:      unified,
		snooze:       snooze,
		sync:         sync,
		compose:      compose,
		tags:         tags,
		tagSync:      tagSync,
		body:         body,
		actions:      actions,
		folders:      folders,
		rules:        rules,
		templates:    templates,
		contacts:     contacts,
		calendar:     calendar,
		calendarEdit: calendarEdit,
		scheduling:   scheduling,
		remoteImages: remoteImages,
		caldav:       caldav,
	}
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
	// Bring the window to the front before the close-choice dialog is shown. A close requested while the
	// window is minimised or behind other windows (for example the taskbar button's "Close window") would
	// otherwise render the dialog on a background window, so it looks like nothing happened until the user
	// refocuses PigeonPost by hand.
	a.revealWindow()
	taskbar.FocusMainWindow(a.title)
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

// onSecondInstance runs in the already-running instance when the user launches PigeonPost again, including
// when Windows launches it to open a double-clicked .eml file or a clicked mailto: link (PigeonPost being
// the registered handler). Rather than start a second window it reveals this one (which may be hidden in
// the tray or minimised) then opens any .eml or mailto: passed on the second launch's command line.
func (a *App) onSecondInstance(data options.SecondInstanceData) {
	a.revealWindow()
	if path := firstEmailFileArg(data.Args); path != "" {
		a.openEmailFile(path)
	}
	if uri := firstMailtoArg(data.Args); uri != "" {
		a.openMailto(uri)
	}
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

// ReorderAccounts sets the sidebar order of accounts from the given full list of ids, most preferred
// first. The front end sends the complete new order after a drag or an up/down move.
func (a *App) ReorderAccounts(orderedIDs []string) error {
	return a.accounts.Reorder(a.ctx, orderedIDs)
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

// GetMessageBody returns a message's full body, fetching and caching it on the first open.
func (a *App) GetMessageBody(messageID string) (MessageBodyDTO, error) {
	body, err := a.body.Body(a.ctx, messageID)
	if err != nil {
		return MessageBodyDTO{}, err
	}
	return toMessageBodyDTO(body), nil
}

// SyncAccount pulls folders and messages from the server into the local cache. The sent-folder
// reconciliation runs first, so any stray sent folders are merged into the one canonical Sent on the
// server and the sync then stores the clean structure; the pass is idempotent, so on a healthy account
// it does nothing.
func (a *App) SyncAccount(accountID string) error {
	if err := a.folders.ReconcileSent(a.ctx, accountID); err != nil {
		return err
	}
	return a.sync.SyncAccount(a.ctx, accountID)
}

// SyncFolder refreshes a single folder's messages from the server, the light path used when a folder
// is opened rather than syncing the whole account.
func (a *App) SyncFolder(folderID string) error {
	return a.sync.SyncFolder(a.ctx, folderID)
}

// SyncAllInboxes refreshes every account's inbox folders from their servers, the unified mailbox's
// counterpart to SyncFolder. The arrivals SyncInboxes reports are discarded: they feed the new-mail
// notifier's own passes, and mail fetched here is cached before that pass exactly as an opened folder's
// sync caches what the user is already looking at.
func (a *App) SyncAllInboxes() error {
	_, err := a.sync.SyncInboxes(a.ctx)
	return err
}
