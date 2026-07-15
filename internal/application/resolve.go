package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// resolveMessageContext locates a cached message together with the folder it lives in and the account
// that folder belongs to. This three-step lookup is what every server-side message action needs before
// it can act, so keeping it (and its error wrapping) in one place removes the copies that would otherwise
// drift. Callers take whichever of the three values they need and pass the error straight up.
func resolveMessageContext(ctx context.Context, messages MailStore, accounts AccountStore, messageID string) (domain.MessageSummary, domain.Folder, domain.Account, error) {
	msg, err := messages.GetMessage(ctx, messageID)
	if err != nil {
		return domain.MessageSummary{}, domain.Folder{}, domain.Account{}, fmt.Errorf("locate message %q: %w", messageID, err)
	}
	folder, err := messages.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return domain.MessageSummary{}, domain.Folder{}, domain.Account{}, fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	account, err := accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return domain.MessageSummary{}, domain.Folder{}, domain.Account{}, fmt.Errorf("locate account %q: %w", folder.AccountID(), err)
	}
	return msg, folder, account, nil
}

// folderLister is the narrow slice of a store the folder-path lookup needs: the ability to list an
// account's folders. Keeping the dependency this small lets both the action and compose use cases share
// the lookup without either depending on the other's fuller store interface.
type folderLister interface {
	ListFolders(ctx context.Context, accountID string) ([]domain.Folder, error)
}

// folderPathByKind returns the server path of the account's folder of the given well-known kind, and
// whether one was found. A missing folder of that kind is not an error (it returns "", false, nil), so
// each caller applies its own policy: permanent-delete when there is no Trash, skip the Sent copy when
// there is no Sent, ErrNoDraftsFolder when a draft cannot be saved, and so on.
func folderPathByKind(ctx context.Context, store folderLister, accountID string, kind domain.FolderKind) (string, bool, error) {
	folder, ok, err := folderByKind(ctx, store, accountID, kind)
	if err != nil || !ok {
		return "", ok, err
	}
	return folder.Path(), true, nil
}

// folderByKind is the folder-valued form of folderPathByKind, for callers that need the folder's id
// as well as its path (predicting a moved message's new id needs the destination folder id). The
// same missing-is-not-an-error contract applies.
func folderByKind(ctx context.Context, store folderLister, accountID string, kind domain.FolderKind) (domain.Folder, bool, error) {
	folders, err := store.ListFolders(ctx, accountID)
	if err != nil {
		return domain.Folder{}, false, err
	}
	for _, folder := range folders {
		if folder.Kind() == kind {
			return folder, true, nil
		}
	}
	return domain.Folder{}, false, nil
}
