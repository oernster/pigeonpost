import {Dispatch, SetStateAction, useEffect, useRef} from 'react'
import {Folder, Message, Tag, api} from '../api'
import {ComposeInitial} from '../components/ComposeModal'
import {MenuItem} from '../components/Menu'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {matchesShortcut} from '../shortcuts'
import {snoozeChoices} from '../schedule'
import {isJunkFolderMessage} from '../folderPaths'
import {isTextEntry} from '../editClipboard'

// undoSendChoices are the offered undo-send windows in seconds; 0 turns the hold off and sends
// immediately. defaultUndoSendSeconds is the out-of-the-box window.
export const undoSendChoices = [0, 5, 10, 20, 30] as const
export const defaultUndoSendSeconds = 10

// MenusDeps is the full action surface the menu bar projects. It is large because the menus touch nearly
// everything; each field is used verbatim by the item that names it. The derived gating flags (canMailAct and
// the rest) are computed in App because the titlebar buttons share them, so they are passed in rather than
// recomputed here.
export interface MenusDeps {
    // Derived gating values, shared with the titlebar so they stay in App.
    activeMessage: Message | null
    activeOutbox: boolean
    canMailAct: boolean
    canReplyAll: boolean
    // canMoveCopy gates Move to, Copy to and Mark as junk: false for POP3 (no server-side folders) and
    // in the unified mailbox (its rows span accounts while the folder targets belong to one).
    canMoveCopy: boolean
    selectedAccount: string
    accountSyncing: boolean
    isWindows: boolean
    conversationView: boolean
    previewEnabled: boolean
    autoLoadImages: boolean
    // unifiedMailbox is the View tick that shows the combined all-inboxes entry in the sidebar.
    unifiedMailbox: boolean
    // The undo-send window in seconds (0 = off) and its setter, shown as a Mail-menu submenu of ticks.
    undoSendSeconds: number
    setUndoSendSeconds: (seconds: number) => void
    // The folder list (Move/Copy targets) and the open message's tags (the applied ticks).
    folders: Folder[]
    messageTags: Tag[]
    // File menu.
    saveMessageAs: (message: Message) => Promise<void>
    printMessage: (message: Message) => Promise<void>
    // Edit menu. undoText / redoText name the top history entry ("Undo delete") or are null when
    // there is nothing to unwind. canCutNow / canCopyNow / canPasteNow gate the clipboard items
    // live; App computes them from the text selection and the message selection together, and the
    // three actions dispatch to whichever level applies (text wins).
    undoText: string | null
    redoText: string | null
    undoAction: () => Promise<void>
    redoAction: () => Promise<void>
    canCutNow: boolean
    canCopyNow: boolean
    canPasteNow: boolean
    cut: () => void
    copy: () => void
    paste: () => void
    selectAll: () => void
    canSelectAll: boolean
    setManagingRules: Dispatch<SetStateAction<boolean>>
    setManagingTemplates: Dispatch<SetStateAction<boolean>>
    focusSearch: () => void
    // View menu.
    toggleConversationView: () => void
    togglePreview: () => void
    toggleAutoLoadImages: () => void
    toggleUnifiedMailbox: () => void
    // Mail menu.
    signatureHtml: () => string
    setComposeInitial: Dispatch<SetStateAction<ComposeInitial | undefined>>
    setComposing: Dispatch<SetStateAction<boolean>>
    setSettingUp: Dispatch<SetStateAction<boolean>>
    sync: () => Promise<void>
    openInNewTab: (message: Message, fromKeyboard?: boolean) => void
    openReply: (message: Message) => void
    openReplyAll: (message: Message) => void
    openForward: (message: Message) => void
    attachToNewMessage: (message: Message) => void
    setReadState: (message: Message, read: boolean) => Promise<void>
    toggleFlag: (message: Message) => Promise<void>
    toggleTag: (tagId: string, assigned: boolean) => Promise<void>
    moveMessage: (message: Message, destFolderId: string) => Promise<void>
    copyMessage: (message: Message, destFolderId: string) => Promise<void>
    markJunk: (message: Message) => Promise<void>
    markNotJunk: (message: Message) => Promise<void>
    // Snooze: hide the active message until a chosen moment (a preset, or the picker dialog), and bring
    // a hidden one back.
    snoozeTo: (message: Message, at: Date) => Promise<void>
    unsnooze: (message: Message) => Promise<void>
    setSnoozePickerFor: Dispatch<SetStateAction<Message | null>>
    setMessageToCancelSend: Dispatch<SetStateAction<Message | null>>
    requestDelete: (message: Message) => void
    setMessageToPurge: Dispatch<SetStateAction<Message | null>>
    // Help menu.
    showAbout: () => Promise<void>
    showLicence: () => Promise<void>
    checkUpdates: () => void
}

