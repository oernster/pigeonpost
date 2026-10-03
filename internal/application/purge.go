package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// purgeViaTrash permanently deletes uids from folder by moving them into the account's Trash and
// expunging them from there, the route a provider needs when an in-place expunge does not delete.
//
// On an ordinary IMAP server a permanent delete is one step: mark \Deleted and expunge where the message
// stands. Gmail's mailboxes are labels, so an expunge removes the label the message was expunged from and
// then applies the account's own IMAP setting, which defaults to archiving rather than deleting. The
// message survives in All Mail. A rule documented as irreversible therefore kept everything it claimed to
// destroy, silently; 30 LinkedIn messages a destroying rule had "deleted" were found intact. Moving to
// the Bin and expunging there is the route Gmail does honour as a deletion.
//
// It reports whether it handled the deletion. False means the caller should take its own single-step
// route, which happens in three cases: a provider that deletes on an expunge like an ordinary IMAP server,
// a folder that IS the Trash, where an expunge already deletes and a hop into itself would be a no-op,
// plus an account with no Trash folder at all, where there is nowhere to hop to.
//
// A message the server moves without saying where it landed cannot be addressed in the Bin afterwards, so
// it is left there and named in the error rather than counted as destroyed. That is recoverable and
// stated, which is the whole complaint about the behaviour this replaces.
func purgeViaTrash(ctx context.Context, remote MailActions, store folderLister,
	account domain.Account, folder domain.Folder, uids []string) (bool, error) {
	handled, _, err := purgeViaTrashReporting(ctx, remote, store, account, folder, uids)
	return handled, err
}

// purgeViaTrashReporting is purgeViaTrash that also returns the uids that left folder, so a caller holding
// cached rows can drop exactly those even when it also reports an error. A move into Trash refused part
// way still moved what it accepted; those are purged from Trash all the same and counted as gone. A
// message moved but not located in Trash has also left folder, as has one whose purge there failed: it
// now sits in Trash, where the next sync lists it, so it counts as gone from folder while the error
// names it.
func purgeViaTrashReporting(ctx context.Context, remote MailActions, store folderLister,
	account domain.Account, folder domain.Folder, uids []string) (bool, []string, error) {
	if !account.ExpungeArchivesInPlace() || folder.Kind() == domain.FolderTrash {
		return false, nil, nil
	}
	trash, ok, err := folderByKind(ctx, store, account.ID(), domain.FolderTrash)
	if err != nil {
		return false, nil, fmt.Errorf("resolve trash for a permanent delete in %q: %w", folder.Path(), err)
	}
	if !ok {
		return false, nil, nil
	}

	moved, moveErr := remote.DeleteMany(ctx, account, folder, uids, trash.Path())
	var errs []error
	left := landedUIDs(uids, moved, moveErr)
	if moveErr != nil {
		errs = append(errs, fmt.Errorf("move message(s) from %q to %q: %d of %d accepted: %w",
			folder.Path(), trash.Path(), len(left), len(uids), moveErr))
	}

	inTrash := make([]string, 0, len(left))
	stranded := 0
	for _, uid := range left {
		if newUID := moved[uid]; newUID != "" {
			inTrash = append(inTrash, newUID)
			continue
		}
		stranded++
	}
	if len(inTrash) > 0 {
		if _, err := remote.DeleteMany(ctx, account, trash, inTrash, ""); err != nil {
			errs = append(errs, fmt.Errorf("purge %d message(s) from %q: %w", len(inTrash), trash.Path(), err))
		}
	}
	if stranded > 0 {
		errs = append(errs, fmt.Errorf("moved %d message(s) to %q that the server did not locate there, so they could not be purged", stranded, trash.Path()))
	}
	return true, left, errors.Join(errs...)
}
