import {Dispatch, SetStateAction, useCallback, useRef, useState} from 'react'
import {api, Folder, Message} from '../api'
import {OUTBOX_FOLDER_ID, isOutboxMessage} from '../outbox'
import {putBack, takeOut, type Lifted} from '../optimisticList'
import type {MoveItem} from '../undoStack'
import type {MessageStore} from './useMessageStore'
import {Confirmation, deleteManyConfirmation, purgeManyConfirmation} from '../confirmations'
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
    // isPop3 is whether the selected account has no Trash, which the bulk delete confirmation has to say.
    isPop3: boolean
}

export interface BulkActions {
    // The two pending confirmations this hook owns, each null when nothing is pending.
    deleteConfirmation: Confirmation | null
    purgeConfirmation: Confirmation | null
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
    // dropMessageOnFolder reports whether the drop was taken, so the folder row only shows its landed
    // confirmation for a move that is actually happening.
    dropMessageOnFolder: (messageId: string, folderId: string) => boolean
}

// MoveSnapshot is what an optimistic removal took out of each on-screen list, kept only until the server
// answers so a refused or partial move can put the untouched rows back where they were.
interface MoveSnapshot {
    messages: Lifted[]
    searchResults: Lifted[]
    tabs: Lifted[]
    selected: Message | null
}