export interface Menus {
    fileMenu: MenuItem[]
    editMenu: MenuItem[]
    viewMenu: MenuItem[]
    mailMenu: MenuItem[]
    helpMenu: MenuItem[]
}

// useMenus builds the five menu-bar definitions and wires the keyboard accelerators that fire the same items
// from anywhere in the main window. The definitions are rebuilt every render (like the App code they replace),
// so an item's enabled state and label always reflect the current selection; menuShortcutsRef holds the latest
// items for the global keydown handler, which is suppressed while a dialog or the context menu is open.
export function useMenus(deps: MenusDeps): Menus {
    const {
        activeMessage, activeOutbox, canMailAct, canReplyAll, canMoveCopy, selectedAccount, accountSyncing,
        isWindows, conversationView, previewEnabled, autoLoadImages, unifiedMailbox, undoSendSeconds, setUndoSendSeconds,
        folders, messageTags,
        saveMessageAs, printMessage,
        undoText, redoText, undoAction, redoAction, canCutNow, canCopyNow, canPasteNow, cut, copy, paste, selectAll, canSelectAll,
        setManagingRules, setManagingTemplates, focusSearch,
        toggleConversationView, togglePreview, toggleAutoLoadImages, toggleUnifiedMailbox,
        signatureHtml, setComposeInitial, setComposing, setSettingUp, sync, openInNewTab,
        openReply, openReplyAll, openForward, attachToNewMessage, setReadState, toggleFlag, toggleTag,
        moveMessage, copyMessage, markJunk, markNotJunk, snoozeTo, unsnooze, setSnoozePickerFor,
        setMessageToCancelSend, requestDelete, setMessageToPurge,
        showAbout, showLicence, checkUpdates,
    } = deps

    // menuShortcutsRef holds the current menu items so the global accelerator handler always sees the
    // latest labels, enabled state and callbacks without re-binding its listener on every render.
    const menuShortcutsRef = useRef<MenuItem[]>([])

    // Menu accelerators (Compose, Sync, the reading pane and any others defined on the menus) fire from
    // anywhere in the main window, driven by the same item definitions the menus render so an item's hint
    // and its wired key never drift. They are suppressed while a dialog or the context menu is open, so a
    // shortcut never acts behind one. A disabled item (Compose with no account selected, say) is skipped,
    // as is a hintOnly item (its key is owned natively or by the list keyboard handler) and a skipInText
    // item while a text field has focus (Ctrl+Z there is the field's own undo).
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (document.querySelector('.modal, .context-menu') !== null) {
                return
            }
            const inText = isTextEntry(e.target as Element | null)
            for (const item of menuShortcutsRef.current) {
                if (item.hintOnly || item.disabled || !item.onClick || (item.skipInText && inText)) {
                    continue
                }
                const shortcuts = [item.shortcut, ...(item.altShortcuts ?? [])]
                if (shortcuts.some((s) => s !== undefined && matchesShortcut(e, s))) {
                    e.preventDefault()
                    item.onClick()
                    return
                }
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [])

    const mailMoveTargets = activeMessage ? folders.filter((f) => f.id !== activeMessage.folderId) : []
    const appliedTagIds = new Set(messageTags.map((t) => t.id))
    const fileMenu: MenuItem[] = [
        {
            label: 'Save as...',
            shortcut: 'Ctrl+S',
            disabled: !canMailAct,
            onClick: () => activeMessage && void saveMessageAs(activeMessage),
        },
        {
            label: 'Print...',
            shortcut: 'Ctrl+P',
            disabled: !canMailAct,
            onClick: () => activeMessage && void printMessage(activeMessage),
        },
    ]
    // The Edit menu follows Thunderbird's 3-pane layout: undo/redo over the mail actions, then the
    // clipboard group with Delete as its message-level member, then selection, then the app's own
    // tail (Search in the slot TB gives Find). Undo and Redo yield to a focused text field, where
    // the same keys are the field's native text undo; the clipboard and Delete hints are display
    // only because their keys are owned natively or by the list keyboard handler.
    const editMenu: MenuItem[] = [
        {
            label: undoText ?? 'Undo',
            shortcut: 'Ctrl+Z',
            skipInText: true,
            disabled: undoText === null,
            onClick: () => void undoAction(),
        },
        {
            label: redoText ?? 'Redo',
            shortcut: isWindows ? 'Ctrl+Y' : 'Ctrl+Shift+Z',
            altShortcuts: ['Ctrl+Y', 'Ctrl+Shift+Z'],
            skipInText: true,
            disabled: redoText === null,
            onClick: () => void redoAction(),
        },
        {label: '', separator: true},
        {label: 'Cut', shortcut: 'Ctrl+X', hintOnly: true, disabled: !canCutNow, onClick: cut},
        {label: 'Copy', shortcut: 'Ctrl+C', hintOnly: true, disabled: !canCopyNow, onClick: copy},
        {label: 'Paste', shortcut: 'Ctrl+V', hintOnly: true, disabled: !canPasteNow, onClick: paste},
        {
            label: 'Delete',
            shortcut: 'Del',
            hintOnly: true,
            disabled: !canMailAct,
            onClick: () => activeMessage && requestDelete(activeMessage),
        },
        {
            label: 'Delete permanently',
            shortcut: 'Shift+Del',
            hintOnly: true,
            disabled: !canMailAct,
            onClick: () => activeMessage && setMessageToPurge(activeMessage),
        },
        {label: '', separator: true},
        {label: 'Select all', shortcut: 'Ctrl+A', hintOnly: true, disabled: !canSelectAll, onClick: selectAll},
        {label: '', separator: true},
        {
            label: 'Search',
            icon: '\u{1F50D}',
            shortcut: 'Ctrl+K',
            onClick: focusSearch,
        },
        {
            label: 'Rules',
            icon: '\u{1F4CF}',
            onClick: () => setManagingRules(true),
        },
        {
            label: 'Templates',
            icon: '\u{1F4C4}',
            onClick: () => setManagingTemplates(true),
        },
    ]
    const viewMenu: MenuItem[] = [
        {
            label: 'Conversation view',
            checked: conversationView,
            onClick: toggleConversationView,
        },
        {
            label: 'Unified mailbox',
            checked: unifiedMailbox,
            onClick: toggleUnifiedMailbox,
        },
        {
            label: 'Reading pane',
            shortcut: 'F8',
            checked: previewEnabled,
            onClick: togglePreview,
        },
        {
            label: 'Load images by default',
            checked: autoLoadImages,
            onClick: toggleAutoLoadImages,
        },
    ]
    const mailMenu: MenuItem[] = [
        {
            label: 'Compose',
            shortcut: 'Ctrl+N',
            disabled: !selectedAccount,
            onClick: () => {
                const sig = signatureHtml()
                setComposeInitial(sig ? {bodyHtml: `<p></p>${sig}`} : undefined)
                setComposing(true)
            },
        },
        {
            label: 'Add account',
            onClick: () => setSettingUp(true),
        },
        {
            label: accountSyncing ? 'Synchronising…' : 'Sync',
            shortcut: 'F9',
            disabled: !selectedAccount || accountSyncing,
            onClick: () => void sync(),
        },
        {
            label: 'Undo send',
            icon: '\u{23F3}',
            submenu: undoSendChoices.map((seconds) => ({
                label: seconds === 0 ? 'Off' : `${seconds} seconds`,
                checked: undoSendSeconds === seconds,
                onClick: () => setUndoSendSeconds(seconds),
            })),
        },
        ...(isWindows
            ? [{
                label: 'Set as default for .eml...',
                icon: '\u{1F4CC}',
                onClick: () => void api.showDefaultAppSettings(),
            }]
            : []),
        {label: '', separator: true},
        {
            label: 'Open in new tab',
            shortcut: 'Ctrl+T',
            disabled: !canMailAct,
            onClick: () => activeMessage && openInNewTab(activeMessage),
        },
        {label: '', separator: true},
        {
            label: 'Respond',
            icon: '\u{21A9}\u{FE0F}',
            disabled: !canMailAct,
            submenu: [
                {label: 'Reply', icon: '\u{21A9}\u{FE0F}', disabled: !canMailAct, onClick: () => activeMessage && openReply(activeMessage)},
                {label: 'Reply all', icon: '\u{1F465}', disabled: !canReplyAll, onClick: () => activeMessage && openReplyAll(activeMessage)},
                {label: 'Forward', icon: '\u{21AA}\u{FE0F}', disabled: !canMailAct, onClick: () => activeMessage && openForward(activeMessage)},
                {
                    label: 'Attach to new message',
                    icon: '\u{1F4CE}',
                    disabled: !canMailAct,
                    onClick: () => activeMessage && attachToNewMessage(activeMessage),
                },
            ],
        },
        {label: '', separator: true},
        {
            label: activeMessage?.read ? 'Mark as unread' : 'Mark as read',
            disabled: !canMailAct,
            onClick: () => activeMessage && void setReadState(activeMessage, !activeMessage.read),
        },
        {
            label: activeMessage?.flagged ? 'Remove star' : 'Add star',
            disabled: !canMailAct,
            onClick: () => activeMessage && void toggleFlag(activeMessage),
        },
        {
            label: 'Tag with colour',
            disabled: !canMailAct,
            submenu: TAG_PALETTE.map((c) => {
                const id = colourTagId(c.colour)
                const on = appliedTagIds.has(id)
                return {label: c.name, swatch: c.colour, checked: on, onClick: () => void toggleTag(id, !on)}
            }),
        },
        ...(activeMessage && activeMessage.snoozedUntilMs > 0
            ? [{
                label: 'Unsnooze',
                icon: '\u{23F0}',
                disabled: !canMailAct,
                onClick: () => activeMessage && void unsnooze(activeMessage),
            }]
            : [{
                label: 'Snooze',
                icon: '\u{23F0}',
                disabled: !canMailAct,
                submenu: [
                    ...snoozeChoices(new Date()).map((choice) => ({
                        label: choice.label,
                        onClick: () => activeMessage && void snoozeTo(activeMessage, choice.at),
                    })),
                    {label: 'Pick a time...', onClick: () => activeMessage && setSnoozePickerFor(activeMessage)},
                ],
            }]),
        {label: '', separator: true},
        {
            label: 'Move to',
            disabled: !canMailAct || !canMoveCopy || mailMoveTargets.length === 0,
            submenu: mailMoveTargets.map((f) => ({
                label: f.name,
                onClick: () => activeMessage && void moveMessage(activeMessage, f.id),
            })),
        },
        {
            label: 'Copy to',
            disabled: !canMailAct || !canMoveCopy || mailMoveTargets.length === 0,
            submenu: mailMoveTargets.map((f) => ({
                label: f.name,
                onClick: () => activeMessage && void copyMessage(activeMessage, f.id),
            })),
        },
        // A message already in Junk offers the rescue back to the inbox instead of re-junking.
        activeMessage && isJunkFolderMessage(activeMessage, folders) ? {
            label: 'Not junk',
            disabled: !canMailAct || !canMoveCopy,
            onClick: () => activeMessage && void markNotJunk(activeMessage),
        } : {
            label: 'Mark as junk',
            disabled: !canMailAct || !canMoveCopy,
            onClick: () => activeMessage && void markJunk(activeMessage),
        },
        {label: '', separator: true},
        {
            label: 'Cancel send',
            disabled: !activeOutbox,
            onClick: () => activeMessage && setMessageToCancelSend(activeMessage),
        },
    ]
    const helpMenu: MenuItem[] = [
        {label: 'About PigeonPost', onClick: () => void showAbout()},
        {label: 'Licence', onClick: () => void showLicence()},
        {label: 'Check for Updates', onClick: checkUpdates},
    ]
    menuShortcutsRef.current = [...fileMenu, ...editMenu, ...viewMenu, ...mailMenu, ...helpMenu]

    return {fileMenu, editMenu, viewMenu, mailMenu, helpMenu}
}
