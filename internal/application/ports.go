package application

import (
	"context"

	"github.com/oernster/pigeonpost/internal/domain"
)

// AccountStore persists and retrieves accounts. Credentials are not part of this contract; they are
// held in the OS keychain and referenced separately.
type AccountStore interface {
	ListAccounts(ctx context.Context) ([]domain.Account, error)
	GetAccount(ctx context.Context, id string) (domain.Account, error)
	SaveAccount(ctx context.Context, account domain.Account) error
}

// MailStore is the local cache of folders and message summaries. The UI reads from here so it works
// offline; the sync service writes to it.
type MailStore interface {
	ListFolders(ctx context.Context, accountID string) ([]domain.Folder, error)
	SaveFolders(ctx context.Context, accountID string, folders []domain.Folder) error
	ListMessages(ctx context.Context, folderID string) ([]domain.MessageSummary, error)
	SaveMessages(ctx context.Context, folderID string, messages []domain.MessageSummary) error
	SetSeen(ctx context.Context, messageID string, seen bool) error
}

// MailSource is a remote mail server (IMAP/POP3) from which folders and message summaries are pulled.
type MailSource interface {
	FetchFolders(ctx context.Context, account domain.Account) ([]domain.Folder, error)
	FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error)
}

// MailTransport sends an outgoing message via an account's outgoing (SMTP) server.
type MailTransport interface {
	Send(ctx context.Context, account domain.Account, msg domain.OutgoingMessage) error
}
