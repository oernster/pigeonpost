package imap

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"

	"github.com/oernster/pigeonpost/internal/domain"
)

// bulkBatchSize caps how many UIDs go into a single MOVE or STORE command. The whole operation still
// runs over one connection; chunking only keeps each command's line length within server limits, since
// a bulk delete or move can carry thousands of UIDs (Gmail "All Mail" selections in particular).
const bulkBatchSize = 500

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
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return nil, fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	nums, err := parseUIDs(uids)
	if err != nil {
		return nil, err
	}

	moved := map[string]string{}
	store := &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}
	for start := 0; start < len(nums); start += bulkBatchSize {
		end := start + bulkBatchSize
		if end > len(nums) {
			end = len(nums)
		}
		set := imap.UIDSet{}
		for _, u := range nums[start:end] {
			set.AddNum(u)
		}
		if trashPath != "" {
			data, err := client.Move(set, trashPath).Wait()
			if err != nil {
				return moved, fmt.Errorf("imap: move %d messages to %q: %w", end-start, trashPath, err)
			}
			for src, dst := range movedUIDs(data) {
				moved[src] = dst
			}
			continue
		}
		// Permanent delete: flag this chunk \Deleted then expunge it. Expunging per chunk rather than once
		// at the end keeps each EXPUNGE bounded; a single expunge of a very large mailbox (a full Gmail Bin,
		// say) holds the connection long enough that the server drops it with an unexpected EOF.
		if err := client.Store(set, store, nil).Close(); err != nil {
			return nil, fmt.Errorf("imap: mark %d messages \\Deleted: %w", end-start, err)
		}
		if err := client.Expunge().Close(); err != nil {
			return nil, fmt.Errorf("imap: expunge %d messages: %w", end-start, err)
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
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return nil, fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	nums, err := parseUIDs(uids)
	if err != nil {
		return nil, err
	}

	moved := map[string]string{}
	for start := 0; start < len(nums); start += bulkBatchSize {
		end := start + bulkBatchSize
		if end > len(nums) {
			end = len(nums)
		}
		set := imap.UIDSet{}
		for _, u := range nums[start:end] {
			set.AddNum(u)
		}
		data, err := client.Move(set, destPath).Wait()
		if err != nil {
			return moved, fmt.Errorf("imap: move %d messages to %q: %w", end-start, destPath, err)
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
