import {Dispatch, SetStateAction, useEffect} from 'react'
import {AboutInfo, Account, Folder, Message} from '../api'
import {stepFocusRing, trapTab} from '../focusRing'
import type {FolderPrompt} from './useFolders'

// MessageListKeyboardDeps is what the window keyboard handler reads: the current view (the list the arrows and
// Ctrl+A act on) and its selection, every overlay state (keyboard handling for the list is suppressed while any
// is open), and the handlers a key fires (open, delete, the folder delete). The setters and openInNewTab are
// stable, so they are used but deliberately kept out of the effect's dependency array.
export interface MessageListKeyboardDeps {
    searchActive: boolean
    searchResults: Message[]
    displayMessages: Message[]
    selectedMessage: Message | null
    setSelectedMessage: Dispatch<SetStateAction<Message | null>>
    markedIds: Set<string>
    setMarkedIds: Dispatch<SetStateAction<Set<string>>>
    anchorId: string | null
    setAnchorId: Dispatch<SetStateAction<string | null>>
    setReadingFull: Dispatch<SetStateAction<boolean>>
    splashVisible: boolean
    composing: boolean
    settingUp: boolean
    accountToEdit: Account | null
    managingRules: boolean
    managingContacts: boolean
    managingCalendar: boolean
    about: AboutInfo | null
    licence: string | null
    folderPrompt: FolderPrompt | null
    messageToCancelSend: Message | null
    messageToDelete: Message | null
    accountToDelete: Account | null
    folderToDelete: Folder | null
    messageToPurge: Message | null
    contextMenu: {message: Message; x: number; y: number} | null
    bulkToDelete: Message[] | null
    bulkToPurge: Message[] | null
    folders: Folder[]
    requestDelete: (message: Message) => void
    openInNewTab: (message: Message, fromKeyboard?: boolean) => void
    setMessageToPurge: Dispatch<SetStateAction<Message | null>>
    setBulkToPurge: Dispatch<SetStateAction<Message[] | null>>
    setBulkToDelete: Dispatch<SetStateAction<Message[] | null>>
    setFolderToDelete: Dispatch<SetStateAction<Folder | null>>
    togglePreview: () => void
}

