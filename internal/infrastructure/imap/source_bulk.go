package imap

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

// bulkBatchSize caps how many UIDs go into a single MOVE or STORE command. The whole operation still
// runs over one connection; chunking only keeps each command's line length within server limits, since
// a bulk delete or move can carry thousands of UIDs (Gmail "All Mail" selections in particular).
const bulkBatchSize = 500

// uidChunk is one command's worth of UIDs: the set to send plus how many it holds, for the error text.
type uidChunk struct {
	set   imap.UIDSet
	count int
}

// uidChunks parses the uids (failing on the first malformed one) and splits them into sets of at most
// bulkBatchSize, so each bulk command stays within server line-length limits.
func uidChunks(uids []string) ([]uidChunk, error) {
	nums, err := parseUIDs(uids)
	if err != nil {
		return nil, err
	}
	return chunkUIDs(nums), nil
}

// chunkUIDs splits already-parsed UIDs into sets of at most bulkBatchSize. uidChunks is its form for
// UID strings; the move-all fallback feeds it the UIDs a search found.
func chunkUIDs(nums []imap.UID) []uidChunk {
	chunks := make([]uidChunk, 0, len(nums)/bulkBatchSize+1)
	for start := 0; start < len(nums); start += bulkBatchSize {
		end := min(start+bulkBatchSize, len(nums))
		set := imap.UIDSet{}
		for _, u := range nums[start:end] {
			set.AddNum(u)
		}
		chunks = append(chunks, uidChunk{set: set, count: end - start})
	}
	return chunks
}

// openFolder connects to the account and selects the folder, the preamble every action shares. The
// caller logs the client out when done.
func (s *Source) openFolder(ctx context.Context, account domain.Account, folder domain.Folder) (*imapclient.Client, error) {
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		_ = client.Logout().Wait()
		return nil, fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}
	return client, nil
}

// storeFlagMany adds or removes one flag on several messages in one folder over a single connection,
// issued in UID chunks. storeFlag is its one-message case.
func (s *Source) storeFlagMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, flag imap.Flag, set bool) error {
	if len(uids) == 0 {
		return nil
	}
	chunks, err := uidChunks(uids)
	if err != nil {
		return err
	}
	client, err := s.openFolder(ctx, account, folder)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	op := imap.StoreFlagsDel
	if set {
		op = imap.StoreFlagsAdd
	}
	store := &imap.StoreFlags{Op: op, Silent: true, Flags: []imap.Flag{flag}}
	for _, chunk := range chunks {
		if err := client.Store(chunk.set, store, nil).Close(); err != nil {
			return fmt.Errorf("imap: store %s on %d messages: %w", flag, chunk.count, err)
		}
	}
	return nil
}

// SetSeenMany sets or clears \Seen on several messages in one folder with one login, the batched form
// of SetSeen: a bulk mark-read costs one connection per folder rather than one per message. It
// satisfies application.MailActions.
func (s *Source) SetSeenMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, seen bool) error {
	return s.storeFlagMany(ctx, account, folder, uids, imap.FlagSeen, seen)
}

// DeleteMany removes several messages in one folder over a single connection: it selects the folder
// once, then moves them to trashPath (see moveUIDs). With trashPath empty it instead marks them
// \Deleted and expunges exactly those UIDs (see expungeUIDs). Both are issued in UID chunks so one
// command never grows too long. This is the batched form of Delete: a bulk delete costs one login for
// the whole selection rather than one per message, which is what keeps it under Gmail's
// simultaneous-connection cap. When the messages moved to trashPath it returns each source UID's
// destination UID where the server reports them via COPYUID, together with any error, for the chunks
// moved before it. Permanent deletion maps each destroyed UID to an empty destination, likewise for
// the chunks destroyed before an error. It satisfies application.MailActions.
func (s *Source) DeleteMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, trashPath string) (map[string]string, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	chunks, err := uidChunks(uids)
	if err != nil {
		return nil, err
	}
	client, err := s.openFolder(ctx, account, folder)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout().Wait() }()

	if trashPath != "" {
		return moveChunks(client, chunks, trashPath)
	}
	// Permanent delete: flag this chunk \Deleted then expunge it. Expunging per chunk rather than once at
	// the end keeps each EXPUNGE bounded; a single expunge of a very large mailbox (a full Gmail Bin, say)
	// holds the connection long enough that the server drops it with an unexpected EOF.
	//
	// A chunk refused part way through the selection leaves the earlier chunks already expunged, so the
	// returned map names their UIDs (with no destination: they landed nowhere) together with the error;
	// the caller drops exactly those rather than showing them at UIDs that no longer exist. The chunks are
	// cut from uids in order, so each one covers the next chunk.count of them.
	destroyed := map[string]string{}
	done := 0
	for _, chunk := range chunks {
		if err := expungeUIDs(client, chunk.set); err != nil {
			return destroyed, fmt.Errorf("imap: delete %d messages: %w", chunk.count, err)
		}
		for _, uid := range uids[done : done+chunk.count] {
			destroyed[uid] = ""
		}
		done += chunk.count
	}
	return destroyed, nil
}

// MoveMany relocates several messages from one folder to destPath over a single connection, issued in
// UID chunks so one command never grows too long. It is the batched form of Move: a bulk move or a
// drag-and-drop of a selection costs one login for the whole selection rather than one per message,
// which is what keeps it under Gmail's simultaneous-connection cap. It returns each source UID's
// destination UID where the server reports them via COPYUID. It satisfies application.MailActions.
func (s *Source) MoveMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, destPath string) (map[string]string, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	chunks, err := uidChunks(uids)
	if err != nil {
		return nil, err
	}
	client, err := s.openFolder(ctx, account, folder)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout().Wait() }()
	return moveChunks(client, chunks, destPath)
}

// moveChunks moves each chunk to destPath through moveUIDs and gathers the source-to-destination UIDs.
// On a refused chunk it returns the map of the chunks moved before it together with the error, so the
// caller can keep what did move (a chunk copied but not removed is not counted: it is still in the
// source).
func moveChunks(client *imapclient.Client, chunks []uidChunk, destPath string) (map[string]string, error) {
	moved := map[string]string{}
	for _, chunk := range chunks {
		landed, err := moveUIDs(client, chunk.set, destPath)
		if err != nil {
			return moved, fmt.Errorf("imap: move %d messages to %q: %w", chunk.count, destPath, err)
		}
		for src, dst := range landed {
			moved[src] = dst
		}
	}
	return moved, nil
}

// parseUIDs parses each uid string, failing on the first malformed one. It is the batched form of
// parseUID shared by DeleteMany and MoveMany.
func parseUIDs(uids []string) ([]imap.UID, error) {
	nums := make([]imap.UID, 0, len(uids))
	for _, uid := range uids {
		u, err := parseUID(uid)
		if err != nil {
			return nil, err
		}
		nums = append(nums, u)
	}
	return nums, nil
}
