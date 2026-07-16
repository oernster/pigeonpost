import {useCallback, useState} from 'react'
import {Message, api} from '../api'
import type {MoveItem} from '../undoStack'
import type {MessageStore} from './useMessageStore'
import type {UndoRecorder} from './useUndoRedo'

// MessageClipboardDeps is what pasting needs from the rest of App: the message store (a pasted cut
// shows its rows immediately), the id of the folder being viewed (the optimistic rows belong in
// the list only when the paste targets that folder; a paste onto another folder via its context
// menu must not show rows here), the undo recorder the completed move reports to, the two badge
// refreshers and the error sink.
export interface MessageClipboardDeps {
    store: MessageStore
    selectedFolderId: string
    undo: UndoRecorder
    loadUnread: () => Promise<void>
    refreshFolders: () => Promise<void>
    setError: (message: string) => void
}

export interface MessageClipboard {
    // hasClip gates Edit > Paste at the message level.
    hasClip: boolean
    cutMessages: (messages: Message[]) => void
    copyMessages: (messages: Message[]) => void
    pasteInto: (destFolderId: string) => Promise<void>
}

// useMessageClipboard is the message-level half of Edit > Cut / Copy / Paste, file-manager style:
// Cut or Copy puts the selected messages on an internal clipboard and nothing touches the server
// until Paste files them into the folder being viewed (a cut moves, a copy duplicates). Cut is
// therefore always safe: an unpasted cut simply expires when something else is cut or copied.
//
// Pasting a cut is optimistic, so it feels instant: the rows appear in the open folder at once,
// the server move runs behind them, and each row is then re-pointed at the id the server says it
// now carries (which is also what the recorded undo entry addresses). A move the server refuses
// rolls its row back out and reports through the error sink; if the whole call fails the clipboard
// is restored so the paste can be retried. A pasted copy stays on the clipboard because the
// originals are untouched and can be pasted again elsewhere; its duplicates appear on the
// destination sync (their server identities are brand new, so there is nothing to show early).
export function useMessageClipboard(deps: MessageClipboardDeps): MessageClipboard {
    const {store, selectedFolderId, undo, loadUnread, refreshFolders, setError} = deps
    const {messages, setMessages, applyToAllLists, removeFromAllLists} = store
    const [clip, setClip] = useState<{mode: 'move' | 'copy'; messages: Message[]} | null>(null)

    const take = useCallback((mode: 'move' | 'copy', messages: Message[]) => {
        if (messages.length === 0) {
            return
        }
        setClip({mode, messages})
    }, [])

    const cutMessages = useCallback((messages: Message[]) => take('move', messages), [take])
    const copyMessages = useCallback((messages: Message[]) => take('copy', messages), [take])

    // syncDestination pulls the destination's listing so the server's view of the paste lands (and
    // reconciles the optimistic rows); best-effort, the next background sync covers a failure.
    const syncDestination = useCallback(async (destFolderId: string) => {
        try {
            await api.syncFolder(destFolderId)
        } catch {
            // Reconciled by the next background sync.
        }
    }, [])

    // pasteMove is the optimistic move behind pasting a cut. The clipboard rows not already in the
    // open folder are inserted immediately (re-homed to the destination), then the batched server
    // move runs and the outcome is reconciled: moved rows are re-pointed at their new server ids,
    // refused rows are rolled back out and one undo entry records the whole paste.
    const pasteMove = useCallback(async (taken: Message[], destFolderId: string) => {
        // The optimistic rows belong in the on-screen list only when the paste targets the folder
        // being viewed. Which rows to insert is decided against the list as it stands (a paste into
        // the folder a message already sits in must not duplicate its row); the updater re-checks
        // so a racing list change still cannot double-insert.
        const have = new Set(messages.map((m) => m.id))
        const add = destFolderId === selectedFolderId
            ? taken.filter((m) => !have.has(m.id)).map((m) => ({...m, folderId: destFolderId}))
            : []
        const inserted = add.map((r) => r.id)
        if (add.length > 0) {
            setMessages((prev) => {
                const present = new Set(prev.map((m) => m.id))
                const fresh = add.filter((r) => !present.has(r.id))
                return fresh.length > 0 ? [...prev, ...fresh] : prev
            })
        }
        setError('')
        try {
            const result = await api.moveMessages(taken.map((m) => m.id), destFolderId)
            const moved = new Set(result.ids)
            const rolledBack = inserted.filter((id) => !moved.has(id))
            if (rolledBack.length > 0) {
                removeFromAllLists(new Set(rolledBack))
            }
            // Re-point each moved row at the id the server says it now carries, so later actions
            // (open, delete, another cut) address the real message rather than the stale id.
            const newIds = result.newIds ?? {}
            applyToAllLists((m) => (newIds[m.id] ? {...m, id: newIds[m.id]} : m))
            const sources = new Map(taken.map((m) => [m.id, m.folderId]))
            const items: MoveItem[] = result.ids
                .filter((id) => Boolean(newIds[id]) && sources.get(id) !== undefined)
                .map((id) => ({messageId: newIds[id], sourceFolderId: sources.get(id) as string}))
            if (items.length > 0) {
                undo.push({kind: 'move', flavour: 'move', items, destFolderId})
            }
            if (result.error) {
                setError(`${result.failed} of ${taken.length} messages could not be pasted: ${result.error}`)
            }
            await syncDestination(destFolderId)
            await Promise.all([loadUnread(), refreshFolders()])
        } catch (e) {
            // The whole move failed: roll the optimistic rows back out and put the clipboard back
            // so the paste can be retried.
            if (inserted.length > 0) {
                removeFromAllLists(new Set(inserted))
            }
            setClip({mode: 'move', messages: taken})
            setError(`Paste failed: ${String(e)}`)
        }
    }, [messages, selectedFolderId, setMessages, removeFromAllLists, applyToAllLists, undo, syncDestination, loadUnread, refreshFolders, setError])

    const pasteInto = useCallback(async (destFolderId: string) => {
        if (!clip || destFolderId === '') {
            return
        }
        if (clip.mode === 'move') {
            // Consume the cut before the server settles: its rows are already re-homed on screen.
            setClip(null)
            await pasteMove(clip.messages, destFolderId)
            return
        }
        setError('')
        let failed = 0
        for (const m of clip.messages) {
            try {
                await api.copyMessage(m.id, destFolderId)
            } catch {
                failed += 1
            }
        }
        await syncDestination(destFolderId)
        if (failed > 0) {
            setError(`${failed} of ${clip.messages.length} messages could not be pasted.`)
        }
    }, [clip, pasteMove, syncDestination, setError])

    return {hasClip: clip !== null, cutMessages, copyMessages, pasteInto}
}