// useMessageListKeyboard installs the window keydown handler that drives the message list and the main-window
// focus ring: Tab and the arrows step the ring (mirroring each other), Up/Down move the message selection,
// Ctrl+A selects the view, Ctrl+Space toggles a row, Enter or Space opens the focused message, and Delete
// deletes the selection (or the focused custom folder). Handling is suppressed while a dialog is open or the
// user is typing, so it never competes with text entry or a modal.
export function useMessageListKeyboard(deps: MessageListKeyboardDeps): void {
    const {
        searchActive, searchResults, displayMessages, selectedMessage, setSelectedMessage,
        markedIds, setMarkedIds, anchorId, setAnchorId, setReadingFull,
        splashVisible, composing, settingUp, accountToEdit, managingRules, managingContacts, managingCalendar,
        about, licence, folderPrompt, messageToCancelSend, messageToDelete, accountToDelete, folderToDelete,
        messageToPurge, contextMenu, bulkToDelete, bulkToPurge, folders,
        requestDelete, openInNewTab, setMessageToPurge, setBulkToPurge, setBulkToDelete, setFolderToDelete,
        togglePreview,
    } = deps

    // Keyboard control for the message list: Arrow Up/Down move the selection, Delete asks to delete
    // the selected message (to Trash where possible), and Shift+Delete asks to delete it permanently.
    // Handling is suppressed while any dialog is open or while the user is typing in a field, so it
    // never competes with text entry or a modal.
    useEffect(() => {
        const overlayOpen =
            splashVisible || composing || settingUp || Boolean(accountToEdit) ||
            managingRules || managingContacts || managingCalendar || Boolean(about) || Boolean(licence) || Boolean(folderPrompt) ||
            Boolean(messageToCancelSend) ||
            Boolean(messageToDelete) || Boolean(accountToDelete) || Boolean(folderToDelete) ||
            Boolean(messageToPurge) || Boolean(contextMenu) || Boolean(bulkToDelete) || Boolean(bulkToPurge)
        const list = searchActive ? searchResults : displayMessages
        const onKeyDown = (e: KeyboardEvent) => {
            const target = e.target as HTMLElement | null
            const isText = Boolean(target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' ||
                target.tagName === 'SELECT' || target.isContentEditable))

            // A dialog traps focus: Tab/Shift+Tab and Left/Right cycle only within it, so focus can
            // neither leave the dialog nor reach the window behind it. Left/Right in a text field still
            // move the caret. Nothing else (list navigation, delete) acts while a dialog is open.
            if (document.querySelector('.modal') !== null) {
                if (e.key === 'Tab') {
                    trapTab(e)
                } else if ((e.key === 'ArrowRight' || e.key === 'ArrowLeft') && !isText) {
                    e.preventDefault()
                    stepFocusRing(e.key === 'ArrowRight' ? 1 : -1)
                }
                return
            }

            // Tab and Shift+Tab step the focus ring, exactly mirroring Right and Left, so the whole main
            // window is one ring with a single order. Driving Tab through the ring (rather than letting the
            // browser fall back to native Tab once focus is on a control) is what keeps Tab and Right
            // identical: native Tab would otherwise step into controls the ring deliberately skips (such as a
            // message row's star button) and then bounce focus back to the first stop. This runs before the
            // text-field guard so Tab also leaves a text field by the ring. The neutral start sink owns the
            // very first Tab on launch, stopping propagation before this runs; a context menu owns its own
            // keys, so the ring stays disabled while one is open.
            if (e.key === 'Tab') {
                if (contextMenu) {
                    return
                }
                e.preventDefault()
                stepFocusRing(e.shiftKey ? -1 : 1)
                return
            }
            // A dropdown drops open on the Down cursor and retracts on Up, so a focused select reveals its
            // list on Down rather than silently changing value (which for the reader's action selects would
            // fire a move or copy). Left and Right step the ring out of it like any stop; every other key
            // keeps the native select behaviour (type to jump to an option). The menu titles already open on
            // Down through their own handler.
            if (target && target.tagName === 'SELECT') {
                if (e.key === 'ArrowDown') {
                    e.preventDefault()
                    ;(target as HTMLSelectElement & {showPicker?: () => void}).showPicker?.()
                    return
                }
                if (e.key === 'ArrowUp') {
                    e.preventDefault()
                    return
                }
                if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                    e.preventDefault()
                    stepFocusRing(e.key === 'ArrowRight' ? 1 : -1)
                    return
                }
                return
            }
            // A text field keeps its own caret keys: the arrows move the caret, not the ring.
            if (isText) {
                return
            }
            // Right/Left step the focus ring, mirroring Tab/Shift+Tab across the main window. A context
            // menu owns its own keys, so the ring stays disabled while one is open; the splash does not
            // block it, so the very first Right on launch enters the ring.
            if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                if (contextMenu) {
                    return
                }
                e.preventDefault()
                stepFocusRing(e.key === 'ArrowRight' ? 1 : -1)
                return
            }
            if (overlayOpen) {
                return
            }
            if ((e.key === 'a' || e.key === 'A') && (e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey) {
                // Ctrl/Cmd+A selects every message in the current view (the open folder, or the search
                // results) so the whole lot can be deleted or moved at once. Delete then opens the
                // count-named bulk confirm. Suppressed inside text fields above, so it never steals the
                // native select-all while typing.
                if (list.length === 0) {
                    return
                }
                e.preventDefault()
                setMarkedIds(new Set(list.map((m) => m.id)))
                setAnchorId(list[0].id)
                return
            }
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                // The folder and account lists own their own Up/Down (they navigate folders and accounts);
                // do not also move the message selection when focus is within either of them.
                if (target && target.closest('[data-folder-list], [data-account-list]')) {
                    return
                }
                if (list.length === 0) {
                    return
                }
                e.preventDefault()
                const down = e.key === 'ArrowDown'
                const curIdx = selectedMessage ? list.findIndex((m) => m.id === selectedMessage.id) : -1
                const nextIdx = curIdx === -1
                    ? (down ? 0 : list.length - 1)
                    : (down ? Math.min(curIdx + 1, list.length - 1) : Math.max(curIdx - 1, 0))
                const next = list[nextIdx]
                if (!next) {
                    return
                }
                // Move DOM focus onto the row the cursor lands on unless another real control holds focus.
                // That covers being already on a message row and sitting on a neutral spot (the start sink
                // or the body, both tabindex -1), so focus and the cursor stay together and Enter or Space
                // can open the focused row. An arrow pressed while a header button holds focus still moves
                // the selection without stealing that focus.
                const active = document.activeElement as HTMLElement | null
                if (!active || active.tabIndex < 0 || active.classList.contains('message-row')) {
                    document.querySelectorAll<HTMLElement>('.message-list .message-row').forEach((row) => {
                        if (row.getAttribute('data-mid') === next.id) {
                            row.focus()
                        }
                    })
                }
                if (e.shiftKey) {
                    // Shift extends the contiguous selection from the anchor to the new cursor, the way a
                    // Shift click does, taking the current row as the anchor when there is not one yet.
                    const anchor = anchorId ?? (selectedMessage ? selectedMessage.id : next.id)
                    const aIdx = list.findIndex((m) => m.id === anchor)
                    if (aIdx === -1) {
                        setMarkedIds(new Set([next.id]))
                    } else {
                        const [lo, hi] = aIdx <= nextIdx ? [aIdx, nextIdx] : [nextIdx, aIdx]
                        setMarkedIds(new Set(list.slice(lo, hi + 1).map((m) => m.id)))
                    }
                    if (!anchorId) {
                        setAnchorId(anchor)
                    }
                    setSelectedMessage(next)
                    setReadingFull(false)
                    return
                }
                if (e.ctrlKey || e.metaKey) {
                    // Ctrl moves the focus cursor without changing the selection, so a non-contiguous set can
                    // be built with Ctrl+Space. Materialise the single selection first so moving the cursor
                    // off it does not silently drop it.
                    setMarkedIds((prev) => (prev.size ? prev : new Set<string>(selectedMessage ? [selectedMessage.id] : [])))
                    setSelectedMessage(next)
                    setAnchorId(next.id)
                    setReadingFull(false)
                    return
                }
                // Plain arrow is single-select: it drops any Ctrl/Shift selection and re-anchors here.
                setSelectedMessage(next)
                setMarkedIds(new Set())
                setAnchorId(next.id)
                return
            }
            if ((e.key === ' ' || e.code === 'Space') && (e.ctrlKey || e.metaKey)) {
                // Ctrl+Space toggles the focused row in or out of the selection, the keyboard equivalent of a
                // Ctrl click, so a non-contiguous set can be built with Ctrl+Arrow then Ctrl+Space.
                if (!selectedMessage) {
                    return
                }
                e.preventDefault()
                setMarkedIds((prev) => {
                    const base = prev.size ? new Set(prev) : new Set<string>([selectedMessage.id])
                    if (base.has(selectedMessage.id)) {
                        base.delete(selectedMessage.id)
                    } else {
                        base.add(selectedMessage.id)
                    }
                    return base
                })
                setAnchorId(selectedMessage.id)
                return
            }
            // Enter or plain Space opens the focused or selected message. Owned by the window (not just the
            // message row) so it works even when a click left focus on a child of the row rather than the row
            // itself. Scoped to the message list and skipped on a button or link, which handle their own
            // activation, so it never hijacks Enter elsewhere.
            if ((e.key === 'Enter' || e.key === ' ' || e.code === 'Space') && !e.ctrlKey && !e.metaKey && !e.shiftKey) {
                const row = target?.closest<HTMLElement>('.message-row')
                if (target && target.tagName !== 'BUTTON' && target.tagName !== 'A' && (row || target.closest('.message-list'))) {
                    const toOpen = row
                        ? list.find((m) => m.id === row.getAttribute('data-mid'))
                        : selectedMessage
                    if (toOpen) {
                        e.preventDefault()
                        openInNewTab(toOpen)
                        return
                    }
                }
            }
            if (e.key === 'Delete') {
                // A folder row owns Delete: it removes the focused custom folder, with the same confirm as
                // the row's delete button. The folder is resolved from whichever element in the tree holds
                // focus (the row after keyboard navigation or a child of it), so it does not depend on the row
                // itself being the exact focus target. A well-known folder is not deletable, so its row just
                // swallows the key rather than falling through to delete a message.
                const folderRow = target?.closest<HTMLElement>('[data-folder-list] [data-folder-id]')
                if (folderRow) {
                    e.preventDefault()
                    const folder = folders.find((f) => f.id === folderRow.getAttribute('data-folder-id'))
                    if (folder && folder.kind === 'custom') {
                        setFolderToDelete(folder)
                    }
                    return
                }
                // Otherwise Delete acts on the message selection: the Ctrl/Shift set if there is one, else the
                // active message. One target uses the single confirm; several use the count confirm.
                const selIds = markedIds.size ? markedIds : (selectedMessage ? new Set([selectedMessage.id]) : new Set<string>())
                const targets = list.filter((m) => selIds.has(m.id))
                if (targets.length === 0) {
                    return
                }
                e.preventDefault()
                if (targets.length === 1) {
                    if (e.shiftKey) {
                        setMessageToPurge(targets[0])
                    } else {
                        requestDelete(targets[0])
                    }
                } else if (e.shiftKey) {
                    setBulkToPurge(targets)
                } else {
                    setBulkToDelete(targets)
                }
            }
        }
        window.addEventListener('keydown', onKeyDown)
        return () => window.removeEventListener('keydown', onKeyDown)
    }, [
        searchActive, searchResults, displayMessages, selectedMessage, requestDelete, markedIds, anchorId,
        splashVisible, composing, settingUp, accountToEdit, managingRules, managingContacts, managingCalendar, about,
        licence, folderPrompt, messageToDelete, accountToDelete, folderToDelete, messageToPurge,
        contextMenu, messageToCancelSend, bulkToDelete, bulkToPurge, togglePreview, folders,
    ])
}
