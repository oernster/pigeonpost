import {Dispatch, MutableRefObject, SetStateAction, useCallback, useEffect, useState} from 'react'
import {Folder, api} from '../api'
import {OUTBOX_FOLDER_ID} from '../outbox'
import type {MessageStore} from './useMessageStore'

// autoSyncIntervalMs is how often the folder on screen is refreshed from the server in the background,
// so new mail in the open folder appears without a manual sync.
const millisPerMinute = 60 * 1000
const autoSyncIntervalMs = 5 * millisPerMinute

// SyncDeps is what syncing needs from the rest of App: the selected account (whose mailbox is synced), the
// selected folder and its ref (the folder a sync or the background poll reloads), the folder-list setter, the
// message store, the outbox refresher, the unread-count refresher and the error sink.
export interface SyncDeps {
    selectedAccount: string
    selectedFolder: string
    selectedFolderRef: MutableRefObject<string>
    setFolders: Dispatch<SetStateAction<Folder[]>>
    store: MessageStore
    refreshOutbox: () => Promise<void>
    loadUnread: () => Promise<void>
    setError: (message: string) => void
}

export interface Sync {
    syncingAccounts: Set<string>
    sync: () => Promise<void>
    // accountSyncing is true while the selected account's mailbox sync is running, so the Sync control
    // disables and relabels for that account only.
    accountSyncing: boolean
}

// useSync owns the mailbox sync (a manual full-account sync, and the periodic light refresh of the folder on
// screen) and the per-account "is syncing" state. A full sync flushes the outbox, refreshes the folder list
// and the open folder and updates the unread counts; the background poll re-syncs just the open folder.
export function useSync(deps: SyncDeps): Sync {
    const {
        selectedAccount, selectedFolder, selectedFolderRef, setFolders, store,
        refreshOutbox, loadUnread, setError,
    } = deps
    const {setMessages} = store

    const [syncingAccounts, setSyncingAccounts] = useState<Set<string>>(() => new Set<string>())

    const sync = useCallback(async () => {
        if (!selectedAccount) {
            return
        }
        const accountId = selectedAccount
        setSyncingAccounts((prev) => new Set(prev).add(accountId))
        setError('')
        try {
            await api.syncAccount(accountId)
            // Connectivity is back: flush anything queued while offline, then refresh views.
            await api.replayOutbox()
            setFolders(await api.listFolders(accountId))
            if (selectedFolder) {
                setMessages(await api.listMessages(selectedFolder))
            }
            await refreshOutbox()
            await loadUnread()
        } catch (e) {
            setError(String(e))
        } finally {
            setSyncingAccounts((prev) => {
                const next = new Set(prev)
                next.delete(accountId)
                return next
            })
        }
    }, [selectedAccount, selectedFolder, refreshOutbox, loadUnread])

    // accountSyncing is true while the selected account's mailbox sync is running, so the Sync control
    // disables and relabels for that account only; other accounts stay syncable one by one.
    const accountSyncing = selectedAccount !== '' && syncingAccounts.has(selectedAccount)

    // Periodic light refresh of the folder on screen: syncs only that folder (not the whole account)
    // and reloads it, so new mail in the open folder appears without a manual sync.
    useEffect(() => {
        // The Outbox is synthetic, so there is no server folder to poll.
        if (!selectedFolder || selectedFolder === OUTBOX_FOLDER_ID) {
            return
        }
        const interval = window.setInterval(() => {
            void (async () => {
                try {
                    await api.syncFolder(selectedFolder)
                    // Only replace the list if the user is still on this folder.
                    if (selectedFolderRef.current === selectedFolder) {
                        setMessages(await api.listMessages(selectedFolder))
                    }
                    await loadUnread()
                } catch {
                    // A background refresh failure (offline) must not disrupt the UI.
                }
            })()
        }, autoSyncIntervalMs)
        return () => window.clearInterval(interval)
    }, [selectedFolder, loadUnread])

    return {syncingAccounts, sync, accountSyncing}
}
