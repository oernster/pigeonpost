import {Dispatch, SetStateAction, useCallback, useEffect, useMemo, useState} from 'react'
import {Folder, Message, OutboxItem, api} from '../api'
import {OUTBOX_FOLDER_ID} from '../outbox'

// OutboxDeps is what the outbox needs from the rest of App: the selected account (whose queue is shown),
// the account's real folders (to append the synthetic Outbox folder to) and the error sink.
export interface OutboxDeps {
    selectedAccount: string
    folders: Folder[]
    setError: (message: string) => void
}

export interface Outbox {
    // outbox is the whole queue across accounts; outboxForAccount is just the selected account's items,
    // shown under its Outbox folder.
    outbox: OutboxItem[]
    outboxForAccount: OutboxItem[]
    refreshOutbox: () => Promise<void>
    // sidebarFolders is the account's real folders plus the synthetic Outbox folder while it has queued mail.
    sidebarFolders: Folder[]
    messageToCancelSend: Message | null
    setMessageToCancelSend: Dispatch<SetStateAction<Message | null>>
    cancellingSend: boolean
    cancelSend: () => Promise<void>
}

// useOutbox owns the queue of outgoing operations waiting to be sent, surfaced as a per-account synthetic
// Outbox folder, and the cancel-send confirm flow. The queue is loaded on mount and reloaded after a sync,
// a send or a cancel. The effect that keeps the open Outbox VIEW in step with the queue stays in App,
// because it drives folder navigation (falling back to the inbox once the queue empties).
export function useOutbox(deps: OutboxDeps): Outbox {
    const {selectedAccount, folders, setError} = deps

    const [outbox, setOutbox] = useState<OutboxItem[]>([])
    const [messageToCancelSend, setMessageToCancelSend] = useState<Message | null>(null)
    const [cancellingSend, setCancellingSend] = useState<boolean>(false)
    // outboxForAccount is the queued items belonging to the selected account, shown under its Outbox
    // folder. Memoised so the derived message rows and the folder's presence are stable per render.
    const outboxForAccount = useMemo(
        () => outbox.filter((item) => item.accountId === selectedAccount),
        [outbox, selectedAccount],
    )

    // refreshOutbox reloads the queue of outgoing operations waiting to be sent. The queue is surfaced
    // as a per-account Outbox folder, so the full item list is kept, not just a count.
    const refreshOutbox = useCallback(async () => {
        try {
            setOutbox(await api.listOutbox())
        } catch {
            // A queue read failing must not disrupt the UI; leave the last known value.
        }
    }, [])

    useEffect(() => {
        void refreshOutbox()
    }, [refreshOutbox])

    // sidebarFolders is the account's real folders plus a synthetic Outbox folder, shown only while the
    // account has queued mail. The count rides on the unread field so it appears as the folder's badge.
    const sidebarFolders = useMemo<Folder[]>(() => {
        if (outboxForAccount.length === 0) {
            return folders
        }
        const outboxFolder: Folder = {
            id: OUTBOX_FOLDER_ID,
            accountId: selectedAccount,
            path: 'Outbox',
            name: 'Outbox',
            kind: 'outbox',
            unread: outboxForAccount.length,
            total: outboxForAccount.length,
        }
        return [...folders, outboxFolder]
    }, [folders, outboxForAccount, selectedAccount])

    // cancelSend discards the queued outbox item behind the confirmation dialog.
    const cancelSend = useCallback(async () => {
        if (!messageToCancelSend) {
            return
        }
        setCancellingSend(true)
        setError('')
        try {
            await api.cancelOutboxItem(messageToCancelSend.id)
            setMessageToCancelSend(null)
            await refreshOutbox()
        } catch (e) {
            setError(String(e))
        } finally {
            setCancellingSend(false)
        }
    }, [messageToCancelSend, refreshOutbox])

    return {
        outbox, outboxForAccount, refreshOutbox, sidebarFolders,
        messageToCancelSend, setMessageToCancelSend, cancellingSend, cancelSend,
    }
}
