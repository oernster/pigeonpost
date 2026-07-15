import {Dispatch, SetStateAction, useCallback, useState} from 'react'
import {api, Folder, Message} from '../api'
import {neighbourAfterRemoval} from '../messageText'
import type {MoveFlavour} from '../undoStack'
import type {MessageStore} from './useMessageStore'
import type {UndoRecorder} from './useUndoRedo'

// MessageActionsDeps is what the single-message actions need from the rest of App: the message store they
// mutate, the visible list and the search flag (used to pick the next selection after a removal), the
// two badge refreshers and the error sink. loadUnread refreshes the per-account and titlebar unread
// badges; refreshFolders reloads the folder tree, whose rows carry the per-folder unread badge. Every
// action that can change a folder's unread count must refresh both, or the folder badge goes stale.
export interface MessageActionsDeps {
    store: MessageStore
    displayMessages: Message[]
    searchActive: boolean
    // folders lets an action resolve a message's account-mate folder (its Junk folder or its
    // Inbox) so the destination of a junk or rescue can be synced at once.
    folders: Folder[]
    loadUnread: () => Promise<void>
    refreshFolders: () => Promise<void>
    setError: (message: string) => void
    // undo records each completed action so Edit > Undo can unwind it.
    undo: UndoRecorder
}

export interface MessageActions {
    messageToDelete: Message | null
    setMessageToDelete: Dispatch<SetStateAction<Message | null>>
    deletingMessage: boolean
    messageToPurge: Message | null
    setMessageToPurge: Dispatch<SetStateAction<Message | null>>
    purgingMessage: boolean
    requestDelete: (message: Message) => void
    deleteMessage: () => Promise<void>
    deletePermanent: () => Promise<void>
    toggleFlag: (message: Message) => Promise<void>
    moveMessage: (message: Message, destFolderId: string) => Promise<void>
    markJunk: (message: Message) => Promise<void>
    markNotJunk: (message: Message) => Promise<void>
    copyMessage: (message: Message, destFolderId: string) => Promise<void>
    setReadState: (message: Message, read: boolean, record?: boolean) => Promise<void>
    toggleRead: (message: Message) => Promise<void>
    markReadOnView: (message: Message) => void
    markReplied: (id: string) => Promise<void>
    markForwarded: (id: string) => Promise<void>
}

