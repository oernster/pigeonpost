package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// A message's local id is its folder plus its UID; the rules treat an id the cache does not hold as an
// arrival. That holds only while the server keeps its numbering: when it renumbers a mailbox (a
// migration, an index rebuild, a folder deleted and recreated) every message comes back under a new UID
// and would read as new mail, so a destroying rule would wipe the backlog it promised never to touch.
// IMAP announces a renumbering by changing the folder's UIDVALIDITY. The sync therefore records that
// value per folder; when it changes, the sync rebaselines the folder: what the server now holds is
// taken as what the user already had, the cache is replaced with the new UIDs and the rules act on
// nothing that pass.
//
// The value is read through MailSource.FetchMessagesValidity and kept through MailStore's
// FolderUIDValidity pair. A protocol with no UIDVALIDITY (POP3) reports unknownUIDValidity, for which
// the sync keeps its earlier behaviour: nothing is compared and nothing is recorded.

// unknownUIDValidity stands for "the source reported none". RFC 3501 makes UIDVALIDITY a non-zero
// number, so zero can never be a real server's value.
const unknownUIDValidity uint32 = 0

// folderFetch is one folder's freshly fetched messages together with the UIDVALIDITY they were fetched
// under and whether that value differs from the one the folder was last synced under.
type folderFetch struct {
	messages   []domain.MessageSummary
	validity   uint32
	renumbered bool
}

// fetchFolder fetches a folder's messages and decides whether the server has renumbered it since the
// last sync. A folder synced before under the same numbering is refreshed from its listing (see
// sync_incremental.go); any other is fetched in full. A stored value that cannot be read fails the fetch
// rather than defaulting to "unchanged", which would hand a renumbered backlog to the rules.
func (s *SyncService) fetchFolder(ctx context.Context, account domain.Account, folder domain.Folder) (folderFetch, error) {
	if fetch, ok, err := s.fetchChanges(ctx, account, folder); err != nil || ok {
		return fetch, err
	}
	messages, validity, err := s.source.FetchMessagesValidity(ctx, account, folder)
	if err != nil {
		return folderFetch{}, err
	}
	if validity == unknownUIDValidity {
		return folderFetch{messages: messages, validity: validity}, nil
	}
	stored, has, err := s.mail.FolderUIDValidity(ctx, folder.ID())
	if err != nil {
		return folderFetch{}, fmt.Errorf("read UIDVALIDITY for %q: %w", folder.ID(), err)
	}
	return folderFetch{messages: messages, validity: validity, renumbered: has && stored != validity}, nil
}

// knownFor returns the set of ids the rules and the arrival notice treat as already held. On a
// renumbered folder that is everything just fetched, so nothing in it counts as an arrival. The same
// holds for a folder never baselined (see FolderBaselined): its first pass records a starting point,
// so a newly added account's existing inbox is neither announced as new mail nor handed to the rules,
// whichever pass (the front end's sync, an IDLE push or the poll) reaches it first. This is the one
// place the rebaseline is decided, shared by every sync path.
func (f folderFetch) knownFor(cached map[string]struct{}, baselined bool) map[string]struct{} {
	if f.renumbered || !baselined {
		return knownSetOf(f.messages)
	}
	return cached
}

// settleFolder records what a saved folder now stands for: its baseline mark, the pending intents for
// the flags the rules set on its arrivals (see recordRuleMarks), then the UIDVALIDITY its cached UIDs
// belong to. All run only after the save, so a failed save leaves the old value in place and the next
// pass rebaselines again rather than reading the new UIDs as arrivals.
func (s *SyncService) settleFolder(ctx context.Context, folder domain.Folder, validity uint32, marks []ruleMark) error {
	if err := s.markBaselined(ctx, folder); err != nil {
		return fmt.Errorf("mark baseline: %w", err)
	}
	if err := s.recordRuleMarks(ctx, marks); err != nil {
		return fmt.Errorf("record rule flags: %w", err)
	}
	if validity == unknownUIDValidity {
		return nil
	}
	if err := s.mail.SetFolderUIDValidity(ctx, folder.ID(), validity); err != nil {
		return fmt.Errorf("record UIDVALIDITY: %w", err)
	}
	return nil
}
