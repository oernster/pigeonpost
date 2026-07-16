import {Dispatch, SetStateAction, useCallback, useState} from 'react'
import {api, Folder, Message} from '../api'
import {OUTBOX_FOLDER_ID, isOutboxMessage} from '../outbox'
import type {MoveItem} from '../undoStack'
import type {MessageStore} from './useMessageStore'
import type {Selection} from './useSelection'
import type {UndoRecorder} from './useUndoRedo'

// BulkActionsDeps is what the multi-selection actions need from the rest of App: the message store they
// mutate, the selection they read and clear, the folder list (a drop target's account gates which rows
// may move onto it), the unread-count refresher, the folder-list refresher, the error sink and the
// undo recorder each completed bulk action reports to.
export interface BulkActionsDeps {
    store: MessageStore
    selection: Selection
    folders: Folder[]
    loadUnread: () => Promise<void>
    refreshFolders: () => Promise<void>
    setError: (message: string) => void
    undo: UndoRecorder
}

export interface BulkActions {
    bulkToDelete: Message[] | null
    setBulkToDelete: Dispatch<SetStateAction<Message[] | null>>
    bulkDeleting: boolean
    bulkToPurge: Message[] | null
    setBulkToPurge: Dispatch<SetStateAction<Message[] | null>>
    bulkPurging: boolean
    runBulkDelete: (targets: Message[], permanent: boolean) => Promise<void>
    bulkSetRead: (targets: Message[], read: boolean) => Promise<void>
    bulkSetFlag: (targets: Message[], flagged: boolean) => Promise<void>
    bulkMove: (targets: Message[], destFolderId: string) => void
    // bulkMoveIds is the id-level batched move behind bulkMove and drag-and-drop, exposed so the
    // message clipboard's paste-of-a-cut reuses its undo recording and destination sync.
    bulkMoveIds: (ids: string[], destFolderId: string) => Promise<void>
    dropMessageOnFolder: (messageId: string, folderId: string) => void
}