// useMessageActions owns the single-message actions (delete, move, flag, read, junk, copy) and the confirm
// state for the single delete and the permanent delete. Every list change goes through the message store, so
// it shows wherever the message appears. Bulk actions, tag actions and the outbox cancel live in their own
// hooks.
export function useMessageActions(deps: MessageActionsDeps): MessageActions {
    const {store, displayMessages, searchActive, folders, loadUnread, refreshFolders, setError, undo} = deps
    const {
        searchResults, setMessages, setSearchResults, setTabs, setSelectedMessage,
        applyToAllLists, removeFromAllLists,
    } = store

    const [messageToDelete, setMessageToDelete] = useState<Message | null>(null)
    const [deletingMessage, setDeletingMessage] = useState<boolean>(false)
    const [messageToPurge, setMessageToPurge] = useState<Message | null>(null)
    const [purgingMessage, setPurgingMessage] = useState<boolean>(false)

    // refreshBadges refreshes every surface that shows an unread count (the account and titlebar badges
    // and the folder tree's per-folder badges) after an action that can change one. It is one shared
    // refresher precisely so no action can update one badge and forget the other, which is how a read
    // message used to leave its folder's badge stale until relaunch.
    const refreshBadges = useCallback(async () => {
        await Promise.all([loadUnread(), refreshFolders()])
    }, [loadUnread, refreshFolders])

    // syncDestination pulls a destination folder's listing straight away, so a moved, junked or
    // rescued message appears there (and counts toward its unread badge) immediately rather than on
    // the next background sync. Best-effort: a sync failure leaves the next background pass to
    // reconcile and must not fail the action that already succeeded on the server.
    const syncDestination = useCallback(async (destFolderId: string) => {
        try {
            await api.syncFolder(destFolderId)
        } catch {
            // The next background sync reconciles the destination.
        }
    }, [])

    // destinationByKind resolves the account-mate folder of the given kind for a message (its Junk
    // folder for a junking, its Inbox for a rescue), so the action can sync where the message went.
    const destinationByKind = useCallback((message: Message, kind: string): string => {
        const accountId = folders.find((f) => f.id === message.folderId)?.accountId
        return folders.find((f) => f.accountId === accountId && f.kind === kind)?.id ?? ''
    }, [folders])

    // recordMove records one completed move-shaped action for Edit > Undo, addressing the message
    // by the id the server said it now carries. An action the server did not locate (no COPYUID)
    // is simply not recorded, so Undo never offers a reversal it cannot perform.
    const recordMove = useCallback((flavour: MoveFlavour, newId: string, sourceFolderId: string, destFolderId: string) => {
        if (newId === '') {
            return
        }
        undo.push({kind: 'move', flavour, items: [{messageId: newId, sourceFolderId}], destFolderId})
    }, [undo])

    const deleteMessage = useCallback(async () => {
        if (!messageToDelete) {
            return
        }
        const id = messageToDelete.id
        const list = searchActive ? searchResults : displayMessages
        const next = neighbourAfterRemoval(list, id)
        setDeletingMessage(true)
        setError('')
        try {
            const result = await api.deleteMessage(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setTabs((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? next : prev))
            setMessageToDelete(null)
            recordMove('delete', result.newId, messageToDelete.folderId, '')
            await refreshBadges()
        } catch (e) {
            setError(String(e))
        } finally {
            setDeletingMessage(false)
        }
    }, [messageToDelete, searchActive, searchResults, displayMessages, recordMove, refreshBadges])

    // deletePermanent is the confirmed, irreversible delete behind Shift+Delete: it removes the message
    // from the server without moving it to Trash, then advances the selection.
    const deletePermanent = useCallback(async () => {
        if (!messageToPurge) {
            return
        }
        const id = messageToPurge.id
        const list = searchActive ? searchResults : displayMessages
        const next = neighbourAfterRemoval(list, id)
        setPurgingMessage(true)
        setError('')
        try {
            await api.deleteMessagePermanent(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setTabs((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? next : prev))
            setMessageToPurge(null)
            await refreshBadges()
        } catch (e) {
            setError(String(e))
        } finally {
            setPurgingMessage(false)
        }
    }, [messageToPurge, searchActive, searchResults, displayMessages, refreshBadges])

    // requestDelete always asks for confirmation before deleting. The confirmed delete moves the message to
    // Trash where the account has one, or removes it permanently otherwise (the dialog says which). Shared
    // by the Delete key and the context menu.
    const requestDelete = useCallback((message: Message) => {
        setMessageToDelete(message)
    }, [])

    const toggleFlag = useCallback(async (message: Message) => {
        const next = !message.flagged
        setError('')
        try {
            await api.markFlagged(message.id, next)
            const apply = (m: Message): Message => (m.id === message.id ? {...m, flagged: next} : m)
            setMessages((prev) => prev.map(apply))
            setSearchResults((prev) => prev.map(apply))
            setSelectedMessage((prev) => (prev && prev.id === message.id ? {...prev, flagged: next} : prev))
            undo.push({kind: 'flag', items: [{messageId: message.id, before: message.flagged}], after: next})
        } catch (e) {
            setError(String(e))
        }
    }, [undo])

    // markReplied records a sent reply on its original message: the \Answered flag on the server (and the
    // local cache) through the backend, then the answered field in place across every list so the replied
    // glyph appears on the row at once, without waiting for the folder to be reopened. It is best-effort:
    // a failure (an offline reply whose STORE cannot reach the server) leaves the message unflagged until it
    // is acted on again, so the in-memory glyph is only set once the server has actually recorded it. Called
    // fire-and-forget from the composer after a successful send, so it never blocks or fails the send.
    const markReplied = useCallback(async (id: string) => {
        try {
            await api.markReplied(id)
            applyToAllLists((m) => (m.id === id ? {...m, answered: true} : m))
        } catch {
            // Best-effort: leave the message unflagged on failure.
        }
    }, [applyToAllLists])

    // markForwarded is markReplied's counterpart for a forward: it sets the $Forwarded keyword through the
    // backend, then the forwarded field in place across every list. Same best-effort, fire-and-forget contract.
    const markForwarded = useCallback(async (id: string) => {
        try {
            await api.markForwarded(id)
            applyToAllLists((m) => (m.id === id ? {...m, forwarded: true} : m))
        } catch {
            // Best-effort: leave the message unflagged on failure.
        }
    }, [applyToAllLists])

    // moveMessage relocates a message to a folder; a same-folder drop is a no-op on the server.
    // Moving an unread message changes both folders' unread counts, so the badges are refreshed.
    const moveMessage = useCallback(async (message: Message, destFolderId: string) => {
        setError('')
        try {
            const result = await api.moveMessage(message.id, destFolderId)
            removeFromAllLists(new Set([message.id]))
            recordMove('move', result.newId, message.folderId, destFolderId)
            await syncDestination(destFolderId)
            await refreshBadges()
        } catch (e) {
            setError(String(e))
        }
    }, [removeFromAllLists, recordMove, syncDestination, refreshBadges])

    // markJunk files a message into the account's Junk folder and removes it from the current view,
    // advancing the selection and refreshing the unread counts as a move out of the inbox does.
    const markJunk = useCallback(async (message: Message) => {
        const id = message.id
        const list = searchActive ? searchResults : displayMessages
        const next = neighbourAfterRemoval(list, id)
        setError('')
        try {
            const result = await api.markJunk(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setTabs((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? next : prev))
            const dest = destinationByKind(message, 'junk')
            recordMove('junk', result.newId, message.folderId, dest)
            if (dest !== '') {
                await syncDestination(dest)
            }
            await refreshBadges()
        } catch (e) {
            setError(String(e))
        }
    }, [searchActive, searchResults, displayMessages, destinationByKind, recordMove, syncDestination, refreshBadges])

    // markNotJunk rescues a wrongly junked message back to the account's inbox: the row leaves the
    // Junk view at once and the inbox re-lists it on the next sync, the same shape as markJunk in
    // the other direction.
    const markNotJunk = useCallback(async (message: Message) => {
        const id = message.id
        const list = searchActive ? searchResults : displayMessages
        const next = neighbourAfterRemoval(list, id)
        setError('')
        try {
            const result = await api.markNotJunk(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setTabs((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? next : prev))
            const dest = destinationByKind(message, 'inbox')
            recordMove('notJunk', result.newId, message.folderId, dest)
            if (dest !== '') {
                await syncDestination(dest)
            }
            await refreshBadges()
        } catch (e) {
            setError(String(e))
        }
    }, [searchActive, searchResults, displayMessages, destinationByKind, recordMove, syncDestination, refreshBadges])

    // Copy leaves the original in place; the duplicate appears in the destination folder on next sync, so
    // there is no local list change to make here.
    const copyMessage = useCallback(async (message: Message, destFolderId: string) => {
        setError('')
        try {
            await api.copyMessage(message.id, destFolderId)
        } catch (e) {
            setError(String(e))
        }
    }, [])

    // setReadState sets a message's read flag on the server and optimistically in the on-screen lists, so
    // it bolds or un-bolds at once, then refreshes the account and folder badges. Used by the Mark
    // submenu (explicit read/unread) and on view. Only a deliberate change is recorded for undo:
    // record is false for the automatic mark-read-on-view, which would otherwise bury the entry the
    // user actually wants to unwind under one per opened message.
    const setReadState = useCallback(async (message: Message, read: boolean, record = true) => {
        applyToAllLists((m) => (m.id === message.id ? {...m, read} : m))
        try {
            await api.markRead(message.id, read)
            if (record && message.read !== read) {
                undo.push({kind: 'read', items: [{messageId: message.id, before: message.read}], after: read})
            }
            await refreshBadges()
        } catch (e) {
            setError(String(e))
        }
    }, [applyToAllLists, undo, refreshBadges])

    // toggleRead flips a message's read flag by delegating to setReadState, which updates the flag in place
    // across every list (the open reader included) and refreshes the unread counts. It does not reload the
    // folder, so toggling read in a folder of tens of thousands of messages never refetches every row.
    const toggleRead = useCallback(async (message: Message) => {
        await setReadState(message, !message.read)
    }, [setReadState])

    // markReadOnView marks a message read when it is displayed, unless it already is. Automatic,
    // so it is not recorded for undo.
    const markReadOnView = useCallback((message: Message) => {
        if (!message.read) {
            void setReadState(message, true, false)
        }
    }, [setReadState])

    return {
        messageToDelete, setMessageToDelete, deletingMessage,
        messageToPurge, setMessageToPurge, purgingMessage,
        requestDelete, deleteMessage, deletePermanent, toggleFlag, moveMessage, markJunk, markNotJunk, copyMessage,
        setReadState, toggleRead, markReadOnView, markReplied, markForwarded,
    }
}
