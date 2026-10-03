package imap

import (
	"errors"
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// ErrNoSelectiveExpunge reports a server that offers no UID EXPUNGE (neither UIDPLUS nor IMAP4rev2), so
// the only way to remove a message is a plain EXPUNGE that would also destroy every other message
// flagged \Deleted in the folder. The adapter refuses rather than risk that; nothing was changed.
var ErrNoSelectiveExpunge = errors.New("imap: the server cannot remove only the chosen messages (no UIDPLUS); nothing was changed")

// markDeleted is the silent STORE that flags messages \Deleted ahead of their expunge.
var markDeleted = &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}

// moveUIDs relocates exactly the given UIDs from the selected folder to dest and answers each source
// UID's destination UID where the server reports them via COPYUID.
//
// With MOVE (or IMAP4rev2) it sends MOVE. Without it, it does not use go-imap's Move fallback: that
// writes COPY, STORE \Deleted and EXPUNGE in one go before COPY's answer is read, so a refused copy
// (a full Trash, a vanished destination) still expunged the message. Here the COPY is answered first;
// only a copy the server accepted is then flagged and expunged, always by UID. A server that cannot
// expunge by UID is refused before anything is sent (see ErrNoSelectiveExpunge).
func moveUIDs(client *imapclient.Client, set imap.UIDSet, dest string) (map[string]string, error) {
	caps := client.Caps()
	if caps.Has(imap.CapMove) {
		data, err := client.Move(set, dest).Wait()
		if err != nil {
			return nil, err
		}
		return movedUIDs(data), nil
	}
	if !caps.Has(imap.CapUIDPlus) {
		return nil, ErrNoSelectiveExpunge
	}
	data, err := client.Copy(set, dest).Wait()
	if err != nil {
		return nil, err
	}
	if err := expungeUIDs(client, set); err != nil {
		return nil, fmt.Errorf("copied to %q but not removed from the source: %w", dest, err)
	}
	return copiedUIDs(data), nil
}

// expungeUIDs removes exactly the given UIDs from the selected folder: STORE \Deleted on them, then
// UID EXPUNGE of the same set, so a message another client flagged \Deleted without expunging (as
// Thunderbird's "mark as deleted" mode leaves them) survives. Without UIDPLUS it sends nothing and
// answers ErrNoSelectiveExpunge, since a plain EXPUNGE would take those messages too.
func expungeUIDs(client *imapclient.Client, set imap.UIDSet) error {
	if !client.Caps().Has(imap.CapUIDPlus) {
		return ErrNoSelectiveExpunge
	}
	if err := client.Store(set, markDeleted, nil).Close(); err != nil {
		return fmt.Errorf("mark \\Deleted: %w", err)
	}
	if err := client.UIDExpunge(set).Close(); err != nil {
		return fmt.Errorf("expunge: %w", err)
	}
	return nil
}

// moveAllUIDs relocates every message in the selected folder to dest without MOVE: it learns the
// folder's UIDs with UID SEARCH and moves them through moveUIDs in bulk-sized chunks, so the fallback
// expunges only what it copied, never a message that arrived in between.
func moveAllUIDs(client *imapclient.Client, dest string) error {
	if !client.Caps().Has(imap.CapUIDPlus) {
		return ErrNoSelectiveExpunge
	}
	found, err := client.UIDSearch(&imap.SearchCriteria{}, nil).Wait()
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	for _, chunk := range chunkUIDs(found.AllUIDs()) {
		if _, err := moveUIDs(client, chunk.set, dest); err != nil {
			return fmt.Errorf("%d messages: %w", chunk.count, err)
		}
	}
	return nil
}