// useBulkActions owns the actions over a multi-selection (bulk delete, permanent delete, move, read, flag)
// and the drag-and-drop-onto-folder handler, plus the bulk-delete and bulk-purge confirm state. Every list
// change goes through the message store, so it shows wherever a message appears; the selection is
// cleared after a delete or move. The single-message actions live in useMessageActions.
export function useBulkActions(deps: BulkActionsDeps): BulkActions {
    const {store, selection, folders, loadUnread, refreshFolders, setError, undo, isPop3} = deps
    const {
        messages, searchResults, tabs, selectedMessage,
        setMessages, setSearchResults, setTabs, setSelectedMessage,
        applyToAllLists, removeFromAllLists,
    } = store
    const {markedIds, setMarkedIds, setAnchorId} = selection

    const [bulkToDelete, setBulkToDelete] = useState<Message[] | null>(null)
    const [bulkDeleting, setBulkDeleting] = useState<boolean>(false)
    const [bulkToPurge, setBulkToPurge] = useState<Message[] | null>(null)
    const [bulkPurging, setBulkPurging] = useState<boolean>(false)

    // inFlightIds holds the messages whose move the server has not answered yet, so a second drop of the
    // same message is ignored rather than issued twice. A ref, not state: the guard is read and written
    // inside one drop handler and must not wait for a re-render to take effect.
    const inFlightIds = useRef<Set<string>>(new Set())

    // removeIdsFromLists drops a set of message ids from every on-screen list and the selection after a
    // bulk delete or move, then clears the active message if it was among them. All the setters are stable,
    // so it needs no dependencies.
    const removeIdsFromLists = useCallback((ids: Set<string>) => {
        removeFromAllLists(ids)
        setMarkedIds(new Set())
        setAnchorId(null)
    }, [removeFromAllLists])

    // liftFromLists removes the ids from every on-screen list at once and returns what it took. It is what
    // makes a drop visible immediately: an IMAP move can take seconds on a slow provider; a list that
    // does not change reads as a drop that missed, so the user drags again and again. The rows leave now and
    // come back only if the server refuses.
    const liftFromLists = useCallback((ids: Set<string>): MoveSnapshot => {
        const snapshot: MoveSnapshot = {
            messages: takeOut(messages, ids).lifted,
            searchResults: takeOut(searchResults, ids).lifted,
            tabs: takeOut(tabs, ids).lifted,
            selected: selectedMessage && ids.has(selectedMessage.id) ? selectedMessage : null,
        }
        removeIdsFromLists(ids)
        return snapshot
    }, [messages, searchResults, tabs, selectedMessage, removeIdsFromLists])

    // restoreToLists puts back the lifted rows whose ids keep says the server did not move, each at the
    // index it held. A row the server did move stays gone.
    const restoreToLists = useCallback((snapshot: MoveSnapshot, keep: (id: string) => boolean) => {
        const kept = (lifted: Lifted[]) => lifted.filter((entry) => keep(entry.message.id))
        const forMessages = kept(snapshot.messages)
        const forSearch = kept(snapshot.searchResults)
        const forTabs = kept(snapshot.tabs)
        if (forMessages.length > 0) {
            setMessages((prev) => putBack(prev, forMessages))
        }
        if (forSearch.length > 0) {
            setSearchResults((prev) => putBack(prev, forSearch))
        }
        if (forTabs.length > 0) {
            setTabs((prev) => putBack(prev, forTabs))
        }
        const selected = snapshot.selected
        if (selected && keep(selected.id)) {
            setSelectedMessage((prev) => prev ?? selected)
        }
    }, [])

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
        // The rows leave the lists now, not when the server answers; the ids are marked in flight so a
        // repeat drop of the same message while the first is still open is not issued twice.
        for (const id of ids) {
            inFlightIds.current.add(id)
        }
        const snapshot = liftFromLists(new Set(ids))
        try {
            const result = await api.moveMessages(ids, destFolderId)
            const moved = new Set(result.ids)
            restoreToLists(snapshot, (id) => !moved.has(id))
            recordBulkMove('move', result.ids, result.newIds, sources, destFolderId)
            if (result.error) {
                setError(result.offline
                    ? result.error
                    : `${result.failed} of ${ids.length} messages could not be moved: ${result.error}`)
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
            // Nothing moved, so every lifted row goes back where it was rather than vanishing.
            restoreToLists(snapshot, () => true)
            setError(`Move failed: ${String(e)}`)
        } finally {
            for (const id of ids) {
                inFlightIds.current.delete(id)
            }
        }
        await loadUnread()
        await refreshFolders()
    }, [liftFromLists, restoreToLists, sourceFolderOf, recordBulkMove, loadUnread, refreshFolders])

    // dropMessageOnFolder is the drag-and-drop target handler. Dropping a row that is part of the
    // multi-selection moves the whole selection; dropping any other row moves just that one. Messages
    // already in the target folder, synthetic outbox items and rows belonging to a different account
    // than the target folder (a unified-list row cannot move across accounts) are skipped. The move is
    // batched, so a large drop stays under Gmail's connection cap. It reports whether anything is moving,
    // so a drop it skipped entirely is not confirmed on screen as one that landed.
    const dropMessageOnFolder = useCallback((messageId: string, folderId: string): boolean => {
        if (folderId === OUTBOX_FOLDER_ID) {
            return false
        }
        const targetAccount = folders.find((f) => f.id === folderId)?.accountId ?? ''
        const ids = markedIds.has(messageId) && markedIds.size > 1 ? [...markedIds] : [messageId]
        const movable = ids.filter((id) => {
            if (inFlightIds.current.has(id)) {
                return false
            }
            const source = messages.find((m) => m.id === id) ?? searchResults.find((m) => m.id === id)
            return source !== undefined && source.folderId !== folderId && !isOutboxMessage(source)
                && (!source.accountId || source.accountId === targetAccount)
        })
        setMarkedIds(new Set())
        setAnchorId(null)
        void bulkMoveIds(movable, folderId)
        return movable.length > 0
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
                setError(result.offline
                    ? result.error
                    : `${result.failed} of ${targets.length} messages could not be deleted: ${result.error}`)
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

    const deleteConfirmation = bulkToDelete === null ? null : deleteManyConfirmation(
        bulkToDelete.length,
        isPop3,
        {
            busy: bulkDeleting,
            onConfirm: () => void runBulkDelete(bulkToDelete, false),
            onCancel: () => setBulkToDelete(null),
        },
    )
    const purgeConfirmation = bulkToPurge === null ? null : purgeManyConfirmation(
        bulkToPurge.length,
        {
            busy: bulkPurging,
            onConfirm: () => void runBulkDelete(bulkToPurge, true),
            onCancel: () => setBulkToPurge(null),
        },
    )

    return {
        deleteConfirmation, purgeConfirmation,
        bulkToDelete, setBulkToDelete, bulkDeleting,
        bulkToPurge, setBulkToPurge, bulkPurging,
        runBulkDelete, bulkSetRead, bulkSetFlag, bulkMove, dropMessageOnFolder,
    }
}
