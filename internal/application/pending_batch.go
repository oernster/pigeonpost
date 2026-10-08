package application

import (
	"context"

	"github.com/oernster/pigeonpost/internal/domain"
)

// pendingBatch is the pending intents that share one folder, one key and one value: what one batched push
// carries over one connection. The key is whatever the intent changes (a flag, a tag); messageIDs maps
// each UID back to the message whose intent it settles.
type pendingBatch[K comparable] struct {
	account    domain.Account
	folder     domain.Folder
	key        K
	set        bool
	uids       []string
	messageIDs map[string]string
}

// pendingIntent is one intent as the batching sees it: the message it is for, what it changes and to what.
type pendingIntent[K comparable] struct {
	messageID string
	key       K
	set       bool
}

// batchPending groups the intents by folder, key and value, in the order each batch is first met. Both
// the flag and the tag replay go through it, so neither can drift back to one connection per intent. A
// message's context is resolved now, so the push targets its current UID even when that has changed
// since the intent was recorded; a message that cannot be resolved is left out.
func batchPending[K comparable](ctx context.Context, store MailStore, accounts AccountStore, intents []pendingIntent[K]) []*pendingBatch[K] {
	type groupKey struct {
		folderID string
		key      K
		set      bool
	}
	byKey := make(map[groupKey]*pendingBatch[K])
	var batches []*pendingBatch[K]
	for _, intent := range intents {
		msg, folder, account, err := resolveMessageContext(ctx, store, accounts, intent.messageID)
		if err != nil {
			continue
		}
		k := groupKey{folderID: folder.ID(), key: intent.key, set: intent.set}
		batch, ok := byKey[k]
		if !ok {
			batch = &pendingBatch[K]{account: account, folder: folder, key: intent.key, set: intent.set, messageIDs: map[string]string{}}
			byKey[k] = batch
			batches = append(batches, batch)
		}
		batch.uids = append(batch.uids, msg.UID())
		batch.messageIDs[msg.UID()] = intent.messageID
	}
	return batches
}
