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
// message, which is what keeps it under Gmail's simultaneous-connection cap. It satisfies
// application.MailActions.
func (s *Source) DeleteMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, trashPath string) error {
	if len(uids) == 0 {
		return nil
	}
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	nums := make([]imap.UID, 0, len(uids))
	for _, uid := range uids {
		u, err := parseUID(uid)
		if err != nil {
			return err
		}
		nums = append(nums, u)
	}

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
			if _, err := client.Move(set, trashPath).Wait(); err != nil {
				return fmt.Errorf("imap: move %d messages to %q: %w", end-start, trashPath, err)
			}
			continue
		}
		if err := client.Store(set, store, nil).Close(); err != nil {
			return fmt.Errorf("imap: mark %d messages \\Deleted: %w", end-start, err)
		}
	}

	// A permanent delete expunges once at the end, after every chunk is flagged \Deleted.
	if trashPath == "" {
		if err := client.Expunge().Close(); err != nil {
			return fmt.Errorf("imap: expunge %d messages: %w", len(nums), err)
		}
	}
	return nil
}

// MoveMany relocates several messages from one folder to destPath over a single connection, issued in
// UID chunks so one command never grows too long. It is the batched form of Move: a bulk move or a
// drag-and-drop of a selection costs one login for the whole selection rather than one per message,
// which is what keeps it under Gmail's simultaneous-connection cap. It satisfies application.MailActions.
func (s *Source) MoveMany(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, destPath string) error {
	if len(uids) == 0 {
		return nil
	}
	client, err := s.connect(ctx, account)
	if err != nil {
		return err
	}
	defer func() { _ = client.Logout().Wait() }()

	if _, err := client.Select(folder.Path(), nil).Wait(); err != nil {
		return fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}

	nums := make([]imap.UID, 0, len(uids))
	for _, uid := range uids {
		u, err := parseUID(uid)
		if err != nil {
			return err
		}
		nums = append(nums, u)
	}

	for start := 0; start < len(nums); start += bulkBatchSize {
		end := start + bulkBatchSize
		if end > len(nums) {
			end = len(nums)
		}
		set := imap.UIDSet{}
		for _, u := range nums[start:end] {
			set.AddNum(u)
		}
		if _, err := client.Move(set, destPath).Wait(); err != nil {
			return fmt.Errorf("imap: move %d messages to %q: %w", end-start, destPath, err)
		}
	}
	return nil
}
