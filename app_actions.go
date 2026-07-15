// app_actions.go carries the message-action slice of the Wails facade: the flag, junk, delete,
// move and copy operations the front end invokes on cached messages. Split from app.go to keep
// each facade file within the line budget.
package main

// MarkRead sets or clears a message's read (Seen) state on the server and in the local cache.
func (a *App) MarkRead(messageID string, read bool) error {
	return a.actions.MarkRead(a.ctx, messageID, read)
}

// MarkFlagged sets or clears a message's flagged (starred) state on the server and in the local cache.
func (a *App) MarkFlagged(messageID string, flagged bool) error {
	return a.actions.MarkFlagged(a.ctx, messageID, flagged)
}

// MarkReplied records that a message has been replied to, setting \Answered on the server and in the local
// cache. The front end calls it after a reply is sent, so the original row shows the replied indicator.
func (a *App) MarkReplied(messageID string) error {
	return a.actions.MarkAnswered(a.ctx, messageID, true)
}

// MarkForwarded records that a message has been forwarded, setting the $Forwarded keyword on the server and in
// the local cache. The front end calls it after a forward is sent.
func (a *App) MarkForwarded(messageID string) error {
	return a.actions.MarkForwarded(a.ctx, messageID, true)
}

// DeleteMessage removes a message: it is moved to Trash where one exists, otherwise deleted
// permanently. The local cache is updated to match. The result carries the id the message will
// hold in Trash where the server reported it, so the front end can offer an undo.
func (a *App) DeleteMessage(messageID string) (MoveResultDTO, error) {
	newID, err := a.actions.Delete(a.ctx, messageID)
	return MoveResultDTO{NewId: newID}, err
}

// DeleteMessagePermanent removes a message immediately and irreversibly, without moving it to Trash,
// regardless of which folder it lives in. The local cache is updated to match.
func (a *App) DeleteMessagePermanent(messageID string) error {
	return a.actions.DeletePermanent(a.ctx, messageID)
}

// DeleteMessages removes several messages in one batched pass per folder, far faster than a call per
// message: each is moved to Trash where the account has one, otherwise deleted permanently. It returns
// which ids were removed (so the UI drops exactly those) plus any error text, rather than failing the
// whole call, so a partial success still reports what went.
func (a *App) DeleteMessages(ids []string) BulkResultDTO {
	return a.bulkDelete(ids, false)
}

// DeleteMessagesPermanent removes several messages immediately and irreversibly, without moving them to
// Trash. It is the batched counterpart to DeleteMessagePermanent.
func (a *App) DeleteMessagesPermanent(ids []string) BulkResultDTO {
	return a.bulkDelete(ids, true)
}

// bulkDelete runs the batched delete and packs its outcome into the DTO the front end reads.
func (a *App) bulkDelete(ids []string, permanent bool) BulkResultDTO {
	deleted, newIDs, err := a.actions.DeleteMany(a.ctx, ids, permanent)
	return bulkResult(ids, deleted, newIDs, err)
}

// MoveMessages relocates several messages into one folder in a single batched pass per source folder,
// far faster than a call per message: this is what keeps a drag-and-drop or a bulk "Move to" of a large
// Gmail selection under Gmail's simultaneous-connection cap. It returns which ids moved so the UI drops
// exactly those, plus any error text.
func (a *App) MoveMessages(ids []string, destFolderID string) BulkResultDTO {
	moved, newIDs, err := a.actions.MoveMany(a.ctx, ids, destFolderID)
	return bulkResult(ids, moved, newIDs, err)
}

// bulkResult packs a batched action's outcome (the ids acted on, where each landed and any error)
// into the DTO the front end reads, reporting how many of the requested ids were not processed.
func bulkResult(requested, acted []string, newIDs map[string]string, err error) BulkResultDTO {
	result := BulkResultDTO{Ids: acted, Failed: len(requested) - len(acted), NewIds: newIDs}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// MoveMessage relocates a message to another folder in the same account. The result carries the id
// the message will hold in the destination where the server reported it, so the front end can offer
// an undo.
func (a *App) MoveMessage(messageID, destFolderID string) (MoveResultDTO, error) {
	newID, err := a.actions.Move(a.ctx, messageID, destFolderID)
	return MoveResultDTO{NewId: newID}, err
}

// MarkJunk moves a message to the account's Junk folder, filing it out of the inbox as spam. It returns
// an error when the account has no Junk folder or the message already lives there. The result carries
// the id the message will hold in Junk where the server reported it, so the front end can offer an undo.
func (a *App) MarkJunk(messageID string) (MoveResultDTO, error) {
	newID, err := a.actions.MarkJunk(a.ctx, messageID)
	return MoveResultDTO{NewId: newID}, err
}

// MarkNotJunk rescues a message from the account's Junk folder back to its Inbox, clearing the junk
// keywords on the server. It returns an error when the message is not in Junk or the account has no
// Inbox folder. The result carries the id the message will hold in the Inbox where the server
// reported it, so the front end can offer an undo.
func (a *App) MarkNotJunk(messageID string) (MoveResultDTO, error) {
	newID, err := a.actions.MarkNotJunk(a.ctx, messageID)
	return MoveResultDTO{NewId: newID}, err
}

// CopyMessage duplicates a message into another folder in the same account, leaving the original.
func (a *App) CopyMessage(messageID, destFolderID string) error {
	return a.actions.Copy(a.ctx, messageID, destFolderID)
}
