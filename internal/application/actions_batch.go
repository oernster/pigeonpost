package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// folderBatch is one folder's share of a bulk action: the folder, its account and the messages in it.
type folderBatch struct {
	account domain.Account
	folder  domain.Folder
	ids     []string
	uids    []string
}

// batchRules shapes batchByFolder for one bulk action. Every hook is optional.
type batchRules struct {
	// skip leaves a message out silently: a move leaves out what is already in the destination.
	skip func(msg domain.MessageSummary) bool
	// admit refuses a message in the given folder with an error, before the folder's account is looked
	// up: a move refuses a folder in another account.
	admit func(messageID string, folder domain.Folder) error
	// prepare runs once, when a folder's batch is first formed: a delete resolves the folder's Trash.
	prepare func(b *folderBatch) error
}

// batchByFolder resolves each message and groups them by folder in first-seen order, so a bulk action
// can make one server round trip per folder. A message that cannot be resolved (or that admit or
// prepare refuses) is left out and contributes its own error, so one bad row never sinks the rest. A
// folder whose lookup fails is not remembered: the next message from it is tried afresh and reports
// its own error, so every message left out is accounted for.
func (s *MessageActionService) batchByFolder(ctx context.Context, messageIDs []string, rules batchRules) ([]*folderBatch, []error) {
	byFolder := map[string]*folderBatch{}
	batches := make([]*folderBatch, 0)
	var errs []error
	for _, id := range messageIDs {
		msg, err := s.store.GetMessage(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("locate message %q: %w", id, err))
			continue
		}
		if rules.skip != nil && rules.skip(msg) {
			continue
		}
		b, ok := byFolder[msg.FolderID()]
		if !ok {
			b, err = s.formBatch(ctx, id, msg.FolderID(), rules)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			byFolder[msg.FolderID()] = b
			batches = append(batches, b)
		}
		b.ids = append(b.ids, id)
		b.uids = append(b.uids, msg.UID())
	}
	return batches, errs
}

// formBatch looks up a folder and its account, applying admit before the account and prepare after,
// and answers an empty batch for it.
func (s *MessageActionService) formBatch(ctx context.Context, messageID, folderID string, rules batchRules) (*folderBatch, error) {
	folder, err := s.store.GetFolder(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("locate folder %q: %w", folderID, err)
	}
	if rules.admit != nil {
		if err := rules.admit(messageID, folder); err != nil {
			return nil, err
		}
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return nil, fmt.Errorf("locate account %q: %w", folder.AccountID(), err)
	}
	b := &folderBatch{account: account, folder: folder}
	if rules.prepare != nil {
		if err := rules.prepare(b); err != nil {
			return nil, err
		}
	}
	return b, nil
}
