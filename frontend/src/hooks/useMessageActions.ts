import {Dispatch, SetStateAction, useCallback, useState} from 'react'
import {api, Message} from '../api'
import {neighbourAfterRemoval} from '../messageText'
import type {MessageStore} from './useMessageStore'

// MessageActionsDeps is what the single-message actions need from the rest of App: the message store they
// mutate, the visible list and the search flag (used to pick the next selection after a removal), the
// unread-count refresher and the error sink.
export interface MessageActionsDeps {
    store: MessageStore
    displayMessages: Message[]
    searchActive: boolean
    loadUnread: () => Promise<void>
    setError: (message: string) => void
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
    copyMessage: (message: Message, destFolderId: string) => Promise<void>
    setReadState: (message: Message, read: boolean) => Promise<void>
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
    const {store, displayMessages, searchActive, loadUnread, setError} = deps
    const {
        searchResults, setMessages, setSearchResults, setTabs, setSelectedMessage,
        applyToAllLists, removeFromAllLists,
    } = store

    const [messageToDelete, setMessageToDelete] = useState<Message | null>(null)
    const [deletingMessage, setDeletingMessage] = useState<boolean>(false)
    const [messageToPurge, setMessageToPurge] = useState<Message | null>(null)
    const [purgingMessage, setPurgingMessage] = useState<boolean>(false)

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
            await api.deleteMessage(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setTabs((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? next : prev))
            setMessageToDelete(null)
            await loadUnread()
        } catch (e) {
            setError(String(e))
        } finally {
            setDeletingMessage(false)
        }
    }, [messageToDelete, searchActive, searchResults, displayMessages, loadUnread])

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
            await loadUnread()
        } catch (e) {
            setError(String(e))
        } finally {
            setPurgingMessage(false)
        }
    }, [messageToPurge, searchActive, searchResults, displayMessages, loadUnread])

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
        } catch (e) {
            setError(String(e))
        }
    }, [])

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

    // moveMessageById moves a message to a folder by id. It backs moveMessage; a same-folder drop is a
    // no-op on the server.
    const moveMessageById = useCallback(async (messageId: string, destFolderId: string) => {
        setError('')
        try {
            await api.moveMessage(messageId, destFolderId)
            removeFromAllLists(new Set([messageId]))
        } catch (e) {
            setError(String(e))
        }
    }, [removeFromAllLists])

    const moveMessage = useCallback(
        (message: Message, destFolderId: string) => moveMessageById(message.id, destFolderId),
        [moveMessageById],
    )

    // markJunk files a message into the account's Junk folder and removes it from the current view,
    // advancing the selection and refreshing the unread counts as a move out of the inbox does.
    const markJunk = useCallback(async (message: Message) => {
        const id = message.id
        const list = searchActive ? searchResults : displayMessages
        const next = neighbourAfterRemoval(list, id)
        setError('')
        try {
            await api.markJunk(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setTabs((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? next : prev))
            await loadUnread()
        } catch (e) {
            setError(String(e))
        }
    }, [searchActive, searchResults, displayMessages, loadUnread])

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
    // it bolds or un-bolds at once. Used by the Mark submenu (explicit read/unread) and on view.
    const setReadState = useCallback(async (message: Message, read: boolean) => {
        applyToAllLists((m) => (m.id === message.id ? {...m, read} : m))
        try {
            await api.markRead(message.id, read)
            await loadUnread()
        } catch (e) {
            setError(String(e))
        }
    }, [applyToAllLists, loadUnread])

    // toggleRead flips a message's read flag by delegating to setReadState, which updates the flag in place
    // across every list (the open reader included) and refreshes the unread counts. It does not reload the
    // folder, so toggling read in a folder of tens of thousands of messages never refetches every row.
    const toggleRead = useCallback(async (message: Message) => {
        await setReadState(message, !message.read)
    }, [setReadState])

    // markReadOnView marks a message read when it is displayed, unless it already is.
    const markReadOnView = useCallback((message: Message) => {
        if (!message.read) {
            void setReadState(message, true)
        }
    }, [setReadState])

    return {
        messageToDelete, setMessageToDelete, deletingMessage,
        messageToPurge, setMessageToPurge, purgingMessage,
        requestDelete, deleteMessage, deletePermanent, toggleFlag, moveMessage, markJunk, copyMessage,
        setReadState, toggleRead, markReadOnView, markReplied, markForwarded,
    }
}
