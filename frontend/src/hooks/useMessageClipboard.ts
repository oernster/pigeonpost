import {useCallback, useState} from 'react'
import {Message, api} from '../api'

// MessageClipboardDeps: bulkMoveIds is useBulkActions' batched move, which already records the
// undo entry, syncs the destination and refreshes the badges, exactly what pasting a cut needs.
export interface MessageClipboardDeps {
    bulkMoveIds: (ids: string[], destFolderId: string) => Promise<void>
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
// therefore always safe: an unpasted cut simply expires when something else is cut or copied. A
// pasted cut is consumed (its ids moved, so they no longer address anything); a pasted copy stays
// on the clipboard because the originals are untouched and can be pasted again elsewhere.
export function useMessageClipboard(deps: MessageClipboardDeps): MessageClipboard {
    const {bulkMoveIds, setError} = deps
    const [clip, setClip] = useState<{mode: 'move' | 'copy'; ids: string[]} | null>(null)

    const take = useCallback((mode: 'move' | 'copy', messages: Message[]) => {
        const ids = messages.map((m) => m.id)
        if (ids.length === 0) {
            return
        }
        setClip({mode, ids})
    }, [])

    const cutMessages = useCallback((messages: Message[]) => take('move', messages), [take])
    const copyMessages = useCallback((messages: Message[]) => take('copy', messages), [take])

    const pasteInto = useCallback(async (destFolderId: string) => {
        if (!clip || destFolderId === '') {
            return
        }
        if (clip.mode === 'move') {
            await bulkMoveIds(clip.ids, destFolderId)
            setClip(null)
            return
        }
        setError('')
        let failed = 0
        for (const id of clip.ids) {
            try {
                await api.copyMessage(id, destFolderId)
            } catch {
                failed += 1
            }
        }
        try {
            // Pull the destination at once so the pasted copies appear immediately; a failure
            // leaves the next background sync to reconcile.
            await api.syncFolder(destFolderId)
        } catch {
            // Reconciled by the next background sync.
        }
        if (failed > 0) {
            setError(`${failed} of ${clip.ids.length} messages could not be pasted.`)
        }
    }, [clip, bulkMoveIds, setError])

    return {hasClip: clip !== null, cutMessages, copyMessages, pasteInto}
}
