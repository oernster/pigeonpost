package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// Every sync used to fetch every message's summary in every folder it touched. Against Outlook.com that
// was measured at up to 40 seconds for 410 messages (2026-10-08), so a 54-folder account holding about
// 47,000 messages never finished a full sync; its Sent folder stopped refreshing on 2026-08-28 because
// no sync ever reached it.
//
// A folder synced before is now refreshed from a listing instead: the UID, flags and tag keywords of
// every message, without headers, which is small even for tens of thousands. Only the UIDs the cache
// does not hold have their summaries fetched; the rest keep their cached headers and take the listing's
// flags and keywords (domain.MergeListing). The result is the same full message set the rest of the sync
// already works on, so the rules, the reconciles and the save are untouched.
//
// The cached headers can only be trusted under the numbering they were cached with. So the listing is
// used only when the folder's UIDVALIDITY matches the one it was settled under. A folder never settled, a
// renumbered one and a protocol with no UIDVALIDITY (POP3) all take the full fetch, as does a folder
// renumbered between the listing and the fetch of its new UIDs.

// fetchChanges refreshes a folder from its listing. ok is false when the folder must be fetched in full
// instead, which the caller then does; an error is a failure of the listing path itself.
func (s *SyncService) fetchChanges(ctx context.Context, account domain.Account, folder domain.Folder) (fetch folderFetch, ok bool, err error) {
	listing, validity, err := s.source.FetchListing(ctx, account, folder)
	if err != nil {
		return folderFetch{}, false, err
	}
	if validity == unknownUIDValidity {
		return folderFetch{}, false, nil
	}
	stored, has, err := s.mail.FolderUIDValidity(ctx, folder.ID())
	if err != nil {
		return folderFetch{}, false, fmt.Errorf("read UIDVALIDITY for %q: %w", folder.ID(), err)
	}
	if !has || stored != validity {
		return folderFetch{}, false, nil
	}
	cached, err := s.mail.ListMessages(ctx, folder.ID())
	if err != nil {
		return folderFetch{}, false, fmt.Errorf("list cached messages for %q: %w", folder.ID(), err)
	}
	var fetched []domain.MessageSummary
	if uids := domain.NewUIDs(cached, listing); len(uids) > 0 {
		var fetchedUnder uint32
		fetched, fetchedUnder, err = s.source.FetchMessagesByUID(ctx, account, folder, uids)
		if err != nil {
			return folderFetch{}, false, err
		}
		if fetchedUnder != validity {
			return folderFetch{}, false, nil
		}
	}
	return folderFetch{messages: domain.MergeListing(cached, listing, fetched), validity: validity}, true, nil
}
