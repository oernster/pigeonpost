package main

import (
	"context"

	"github.com/oernster/pigeonpost/internal/application"
)

// App is the Wails facade: the single boundary the React front end talks to. It holds no business
// logic, delegating every call to an application use case and mapping domain results to DTOs.
type App struct {
	ctx      context.Context
	closer   func() error
	accounts *application.AccountService
	mailbox  *application.MailboxService
	sync     *application.SyncService
	compose  *application.ComposeService
}

// NewApp constructs the facade with its injected use-case services and a closer for shutdown.
func NewApp(
	closer func() error,
	accounts *application.AccountService,
	mailbox *application.MailboxService,
	sync *application.SyncService,
	compose *application.ComposeService,
) *App {
	return &App{
		closer:   closer,
		accounts: accounts,
		mailbox:  mailbox,
		sync:     sync,
		compose:  compose,
	}
}

// startup captures the Wails runtime context.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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

// ListFolders returns the cached folders for an account.
func (a *App) ListFolders(accountID string) ([]FolderDTO, error) {
	folders, err := a.mailbox.Folders(a.ctx, accountID)
	if err != nil {
		return nil, err
	}
	return toFolderDTOs(folders), nil
}

// ListMessages returns the cached message summaries for a folder.
func (a *App) ListMessages(folderID string) ([]MessageDTO, error) {
	messages, err := a.mailbox.Messages(a.ctx, folderID)
	if err != nil {
		return nil, err
	}
	return toMessageDTOs(messages), nil
}

// SyncAccount pulls folders and messages from the server into the local cache.
func (a *App) SyncAccount(accountID string) error {
	return a.sync.SyncAccount(a.ctx, accountID)
}

// MarkRead sets or clears a message's read (Seen) state in the local cache.
func (a *App) MarkRead(messageID string, read bool) error {
	return a.mailbox.MarkRead(a.ctx, messageID, read)
}