// useBulkActions owns the actions over a multi-selection (bulk delete, permanent delete, move, read, flag)
// and the drag-and-drop-onto-folder handler, plus the bulk-delete and bulk-purge confirm state. Every list
// change goes through the message store, so it shows wherever a message appears, and the selection is
// cleared after a delete or move. The single-message actions live in useMessageActions.
export function useBulkActions(deps: BulkActionsDeps): BulkActions {
    const {store, selection, folders, loadUnread, refreshFolders, setError, undo} = deps
    const {
        messages, searchResults, setMessages, setSearchResults, setSelectedMessage,
        applyToAllLists, removeFromAllLists,
    } = store
    const {markedIds, setMarkedIds, setAnchorId} = selection

    const [bulkToDelete, setBulkToDelete] = useState<Message[] | null>(null)
    const [bulkDeleting, setBulkDeleting] = useState<boolean>(false)
    const [bulkToPurge, setBulkToPurge] = useState<Message[] | null>(null)
    const [bulkPurging, setBulkPurging] = useState<boolean>(false)

    // removeIdsFromLists drops a set of message ids from every on-screen list and the selection after a
    // bulk delete or move, and clears the active message if it was among them. All the setters are stable,
    // so it needs no dependencies.
    const removeIdsFromLists = useCallback((ids: Set<string>) => {
        removeFromAllLists(ids)
        setMarkedIds(new Set())
        setAnchorId(null)
    }, [removeFromAllLists])

    // sourceFolderOf resolves the folder a message sits in before a bulk action, from whichever
    // on-screen list carries it, so the undo entry knows where to return it.
    const sourceFolderOf = useCallback((id: string): string => {
        const source = messages.find((m) => m.id === id) ?? searchResults.find((m) => m.id === id)
        return source?.folderId ?? ''
    }, [messages, searchResults])

    // recordBulkMove records a completed bulk move or bulk delete for Edit > Undo: one entry for
    // the whole batch, holding each message's new id (where the server reported one) and the folder
    // it came from. Messages the server did not locate are left out, so undo only promises what it
    // can deliver; when none were located nothing is recorded.
    const recordBulkMove = useCallback((
        flavour: 'move' | 'delete', actedIds: string[], newIds: Record<string, string>,
        sources: Map<string, string>, destFolderId: string,
    ) => {
        const items: MoveItem[] = []
        for (const id of actedIds) {
            const newId = newIds?.[id] ?? ''
            const sourceFolderId = sources.get(id) ?? ''
            if (newId !== '' && sourceFolderId !== '') {
                items.push({messageId: newId, sourceFolderId})
            }
        }
        if (items.length > 0) {
            undo.push({kind: 'move', flavour, items, destFolderId})
        }
    }, [undo])

    // bulkMoveIds moves several messages into a folder in ONE batched backend call (grouped by source
    // folder on the server), rather than a request per message, so a large Gmail selection stays under
    // its simultaneous-connection cap. Shared by drag-and-drop and the bulk "Move to" menu.
    const bulkMoveIds = useCallback(async (ids: string[], destFolderId: string) => {
        if (ids.length === 0 || destFolderId === OUTBOX_FOLDER_ID) {
            return
        }
        setError('')
        // The source folders are read before the move: afterwards the rows are gone from the lists.
        const sources = new Map(ids.map((id) => [id, sourceFolderOf(id)]))
        try {
            const result = await api.moveMessages(ids, destFolderId)
            removeIdsFromLists(new Set(result.ids))
            recordBulkMove('move', result.ids, result.newIds, sources, destFolderId)
            if (result.error) {
                setError(`${result.failed} of ${ids.length} messages could not be moved: ${result.error}`)
            }
            if (result.ids.length > 0) {
                // Pull the destination's listing at once so the moved messages appear there (and
                // count toward its unread badge) immediately; a failure here leaves the next
                // background sync to reconcile.
                try {
                    await api.syncFolder(destFolderId)
                } catch {
                    // Reconciled by the next background sync.
                }
            }
        } catch (e) {
            setError(`Move failed: ${String(e)}`)
        }
        await loadUnread()
        await refreshFolders()
    }, [removeIdsFromLists, sourceFolderOf, recordBulkMove, loadUnread, refreshFolders])

    // dropMessageOnFolder is the drag-and-drop target handler. Dropping a row that is part of the
    // multi-selection moves the whole selection; dropping any other row moves just that one. Messages
    // already in the target folder, synthetic outbox items and rows belonging to a different account
    // than the target folder (a unified-list row cannot move across accounts) are skipped. The move is
    // batched, so a large drop stays under Gmail's connection cap.
    const dropMessageOnFolder = useCallback((messageId: string, folderId: string) => {
        if (folderId === OUTBOX_FOLDER_ID) {
            return
        }
        const targetAccount = folders.find((f) => f.id === folderId)?.accountId ?? ''
        const ids = markedIds.has(messageId) && markedIds.size > 1 ? [...markedIds] : [messageId]
        const movable = ids.filter((id) => {
            const source = messages.find((m) => m.id === id) ?? searchResults.find((m) => m.id === id)
            return source !== undefined && source.folderId !== folderId && !isOutboxMessage(source)
                && (!source.accountId || source.accountId === targetAccount)
        })
        setMarkedIds(new Set())
        setAnchorId(null)
        void bulkMoveIds(movable, folderId)
    }, [markedIds, messages, searchResults, folders, bulkMoveIds])

    // runBulkDelete carries out a confirmed bulk delete or permanent delete over the selected messages in
    // one batched backend call: the server groups them by folder and issues a single delete per folder,
    // rather than a fresh connection per message. The result reports which ids were removed (dropped from
    // the on-screen lists) and how many failed, so a partial delete is never silent.
    const runBulkDelete = useCallback(async (targets: Message[], permanent: boolean) => {
        if (targets.length === 0) {
            return
        }
        const setBusy = permanent ? setBulkPurging : setBulkDeleting
        setBusy(true)
        setError('')
        const ids = targets.map((m) => m.id)
        const sources = new Map(targets.map((m) => [m.id, m.folderId]))
        try {
            const result = permanent
                ? await api.deleteMessagesPermanent(ids)
                : await api.deleteMessages(ids)
            removeIdsFromLists(new Set(result.ids))
            if (!permanent) {
                recordBulkMove('delete', result.ids, result.newIds, sources, '')
            }
            if (result.error) {
                setError(`${result.failed} of ${targets.length} messages could not be deleted: ${result.error}`)
            }
        } catch (e) {
            setError(`Bulk delete failed: ${String(e)}`)
        } finally {
            if (permanent) {
                setBulkToPurge(null)
            } else {
                setBulkToDelete(null)
            }
            setBusy(false)
        }
        await loadUnread()
        await refreshFolders()
    }, [removeIdsFromLists, recordBulkMove, loadUnread, refreshFolders])

    // bulkSetRead sets the read flag on every selected message, updating the lists at once and then
    // persisting each. bulkSetFlag does the same for the star. Both take an explicit value rather than
    // toggling, so a mixed selection ends up uniform.
    const bulkSetRead = useCallback(async (targets: Message[], read: boolean) => {
        const ids = new Set(targets.map((t) => t.id))
        applyToAllLists((m) => (ids.has(m.id) ? {...m, read} : m))
        let failed = 0
        for (const t of targets) {
            try {
                await api.markRead(t.id, read)
            } catch {
                failed += 1
            }
        }
        // One undo entry for the whole batch, remembering each message's prior value so a mixed
        // selection is restored message by message. Rows already at the target value have nothing
        // to restore and are left out.
        const changed = targets.filter((t) => t.read !== read)
        if (changed.length > 0) {
            undo.push({kind: 'read', items: changed.map((t) => ({messageId: t.id, before: t.read})), after: read})
        }
        try {
            await loadUnread()
        } catch {
            // A count refresh is best effort; the optimistic list update already reflects the change.
        }
        if (failed > 0) {
            setError(`${failed} of ${targets.length} messages could not be updated on the server.`)
        }
    }, [undo, loadUnread])

    const bulkSetFlag = useCallback(async (targets: Message[], flagged: boolean) => {
        const ids = new Set(targets.map((t) => t.id))
        const apply = (m: Message): Message => (ids.has(m.id) ? {...m, flagged} : m)
        setMessages((prev) => prev.map(apply))
        setSearchResults((prev) => prev.map(apply))
        setSelectedMessage((prev) => (prev && ids.has(prev.id) ? {...prev, flagged} : prev))
        let failed = 0
        for (const t of targets) {
            try {
                await api.markFlagged(t.id, flagged)
            } catch {
                failed += 1
            }
        }
        // Mirror bulkSetRead: one entry restoring each message's prior star.
        const changed = targets.filter((t) => t.flagged !== flagged)
        if (changed.length > 0) {
            undo.push({kind: 'flag', items: changed.map((t) => ({messageId: t.id, before: t.flagged})), after: flagged})
        }
        if (failed > 0) {
            setError(`${failed} of ${targets.length} messages could not be updated on the server.`)
        }
    }, [undo])

    // bulkMove moves every selected message into the destination folder in one batched call, skipping any
    // already there and any synthetic outbox item.
    const bulkMove = useCallback((targets: Message[], destFolderId: string) => {
        const ids = targets
            .filter((t) => t.folderId !== destFolderId && !isOutboxMessage(t))
            .map((t) => t.id)
        void bulkMoveIds(ids, destFolderId)
    }, [bulkMoveIds])

    return {
        bulkToDelete, setBulkToDelete, bulkDeleting,
        bulkToPurge, setBulkToPurge, bulkPurging,
        runBulkDelete, bulkSetRead, bulkSetFlag, bulkMove, bulkMoveIds, dropMessageOnFolder,
    }
}
