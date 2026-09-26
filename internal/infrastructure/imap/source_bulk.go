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
	chunks := make([]uidChunk, 0, len(nums)/bulkBatchSize+1)
	for start := 0; start < len(nums); start += bulkBatchSize {
		end := min(start+bulkBatchSize, len(nums))
		set := imap.UIDSet{}
		for _, u := range nums[start:end] {
			set.AddNum(u)
		}
		chunks = append(chunks, uidChunk{set: set, count: end - start})
	}
	return chunks, nil
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
// once, then moves them to trashPath (MOVE) or marks them \Deleted and expunges (STORE then EXPUNGE)
// when trashPath is empty, issued in UID chunks so one command never grows too long. This is the
// batched form of Delete: a bulk delete costs one login for the whole selection rather than one per
// message, which is what keeps it under Gmail's simultaneous-connection cap. When the messages moved
// to trashPath it returns each source UID's destination UID where the server reports them via
// COPYUID; permanent deletion returns none. It satisfies application.MailActions.
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

	moved := map[string]string{}
	store := &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}
	for _, chunk := range chunks {
		if trashPath != "" {
			data, err := client.Move(chunk.set, trashPath).Wait()
			if err != nil {
				return moved, fmt.Errorf("imap: move %d messages to %q: %w", chunk.count, trashPath, err)
			}
			for src, dst := range movedUIDs(data) {
				moved[src] = dst
			}
			continue
		}
		// Permanent delete: flag this chunk \Deleted then expunge it. Expunging per chunk rather than once
		// at the end keeps each EXPUNGE bounded; a single expunge of a very large mailbox (a full Gmail Bin,
		// say) holds the connection long enough that the server drops it with an unexpected EOF.
		if err := client.Store(chunk.set, store, nil).Close(); err != nil {
			return nil, fmt.Errorf("imap: mark %d messages \\Deleted: %w", chunk.count, err)
		}
		if err := client.Expunge().Close(); err != nil {
			return nil, fmt.Errorf("imap: expunge %d messages: %w", chunk.count, err)
		}
	}
	return moved, nil
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

	moved := map[string]string{}
	for _, chunk := range chunks {
		data, err := client.Move(chunk.set, destPath).Wait()
		if err != nil {
			return moved, fmt.Errorf("imap: move %d messages to %q: %w", chunk.count, destPath, err)
		}
		for src, dst := range movedUIDs(data) {
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
