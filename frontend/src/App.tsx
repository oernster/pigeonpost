import {useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState} from 'react'
import './App.css'
import brandIcon from './assets/pigeonpost.png'
import {AboutInfo, Account, api, CalendarEvent, Contact, Folder, Message, MessageBody, OutboxItem, Rule, Tag, UnreadCountsResult} from './api'
import {OUTBOX_FOLDER_ID, isOutboxMessage, outboxItemToMessage} from './outbox'
import {applyTheme, loadTheme, Theme} from './theme'
import {TAG_PALETTE, colourTagId} from './tagColours'
import {Sidebar} from './components/Sidebar'
import {MessageList} from './components/MessageList'
import {MessageContextMenu} from './components/MessageContextMenu'
import {Reader} from './components/Reader'
import {MenuBar} from './components/MenuBar'
import {AboutModal} from './components/AboutModal'
import {LicenceModal} from './components/LicenceModal'
import {ComposeInitial, ComposeModal} from './components/ComposeModal'
import {AccountSetupModal} from './components/AccountSetupModal'
import {ConfirmDialog} from './components/ConfirmDialog'
import {PromptDialog} from './components/PromptDialog'
import {RuleManagerModal} from './components/RuleManagerModal'
import {ContactsModal} from './components/ContactsModal'
import {CalendarModal} from './components/CalendarModal'
import {Splash} from './components/Splash'

// focusRingRoot is the container the ring is scoped to: the topmost open modal when one is showing (so
// focus stays trapped within the dialog), otherwise the whole document.
function focusRingRoot(): Document | HTMLElement {
    const modals = document.querySelectorAll<HTMLElement>('.modal')
    return modals.length > 0 ? modals[modals.length - 1] : document
}

// focusRingElements returns the visible, tabbable elements in document order within root: the same set
// the browser steps with Tab. Roving-tabindex lists (messages, folders) contribute a single stop each,
// because their non-current items are tabindex -1, so stepping this ring jumps region to region.
function focusRingElements(root: ParentNode): HTMLElement[] {
    const selector = [
        'a[href]', 'button:not([disabled])', 'input:not([disabled])',
        'select:not([disabled])', 'textarea:not([disabled])', '[tabindex]:not([tabindex="-1"])',
    ].join(',')
    return Array.from(root.querySelectorAll<HTMLElement>(selector)).filter((el) => {
        if (el.tabIndex < 0 || el.hasAttribute('disabled')) {
            return false
        }
        // Match the browser's own tabbability: skip hidden and unlaid-out elements.
        if (el.getClientRects().length === 0 || getComputedStyle(el).visibility === 'hidden') {
            return false
        }
        // Collapse the roving lists (messages, folders) to a single stop each: the row itself is the
        // stop, so skip the action buttons nested inside a row (Up/Down move within the list instead).
        const row = el.closest('.message-row, .list-item.folder')
        return !row || row === el
    })
}

// stepFocusRing moves focus forward (1) or back (-1) through the focus ring, wrapping at the ends, so
// Right/Left mirror Tab/Shift+Tab.
function stepFocusRing(direction: 1 | -1) {
    const items = focusRingElements(focusRingRoot())
    if (items.length === 0) {
        return
    }
    const index = items.indexOf(document.activeElement as HTMLElement)
    const next = index === -1
        ? (direction === 1 ? 0 : items.length - 1)
        : (index + direction + items.length) % items.length
    items[next]?.focus()
}

// trapTab keeps Tab and Shift+Tab inside the open dialog: it wraps at the first and last elements and
// pulls focus back in if it has somehow landed outside, while letting native Tab move between elements
// in the middle (so a rich-text editor keeps its own Tab handling).
function trapTab(e: KeyboardEvent) {
    const root = focusRingRoot()
    const items = focusRingElements(root)
    if (items.length === 0) {
        return
    }
    const first = items[0]
    const last = items[items.length - 1]
    const active = document.activeElement as HTMLElement | null
    if (e.shiftKey && active === first) {
        e.preventDefault()
        last.focus()
    } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
    } else if (!active || !root.contains(active)) {
        e.preventDefault()
        first.focus()
    }
}

function escapeHtml(s: string): string {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// subjectWithPrefix adds "Re:"/"Fwd:" unless the subject already starts with it.
function subjectWithPrefix(prefix: string, subject: string): string {
    const s = subject || '(no subject)'
    return s.toLowerCase().startsWith(prefix.toLowerCase()) ? s : `${prefix} ${s}`
}

// emlFilename builds a safe .eml filename from a message subject, replacing characters a filesystem
// rejects and falling back to a default when the subject is empty.
function emlFilename(subject: string): string {
    const cleaned = subject.replace(/[\\/:*?"<>|\x00-\x1f]/g, '-').trim()
    return `${cleaned || 'message'}.eml`
}

// printFrameId identifies the hidden iframe used for printing, so a previous one is removed before a
// new print rather than accumulating frames.
const printFrameId = 'pp-print-frame'

// autoSyncIntervalMs is how often the folder on screen is refreshed from the server in the background,
// so new mail in the open folder appears without a manual sync.
const millisPerMinute = 60 * 1000
const autoSyncIntervalMs = 5 * millisPerMinute

// printDocument renders a standalone HTML document for printing one message: a short header (subject,
// sender, date) followed by the message body. The body HTML is already sanitised server-side, so it is
// safe to inline here as it is in the reader.
function printDocument(subject: string, sender: string, date: string, contentHtml: string): string {
    const head =
        '<!doctype html><html><head><meta charset="utf-8">' +
        `<title>${subject}</title>` +
        '<style>body{font-family:sans-serif;color:#000;padding:24px}' +
        '.print-head{margin-bottom:16px;border-bottom:1px solid #ccc;padding-bottom:8px}' +
        '.print-subject{font-size:20px;font-weight:600;margin-bottom:6px}' +
        '.print-meta{color:#444;font-size:13px}img{max-width:100%}</style></head><body>'
    const header =
        `<div class="print-head"><div class="print-subject">${subject}</div>` +
        `<div class="print-meta">From: ${sender}</div>` +
        (date ? `<div class="print-meta">Date: ${date}</div>` : '') +
        '</div>'
    return `${head}${header}${contentHtml}</body></html>`
}

// neighbourAfterRemoval returns the message that selection should land on once the message with
// removedId is deleted from list: the following message, or the preceding one when it was last, or
// null when it was the only message. This keeps keyboard triage moving without a manual re-select.
function neighbourAfterRemoval(list: Message[], removedId: string): Message | null {
    const idx = list.findIndex((m) => m.id === removedId)
    if (idx === -1) {
        return null
    }
    if (idx + 1 < list.length) {
        return list[idx + 1]
    }
    if (idx - 1 >= 0) {
        return list[idx - 1]
    }
    return null
}

function App() {
    const READING_PANE_KEY = 'pigeonpost.readingPane'
    const [accounts, setAccounts] = useState<Account[]>([])
    const [selectedAccount, setSelectedAccount] = useState<string>('')
    const [folders, setFolders] = useState<Folder[]>([])
    const [unreadCounts, setUnreadCounts] = useState<UnreadCountsResult>({total: 0, byAccount: {}})
    const [selectedFolder, setSelectedFolder] = useState<string>('')
    const [messages, setMessages] = useState<Message[]>([])
    const [selectedMessage, setSelectedMessage] = useState<Message | null>(null)
    const [error, setError] = useState<string>('')
    const [syncing, setSyncing] = useState<boolean>(false)
    const [outbox, setOutbox] = useState<OutboxItem[]>([])
    const [messageToCancelSend, setMessageToCancelSend] = useState<Message | null>(null)
    const [cancellingSend, setCancellingSend] = useState<boolean>(false)
    // outboxForAccount is the queued items belonging to the selected account, shown under its Outbox
    // folder. Memoised so the derived message rows and the folder's presence are stable per render.
    const outboxForAccount = useMemo(
        () => outbox.filter((item) => item.accountId === selectedAccount),
        [outbox, selectedAccount],
    )
    const [theme, setTheme] = useState<Theme>(loadTheme())
    const [about, setAbout] = useState<AboutInfo | null>(null)
    const [licence, setLicence] = useState<string | null>(null)
    const [composing, setComposing] = useState<boolean>(false)
    const [composeInitial, setComposeInitial] = useState<ComposeInitial | undefined>(undefined)
    const [settingUp, setSettingUp] = useState<boolean>(false)
    const [accountToEdit, setAccountToEdit] = useState<Account | null>(null)
    const [accountToDelete, setAccountToDelete] = useState<Account | null>(null)
    const [folderPrompt, setFolderPrompt] = useState<{mode: 'create' | 'rename'; folder?: Folder} | null>(null)
    const [folderToDelete, setFolderToDelete] = useState<Folder | null>(null)
    const [folderBusy, setFolderBusy] = useState<boolean>(false)
    const [deleting, setDeleting] = useState<boolean>(false)
    const [tags, setTags] = useState<Tag[]>([])
    const [messageTags, setMessageTags] = useState<Tag[]>([])
    const [rules, setRules] = useState<Rule[]>([])
    const [managingRules, setManagingRules] = useState<boolean>(false)
    const [contacts, setContacts] = useState<Contact[]>([])
    const [managingContacts, setManagingContacts] = useState<boolean>(false)
    const [events, setEvents] = useState<CalendarEvent[]>([])
    const [managingCalendar, setManagingCalendar] = useState<boolean>(false)
    const [messageBody, setMessageBody] = useState<MessageBody | null>(null)
    const [bodyLoading, setBodyLoading] = useState<boolean>(false)
    const [searchQuery, setSearchQuery] = useState<string>('')
    const [searchResults, setSearchResults] = useState<Message[]>([])
    const [messageToDelete, setMessageToDelete] = useState<Message | null>(null)
    const [deletingMessage, setDeletingMessage] = useState<boolean>(false)
    const [messageToPurge, setMessageToPurge] = useState<Message | null>(null)
    const [purgingMessage, setPurgingMessage] = useState<boolean>(false)
    const [contextMenu, setContextMenu] = useState<{message: Message; x: number; y: number} | null>(null)
    const [tabs, setTabs] = useState<Message[]>([])
    // previewEnabled controls the right-hand reading pane. When off, the message list is full-width and
    // a message is read by double-clicking it, which opens it full-width (readingFull) with a Back button.
    const [previewEnabled, setPreviewEnabled] = useState<boolean>(() => {
        try {
            return localStorage.getItem(READING_PANE_KEY) !== 'off'
        } catch {
            return true
        }
    })
    const [readingFull, setReadingFull] = useState<boolean>(false)
    // Tracks the folder currently on screen, so a background refresh only replaces the list when the
    // user has not navigated away since it started.
    const selectedFolderRef = useRef<string>('')

    // A neutral, offscreen focus anchor. It takes focus once on launch so nothing is highlighted, yet
    // the very first Tab has a starting point and moves to the first control in the title tray. Without
    // it the WebView starts with focus on no element and the first Tab does nothing.
    const neutralFocusRef = useRef<HTMLSpanElement>(null)
    useEffect(() => {
        neutralFocusRef.current?.focus()
    }, [])

    const searchActive = searchQuery.trim() !== ''
    const [appVersion, setAppVersion] = useState<string>('')
    const [appAuthor, setAppAuthor] = useState<string>('')
    const [splashVisible, setSplashVisible] = useState<boolean>(true)
    const [splashFading, setSplashFading] = useState<boolean>(false)

    useEffect(() => {
        selectedFolderRef.current = selectedFolder
    }, [selectedFolder])

    // Apply the theme before the browser paints, so a toggle changes the emoji and the colours in the
    // same frame rather than repainting twice (the flash).
    useLayoutEffect(() => {
        applyTheme(theme)
    }, [theme])

    useEffect(() => {
        void api.version().then(setAppVersion).catch(() => undefined)
        void api.author().then(setAppAuthor).catch(() => undefined)
        const fade = window.setTimeout(() => setSplashFading(true), 1600)
        const hide = window.setTimeout(() => setSplashVisible(false), 2000)
        return () => {
            window.clearTimeout(fade)
            window.clearTimeout(hide)
        }
    }, [])

    const loadAccounts = useCallback(async () => {
        try {
            setAccounts(await api.listAccounts())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    useEffect(() => {
        void loadAccounts()
    }, [loadAccounts])

    const loadRules = useCallback(async () => {
        try {
            setRules(await api.listRules())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    useEffect(() => {
        void loadRules()
    }, [loadRules])

    const loadContacts = useCallback(async () => {
        try {
            setContacts(await api.listContacts())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    useEffect(() => {
        void loadContacts()
    }, [loadContacts])

    const loadEvents = useCallback(async () => {
        try {
            setEvents(await api.listEvents())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    useEffect(() => {
        void loadEvents()
    }, [loadEvents])

    // loadUnread refreshes the per-account and cross-account unread counts from the local cache. It is
    // called after anything that can change read state (sync, mark read/unread, delete, opening a
    // folder) so the sidebar and titlebar badges stay correct.
    const loadUnread = useCallback(async () => {
        try {
            setUnreadCounts(await api.unreadCounts())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    useEffect(() => {
        void loadUnread()
    }, [loadUnread])

    // Ensure the fixed colour palette exists as tags, so a colour can be applied and its swatch shown. The
    // writes are sequential and only fill in missing colours, because SQLite is single-writer and firing
    // them at once trips "database is locked".
    useEffect(() => {
        void (async () => {
            try {
                const existing = await api.listTags()
                const have = new Set(existing.map((t) => t.id))
                for (const c of TAG_PALETTE) {
                    const id = colourTagId(c.colour)
                    if (!have.has(id)) {
                        await api.saveTag({id, name: c.name, colour: c.colour})
                    }
                }
                setTags(await api.listTags())
            } catch (e) {
                setError(String(e))
            }
        })()
    }, [])

    // Load the tags attached to the selected message whenever the selection changes.
    useEffect(() => {
        if (!selectedMessage) {
            setMessageTags([])
            return
        }
        const messageId = selectedMessage.id
        let active = true
        void api.messageTags(messageId).then((t) => {
            if (active) {
                setMessageTags(t)
            }
        }).catch((e) => setError(String(e)))
        return () => {
            active = false
        }
    }, [selectedMessage])

    // Fetch (and cache) the full body of the selected message. Keyed on the id so re-selecting the
    // same message after a flag change does not re-fetch.
    useEffect(() => {
        if (!selectedMessage) {
            setMessageBody(null)
            return
        }
        // An outbox message is not in the store; show the queued body directly (no fetch).
        if (isOutboxMessage(selectedMessage)) {
            const item = outbox.find((o) => o.id === selectedMessage.id)
            setMessageBody({plain: item?.body ?? '', html: ''})
            setBodyLoading(false)
            return
        }
        const messageId = selectedMessage.id
        let active = true
        setMessageBody(null)
        setBodyLoading(true)
        void api.messageBody(messageId)
            .then((b) => {
                if (active) {
                    setMessageBody(b)
                }
            })
            .catch((e) => {
                if (active) {
                    setError(String(e))
                }
            })
            .finally(() => {
                if (active) {
                    setBodyLoading(false)
                }
            })
        return () => {
            active = false
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [selectedMessage?.id])

    // Debounced full-text search: results replace the folder listing while a query is active.
    useEffect(() => {
        const q = searchQuery.trim()
        if (q === '') {
            setSearchResults([])
            return
        }
        const handle = window.setTimeout(() => {
            void api.searchMessages(q).then(setSearchResults).catch((e) => setError(String(e)))
        }, 250)
        return () => window.clearTimeout(handle)
    }, [searchQuery])

    const toggleTag = useCallback(async (tagId: string, assigned: boolean) => {
        if (!selectedMessage) {
            return
        }
        try {
            await api.setMessageTag(selectedMessage.id, tagId, assigned)
            setMessageTags(await api.messageTags(selectedMessage.id))
        } catch (e) {
            setError(String(e))
        }
    }, [selectedMessage])

    // loadFolderMessages shows a folder's cached messages immediately (so it opens instantly), then
    // refreshes it from the server and updates the list if the user is still on that folder. This is
    // what makes a message moved or deleted into a folder appear when the folder is opened. A sync
    // failure (offline) simply leaves the cached view in place.
    const loadFolderMessages = useCallback(async (id: string) => {
        try {
            setMessages(await api.listMessages(id))
        } catch (e) {
            setError(String(e))
        }
        try {
            await api.syncFolder(id)
            if (selectedFolderRef.current === id) {
                setMessages(await api.listMessages(id))
            }
            await loadUnread()
        } catch {
            // Offline or a transient failure: the cached view stands.
        }
    }, [loadUnread])

    const selectAccount = useCallback(async (id: string) => {
        setSelectedAccount(id)
        setSelectedMessage(null)
        setReadingFull(false)
        try {
            const fetched = await api.listFolders(id)
            setFolders(fetched)
            // Open the account's Inbox straight away (falling back to its first folder) so its messages
            // are visible without a manual click.
            const inbox = fetched.find((f) => f.kind === 'inbox') ?? fetched[0]
            if (inbox) {
                selectedFolderRef.current = inbox.id
                setSelectedFolder(inbox.id)
                await loadFolderMessages(inbox.id)
            } else {
                selectedFolderRef.current = ''
                setSelectedFolder('')
                setMessages([])
            }
        } catch (e) {
            setError(String(e))
        }
    }, [loadFolderMessages])

    const selectFolder = useCallback(async (id: string) => {
        selectedFolderRef.current = id
        setSelectedFolder(id)
        setSelectedMessage(null)
        setReadingFull(false)
        // The Outbox is a synthetic folder: it lists the account's queued items from local state rather
        // than syncing a server mailbox.
        if (id === OUTBOX_FOLDER_ID) {
            setMessages(outboxForAccount.map(outboxItemToMessage))
            return
        }
        try {
            await loadFolderMessages(id)
        } catch (e) {
            setError(String(e))
        }
    }, [loadFolderMessages, outboxForAccount])

    // On first load (or after the account list changes) open the default account automatically, so the
    // app lands on a populated inbox rather than an empty pane.
    useEffect(() => {
        if (!selectedAccount && accounts.length > 0) {
            void selectAccount(accounts[0].id)
        }
    }, [accounts, selectedAccount, selectAccount])

    // refreshOutbox reloads the queue of outgoing operations waiting to be sent. The queue is surfaced
    // as a per-account Outbox folder, so the full item list is kept, not just a count.
    const refreshOutbox = useCallback(async () => {
        try {
            setOutbox(await api.listOutbox())
        } catch {
            // A queue read failing must not disrupt the UI; leave the last known value.
        }
    }, [])

    // Keep the Outbox view live while it is open: re-map the rows when the queue changes, drop a
    // selection whose item was cancelled or sent, and fall back to the inbox once the queue is empty
    // (the synthetic folder then disappears from the sidebar).
    useEffect(() => {
        if (selectedFolder !== OUTBOX_FOLDER_ID) {
            return
        }
        if (outboxForAccount.length === 0) {
            const fallback = folders.find((f) => f.kind === 'inbox') ?? folders[0]
            if (fallback) {
                selectedFolderRef.current = fallback.id
                setSelectedFolder(fallback.id)
                setSelectedMessage(null)
                void loadFolderMessages(fallback.id)
            } else {
                selectedFolderRef.current = ''
                setSelectedFolder('')
                setMessages([])
            }
            return
        }
        setMessages(outboxForAccount.map(outboxItemToMessage))
        setSelectedMessage((prev) =>
            prev && isOutboxMessage(prev) && !outboxForAccount.some((o) => o.id === prev.id) ? null : prev)
    }, [outboxForAccount, selectedFolder, folders, loadFolderMessages])

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

    const sync = useCallback(async () => {
        if (!selectedAccount) {
            return
        }
        setSyncing(true)
        setError('')
        try {
            await api.syncAccount(selectedAccount)
            // Connectivity is back: flush anything queued while offline, then refresh views.
            await api.replayOutbox()
            setFolders(await api.listFolders(selectedAccount))
            if (selectedFolder) {
                setMessages(await api.listMessages(selectedFolder))
            }
            await refreshOutbox()
            await loadUnread()
        } catch (e) {
            setError(String(e))
        } finally {
            setSyncing(false)
        }
    }, [selectedAccount, selectedFolder, refreshOutbox, loadUnread])

    useEffect(() => {
        void refreshOutbox()
    }, [refreshOutbox])

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

    const onAccountSaved = useCallback(async (email: string) => {
        setSettingUp(false)
        setAccountToEdit(null)
        await loadAccounts()
        await selectAccount(email)
    }, [loadAccounts, selectAccount])

    const removeAccount = useCallback(async () => {
        if (!accountToDelete) {
            return
        }
        setDeleting(true)
        setError('')
        try {
            await api.removeAccount(accountToDelete.id)
            if (accountToDelete.id === selectedAccount) {
                setSelectedAccount('')
                setFolders([])
                setSelectedFolder('')
                setMessages([])
                setSelectedMessage(null)
            }
            await loadAccounts()
            setAccountToDelete(null)
        } catch (e) {
            setError(String(e))
        } finally {
            setDeleting(false)
        }
    }, [accountToDelete, selectedAccount, loadAccounts])

    const deleteMessage = useCallback(async () => {
        if (!messageToDelete) {
            return
        }
        const id = messageToDelete.id
        const list = searchActive ? searchResults : messages
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
    }, [messageToDelete, searchActive, searchResults, messages, loadUnread])

    // deletePermanent is the confirmed, irreversible delete behind Shift+Delete: it removes the message
    // from the server without moving it to Trash, then advances the selection.
    const deletePermanent = useCallback(async () => {
        if (!messageToPurge) {
            return
        }
        const id = messageToPurge.id
        const list = searchActive ? searchResults : messages
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
    }, [messageToPurge, searchActive, searchResults, messages, loadUnread])

    // requestDelete always asks for confirmation before deleting. The confirmed delete moves the
    // message to Trash where the account has one, or removes it permanently otherwise (the dialog says
    // which). Shared by the Delete key and the context menu.
    const requestDelete = useCallback((message: Message) => {
        setMessageToDelete(message)
    }, [])

    const closeContextMenu = useCallback(() => setContextMenu(null), [])

    // selectMessage highlights a message. With the reading pane on it shows in the preview (and the view
    // effect marks it read); with the pane off it only highlights the row and stays on the list.
    const selectMessage = useCallback((message: Message) => {
        setSelectedMessage(message)
        setReadingFull(false)
    }, [])

    // openInNewTab pins a message as a reader tab (if not already open) and shows it. With the reading
    // pane off this opens the message full-width (readingFull); with it on the tab appears in the pane.
    const openInNewTab = useCallback((message: Message) => {
        setTabs((prev) => (prev.some((t) => t.id === message.id) ? prev : [...prev, message]))
        setSelectedMessage(message)
        setReadingFull(true)
    }, [])

    // togglePreview flips the reading pane and returns to the list, so toggling never strands the user in
    // the full-width reader.
    const togglePreview = useCallback(() => {
        setPreviewEnabled((v) => !v)
        setReadingFull(false)
    }, [])

    // closeTab removes a tab; if it was the message on screen, selection moves to the neighbouring tab
    // (or clears when none remain).
    const closeTab = useCallback((id: string) => {
        setTabs((prev) => {
            const idx = prev.findIndex((t) => t.id === id)
            const next = prev.filter((t) => t.id !== id)
            setSelectedMessage((sel) => (sel?.id === id ? (next[Math.min(idx, next.length - 1)] ?? null) : sel))
            return next
        })
    }, [])

    // openContextMenu selects the right-clicked message (so the reader and actions target it) and opens
    // the menu at the cursor.
    const openContextMenu = useCallback((message: Message, x: number, y: number) => {
        setSelectedMessage(message)
        setContextMenu({message, x, y})
    }, [])

    // setMessageTagById toggles a tag on any message (not only the selected one), used by the context
    // menu. When it targets the open message, its tag chips are refreshed too.
    const setMessageTagById = useCallback(async (messageId: string, tagId: string, assigned: boolean) => {
        try {
            await api.setMessageTag(messageId, tagId, assigned)
            if (selectedMessage?.id === messageId) {
                setMessageTags(await api.messageTags(messageId))
            }
        } catch (e) {
            setError(String(e))
        }
    }, [selectedMessage])

    // saveMessageAs exports the message as a .eml file via a native save dialog, named from its subject.
    const saveMessageAs = useCallback(async (message: Message) => {
        try {
            await api.saveMessageAs(message.id, emlFilename(message.subject || ''))
        } catch (e) {
            setError(String(e))
        }
    }, [])

    // printMessage prints one message by rendering it into a hidden iframe and invoking the browser's
    // print dialog on that frame, so only the message (not the whole app window) is printed. Remote
    // images, parked in the reader for privacy, are restored for the printed copy.
    const printMessage = useCallback(async (message: Message) => {
        try {
            const body = await api.messageBody(message.id)
            const html = body.html?.trim() ? body.html.replace(/data-pp-src=/g, 'src=') : ''
            const content = html || `<pre>${escapeHtml(body.plain || message.snippet || '')}</pre>`
            const sender = escapeHtml(message.fromName || message.fromAddress || '(unknown sender)')
            const when = message.date ? escapeHtml(new Date(message.date).toLocaleString()) : ''
            const doc = printDocument(escapeHtml(message.subject || '(no subject)'), sender, when, content)

            document.getElementById(printFrameId)?.remove()
            const frame = document.createElement('iframe')
            frame.id = printFrameId
            frame.setAttribute('aria-hidden', 'true')
            frame.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0'
            frame.onload = () => {
                const win = frame.contentWindow
                if (!win) {
                    return
                }
                win.onafterprint = () => frame.remove()
                win.focus()
                win.print()
            }
            document.body.appendChild(frame)
            frame.srcdoc = doc
        } catch (e) {
            setError(String(e))
        }
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

    // moveMessageById moves a message to a folder by id, used by both the menu (via moveMessage) and
    // drag-and-drop. A no-op when the message is dropped on the folder it already lives in.
    const moveMessageById = useCallback(async (messageId: string, destFolderId: string) => {
        setError('')
        try {
            await api.moveMessage(messageId, destFolderId)
            setMessages((prev) => prev.filter((m) => m.id !== messageId))
            setSearchResults((prev) => prev.filter((m) => m.id !== messageId))
            setTabs((prev) => prev.filter((m) => m.id !== messageId))
            setSelectedMessage((prev) => (prev?.id === messageId ? null : prev))
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const moveMessage = useCallback(
        (message: Message, destFolderId: string) => moveMessageById(message.id, destFolderId),
        [moveMessageById],
    )

    // dropMessageOnFolder is the drag-and-drop target handler: it moves the dragged message into the
    // folder it was dropped on, unless that is already its folder.
    const dropMessageOnFolder = useCallback((messageId: string, folderId: string) => {
        const source = messages.find((m) => m.id === messageId) ?? searchResults.find((m) => m.id === messageId)
        if (source && source.folderId === folderId) {
            return
        }
        // The Outbox is synthetic: nothing can be moved into it, and a queued item cannot be moved out.
        if (folderId === OUTBOX_FOLDER_ID || (source && isOutboxMessage(source))) {
            return
        }
        void moveMessageById(messageId, folderId)
    }, [messages, searchResults, moveMessageById])

    // Copy leaves the original in place; the duplicate appears in the destination folder on next sync,
    // so there is no local list change to make here.
    const copyMessage = useCallback(async (message: Message, destFolderId: string) => {
        setError('')
        try {
            await api.copyMessage(message.id, destFolderId)
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const refreshFolders = useCallback(async () => {
        if (selectedAccount) {
            setFolders(await api.listFolders(selectedAccount))
        }
    }, [selectedAccount])

    // submitFolderPrompt handles both create and rename from the shared PromptDialog.
    const submitFolderPrompt = useCallback(async (value: string) => {
        if (!folderPrompt) {
            return
        }
        setFolderBusy(true)
        setError('')
        try {
            if (folderPrompt.mode === 'create') {
                await api.createFolder(selectedAccount, value)
            } else if (folderPrompt.folder) {
                await api.renameFolder(folderPrompt.folder.id, value)
            }
            await refreshFolders()
            setFolderPrompt(null)
        } catch (e) {
            setError(String(e))
        } finally {
            setFolderBusy(false)
        }
    }, [folderPrompt, selectedAccount, refreshFolders])

    const confirmDeleteFolder = useCallback(async () => {
        if (!folderToDelete) {
            return
        }
        setFolderBusy(true)
        setError('')
        try {
            await api.deleteFolder(folderToDelete.id)
            if (folderToDelete.id === selectedFolder) {
                setSelectedFolder('')
                setMessages([])
                setSelectedMessage(null)
            }
            await refreshFolders()
            setFolderToDelete(null)
        } catch (e) {
            setError(String(e))
        } finally {
            setFolderBusy(false)
        }
    }, [folderToDelete, selectedFolder, refreshFolders])

    // quoteFor returns the quoted original for reply/forward: the fetched HTML body when available,
    // otherwise the plain text (or snippet) escaped into a paragraph.
    const quoteFor = (message: Message): string => {
        if (messageBody?.html && messageBody.html.trim() !== '') {
            return messageBody.html
        }
        return `<p>${escapeHtml(messageBody?.plain || message.snippet || '')}</p>`
    }

    const openReply = (message: Message) => {
        const when = message.date ? new Date(message.date).toLocaleString() : ''
        const who = message.fromName || message.fromAddress || 'the sender'
        const header = when ? `On ${when}, ${who} wrote:` : `${who} wrote:`
        setComposeInitial({
            to: message.fromAddress,
            subject: subjectWithPrefix('Re:', message.subject),
            bodyHtml: `<p></p><p>${escapeHtml(header)}</p><blockquote>${quoteFor(message)}</blockquote>`,
        })
        setComposing(true)
    }

    const openReplyAll = (message: Message) => {
        const when = message.date ? new Date(message.date).toLocaleString() : ''
        const who = message.fromName || message.fromAddress || 'the sender'
        const header = when ? `On ${when}, ${who} wrote:` : `${who} wrote:`
        // Address the sender plus everyone on the original To and Cc, dropping our own address and any
        // duplicates so we never reply to ourselves or twice to the same person.
        const seen = new Set<string>([selectedAccount.toLowerCase()])
        const collect = (address: string, into: string[]) => {
            const key = address.trim().toLowerCase()
            if (key !== '' && !seen.has(key)) {
                seen.add(key)
                into.push(address.trim())
            }
        }
        const toList: string[] = []
        const ccList: string[] = []
        collect(message.fromAddress, toList)
        ;(message.to || []).forEach((a) => collect(a.address, toList))
        ;(message.cc || []).forEach((a) => collect(a.address, ccList))
        setComposeInitial({
            to: toList.join(', '),
            cc: ccList.join(', '),
            subject: subjectWithPrefix('Re:', message.subject),
            bodyHtml: `<p></p><p>${escapeHtml(header)}</p><blockquote>${quoteFor(message)}</blockquote>`,
        })
        setComposing(true)
    }

    const openForward = (message: Message) => {
        const who = message.fromName || message.fromAddress || 'unknown sender'
        setComposeInitial({
            to: '',
            subject: subjectWithPrefix('Fwd:', message.subject),
            bodyHtml:
                '<p></p><p>---------- Forwarded message ----------</p>' +
                `<p>From: ${escapeHtml(who)}<br>Subject: ${escapeHtml(message.subject || '(no subject)')}</p>` +
                `<blockquote>${quoteFor(message)}</blockquote>`,
        })
        setComposing(true)
    }

    // attachToNewMessage opens a fresh composer with the chosen message attached as a .eml; the backend
    // fetches its raw bytes and adds it as a message/rfc822 part on send.
    const attachToNewMessage = (message: Message) => {
        setComposeInitial({
            messageAttachments: [{id: message.id, name: emlFilename(message.subject || '')}],
        })
        setComposing(true)
    }

    const showAbout = useCallback(async () => {
        try {
            setAbout(await api.about())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const showLicence = useCallback(async () => {
        try {
            setLicence(await api.licence())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const checkUpdates = useCallback(() => {
        void api.openReleases()
    }, [])

    // setReadState sets a message's read flag on the server and optimistically in the on-screen lists,
    // so it bolds or un-bolds at once. Used by the Mark submenu (explicit read/unread) and on view.
    const setReadState = useCallback(async (message: Message, read: boolean) => {
        const apply = (m: Message): Message => (m.id === message.id ? {...m, read} : m)
        setMessages((prev) => prev.map(apply))
        setSearchResults((prev) => prev.map(apply))
        setTabs((prev) => prev.map(apply))
        setSelectedMessage((prev) => (prev && prev.id === message.id ? {...prev, read} : prev))
        try {
            await api.markRead(message.id, read)
            await loadUnread()
        } catch (e) {
            setError(String(e))
        }
    }, [loadUnread])

    // markReadOnView marks a message read when it is displayed, unless it already is.
    const markReadOnView = useCallback((message: Message) => {
        if (!message.read) {
            void setReadState(message, true)
        }
    }, [setReadState])

    // Persist the reading-pane preference so it survives a restart.
    useEffect(() => {
        try {
            localStorage.setItem(READING_PANE_KEY, previewEnabled ? 'on' : 'off')
        } catch {
            // A storage failure just means the preference is not remembered; the UI still works.
        }
    }, [previewEnabled])

    // A message shown in the reader (the preview pane, or the full-width reader when the pane is off)
    // counts as read, so viewing or double-clicking a message un-bolds it.
    useEffect(() => {
        if ((previewEnabled || readingFull) && selectedMessage && !selectedMessage.read) {
            void markReadOnView(selectedMessage)
        }
    }, [selectedMessage, previewEnabled, readingFull, markReadOnView])

    const toggleRead = useCallback(async (message: Message) => {
        try {
            await api.markRead(message.id, !message.read)
            if (selectedFolder) {
                const refreshed = await api.listMessages(selectedFolder)
                setMessages(refreshed)
                setSelectedMessage(refreshed.find((m) => m.id === message.id) ?? null)
            }
        } catch (e) {
            setError(String(e))
        }
    }, [selectedFolder])

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
            Boolean(messageToPurge) || Boolean(contextMenu)
        const list = searchActive ? searchResults : messages
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

            if (isText) {
                return
            }
            // Right/Left step the focus ring, mirroring Tab/Shift+Tab across the main window. Non-dialog
            // overlays (context menu, splash) have nothing to navigate, so it stays disabled for them.
            if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                if (overlayOpen) {
                    return
                }
                e.preventDefault()
                stepFocusRing(e.key === 'ArrowRight' ? 1 : -1)
                return
            }
            if (overlayOpen) {
                return
            }
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                // The folder list owns its own Up/Down (it navigates folders); do not also move the
                // message selection when focus is within it.
                if (target && target.closest('[data-folder-list]')) {
                    return
                }
                if (list.length === 0) {
                    return
                }
                e.preventDefault()
                const idx = selectedMessage ? list.findIndex((m) => m.id === selectedMessage.id) : -1
                if (e.key === 'ArrowDown') {
                    setSelectedMessage(idx === -1 ? list[0] : list[Math.min(idx + 1, list.length - 1)])
                } else {
                    setSelectedMessage(idx === -1 ? list[list.length - 1] : list[Math.max(idx - 1, 0)])
                }
                return
            }
            if (e.key === 'Delete' && selectedMessage) {
                e.preventDefault()
                if (e.shiftKey) {
                    setMessageToPurge(selectedMessage)
                } else {
                    requestDelete(selectedMessage)
                }
            }
        }
        window.addEventListener('keydown', onKeyDown)
        return () => window.removeEventListener('keydown', onKeyDown)
    }, [
        searchActive, searchResults, messages, selectedMessage, requestDelete,
        splashVisible, composing, settingUp, accountToEdit, managingRules, managingContacts, managingCalendar, about,
        licence, folderPrompt, messageToDelete, accountToDelete, folderToDelete, messageToPurge,
        contextMenu, messageToCancelSend,
    ])

    // A POP3 account has a single downloaded inbox with no server-side folders, message moves or draft
    // mailbox, so those actions are hidden and a delete is permanent rather than a move to Trash.
    const activeAccount = accounts.find((a) => a.id === selectedAccount)
    const isPop3 = activeAccount?.protocol === 'pop3'

    // The message list and reader are extracted so the reading-pane layout can place them side by side
    // (pane on) or swap between them (pane off: list, or the full-width reader when a message is opened).
    const messageListEl = (
        <MessageList
            messages={searchActive ? searchResults : messages}
            selectedMessage={selectedMessage}
            folderSelected={Boolean(selectedFolder)}
            searchQuery={searchQuery}
            searchActive={searchActive}
            onSearchChange={setSearchQuery}
            onSelectMessage={selectMessage}
            onToggleFlag={(m) => void toggleFlag(m)}
            onContextMenu={openContextMenu}
            onOpenInNewTab={openInNewTab}
        />
    )
    const readerEl = (
        <Reader
            message={selectedMessage}
            onToggleRead={(m) => void toggleRead(m)}
            onReply={openReply}
            onReplyAll={openReplyAll}
            onForward={openForward}
            onDelete={(m) => setMessageToDelete(m)}
            onCancelSend={(m) => setMessageToCancelSend(m)}
            folders={folders}
            onMove={(m, dest) => void moveMessage(m, dest)}
            onCopy={(m, dest) => void copyMessage(m, dest)}
            canMoveCopy={!isPop3}
            tags={tags}
            messageTags={messageTags}
            onToggleTag={(tagId, assigned) => void toggleTag(tagId, assigned)}
            body={messageBody}
            bodyLoading={bodyLoading}
            tabs={tabs}
            onSelectTab={setSelectedMessage}
            onCloseTab={closeTab}
            onBack={previewEnabled ? undefined : () => setReadingFull(false)}
        />
    )

    return (
        <div className="app">
            <span
                ref={neutralFocusRef}
                tabIndex={-1}
                aria-hidden="true"
                style={{position: 'absolute', width: 0, height: 0, overflow: 'hidden', outline: 'none'}}
            />
            {splashVisible && <Splash version={appVersion} author={appAuthor} fading={splashFading}/>}
            <header className="titlebar">
                <span className="brand">
                    PigeonPost
                    {unreadCounts.total > 0 && (
                        <span className="titlebar-unread" title={`${unreadCounts.total} unread across all accounts`}>
                            {unreadCounts.total}
                        </span>
                    )}
                </span>
                <div className="titlebar-right">
                    <button
                        className="sync-btn"
                        data-tip="Compose"
                        aria-label="Compose"
                        disabled={!selectedAccount}
                        onClick={() => {
                            setComposeInitial(undefined)
                            setComposing(true)
                        }}
                    >
                        {'\u{1F58A}\u{FE0F}'}
                    </button>
                    <button
                        className="sync-btn"
                        data-tip={syncing ? 'Syncing…' : 'Sync'}
                        aria-label="Sync"
                        disabled={!selectedAccount || syncing}
                        onClick={() => void sync()}
                    >
                        {'\u{267B}\u{FE0F}'}
                    </button>
                    <button
                        className="icon-btn"
                        data-tip={previewEnabled ? 'Hide the reading pane' : 'Show the reading pane'}
                        aria-label={previewEnabled ? 'Hide the reading pane' : 'Show the reading pane'}
                        aria-pressed={previewEnabled}
                        onClick={togglePreview}
                    >
                        {previewEnabled ? '◫\u{FE0E}' : '▯\u{FE0E}'}
                    </button>
                    <span className="titlebar-sep" aria-hidden="true"/>
                    <button className="sync-btn" onClick={() => setSettingUp(true)}>
                        {'\u{2795}'} Add account
                    </button>
                    <button className="sync-btn" onClick={() => setManagingRules(true)}>
                        Rules
                    </button>
                    <button className="sync-btn" onClick={() => setManagingContacts(true)}>
                        Contacts
                    </button>
                    <button className="sync-btn" onClick={() => setManagingCalendar(true)}>
                        Calendar
                    </button>
                    <span className="titlebar-sep" aria-hidden="true"/>
                    <button
                        className="icon-btn theme-toggle"
                        data-tip={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                        aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                        onClick={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
                    >
                        {theme === 'dark' ? '☀️' : '\u{1F319}'}
                    </button>
                    <MenuBar
                        onShowAbout={() => void showAbout()}
                        onShowLicence={() => void showLicence()}
                        onCheckUpdates={checkUpdates}
                    />
                </div>
            </header>
            {error && <div className="error-bar" role="alert">{error}</div>}
            {accounts.length === 0 && !splashVisible ? (
                <div className="empty-state welcome">
                    <img className="welcome-brand" src={brandIcon} alt="" aria-hidden="true"/>
                    <div className="empty-card">
                        <h2>Welcome to PigeonPost</h2>
                        <p>Add a mail account to start reading and sending messages.</p>
                        <button className="btn primary" onClick={() => setSettingUp(true)}>Add account</button>
                    </div>
                </div>
            ) : (
            <div className={'panes' + (previewEnabled ? '' : ' no-preview')}>
                <Sidebar
                    accounts={accounts}
                    selectedAccount={selectedAccount}
                    unreadByAccount={unreadCounts.byAccount}
                    folders={sidebarFolders}
                    selectedFolder={selectedFolder}
                    onSelectAccount={(id) => void selectAccount(id)}
                    onSelectFolder={(id) => void selectFolder(id)}
                    onEditAccount={(account) => setAccountToEdit(account)}
                    onDeleteAccount={(account) => setAccountToDelete(account)}
                    onNewFolder={() => setFolderPrompt({mode: 'create'})}
                    onRenameFolder={(folder) => setFolderPrompt({mode: 'rename', folder})}
                    onDeleteFolder={(folder) => setFolderToDelete(folder)}
                    onDropMessage={dropMessageOnFolder}
                    canManageFolders={!isPop3}
                />
                {previewEnabled ? (
                    <>
                        {messageListEl}
                        {readerEl}
                    </>
                ) : readingFull && selectedMessage ? (
                    readerEl
                ) : (
                    messageListEl
                )}
            </div>
            )}
            <AboutModal about={about} onClose={() => setAbout(null)}/>
            <LicenceModal text={licence} onClose={() => setLicence(null)}/>
            {composing && selectedAccount && (
                <ComposeModal
                    accountId={selectedAccount}
                    initial={composeInitial}
                    canSaveDraft={!isPop3}
                    onClose={() => {
                        setComposing(false)
                        setComposeInitial(undefined)
                        // A message composed while offline is queued: reflect that in the count.
                        void refreshOutbox()
                    }}
                />
            )}
            {settingUp && (
                <AccountSetupModal onClose={() => setSettingUp(false)} onSaved={(email) => void onAccountSaved(email)}/>
            )}
            {accountToEdit && (
                <AccountSetupModal
                    account={accountToEdit}
                    onClose={() => setAccountToEdit(null)}
                    onSaved={(email) => void onAccountSaved(email)}
                />
            )}
            {managingContacts && (
                <ContactsModal
                    contacts={contacts}
                    onChanged={() => void loadContacts()}
                    onClose={() => setManagingContacts(false)}
                />
            )}
            {managingCalendar && (
                <CalendarModal
                    events={events}
                    onChanged={() => void loadEvents()}
                    onClose={() => setManagingCalendar(false)}
                />
            )}
            {managingRules && (
                <RuleManagerModal rules={rules} onChanged={() => void loadRules()} onClose={() => setManagingRules(false)}/>
            )}
            {messageToCancelSend && (
                <ConfirmDialog
                    title="Cancel send"
                    message={`Cancel sending "${messageToCancelSend.subject || '(no subject)'}"? The queued email is discarded and will not be sent.`}
                    confirmLabel="Cancel send"
                    busy={cancellingSend}
                    onConfirm={() => void cancelSend()}
                    onCancel={() => setMessageToCancelSend(null)}
                />
            )}
            {messageToDelete && (
                <ConfirmDialog
                    title="Delete message"
                    message={isPop3
                        ? `Delete "${messageToDelete.subject || '(no subject)'}"? POP3 has no Trash, so it is permanently removed from the server and cannot be recovered.`
                        : `Delete "${messageToDelete.subject || '(no subject)'}"? It is moved to Trash, or deleted permanently if it is already in Trash or the account has no Trash folder.`}
                    confirmLabel="Delete"
                    busy={deletingMessage}
                    onConfirm={() => void deleteMessage()}
                    onCancel={() => setMessageToDelete(null)}
                />
            )}
            {messageToPurge && (
                <ConfirmDialog
                    title="Delete permanently"
                    message={`Permanently delete "${messageToPurge.subject || '(no subject)'}"? It is removed from the server and cannot be recovered.`}
                    confirmLabel="Delete permanently"
                    busy={purgingMessage}
                    onConfirm={() => void deletePermanent()}
                    onCancel={() => setMessageToPurge(null)}
                />
            )}
            {contextMenu && (
                <MessageContextMenu
                    message={contextMenu.message}
                    x={contextMenu.x}
                    y={contextMenu.y}
                    folders={folders}
                    tags={tags}
                    onClose={closeContextMenu}
                    onReply={openReply}
                    onReplyAll={openReplyAll}
                    onForward={openForward}
                    onSetRead={(m, read) => void setReadState(m, read)}
                    onToggleFlag={(m) => void toggleFlag(m)}
                    onMove={(m, dest) => void moveMessage(m, dest)}
                    onCopy={(m, dest) => void copyMessage(m, dest)}
                    canMoveCopy={!isPop3}
                    onSetTag={(id, tagId, assigned) => void setMessageTagById(id, tagId, assigned)}
                    onOpenInNewTab={openInNewTab}
                    onSaveAs={(m) => void saveMessageAs(m)}
                    onPrint={(m) => void printMessage(m)}
                    onAttachToNew={attachToNewMessage}
                    onDelete={requestDelete}
                    onDeletePermanent={(m) => setMessageToPurge(m)}
                    onCancelSend={(m) => setMessageToCancelSend(m)}
                />
            )}
            {accountToDelete && (
                <ConfirmDialog
                    title="Remove account"
                    message={`Remove ${accountToDelete.email}? Its cached mail is deleted from this device and its password is removed from the keychain. Mail on the server is not affected.`}
                    confirmLabel="Remove account"
                    busy={deleting}
                    onConfirm={() => void removeAccount()}
                    onCancel={() => setAccountToDelete(null)}
                />
            )}
            {folderPrompt && (
                <PromptDialog
                    title={folderPrompt.mode === 'create' ? 'New folder' : 'Rename folder'}
                    label={folderPrompt.mode === 'create' ? 'Folder name' : 'New name'}
                    initialValue={folderPrompt.mode === 'rename' ? folderPrompt.folder?.name : ''}
                    confirmLabel={folderPrompt.mode === 'create' ? 'Create' : 'Rename'}
                    busy={folderBusy}
                    onSubmit={(value) => void submitFolderPrompt(value)}
                    onCancel={() => setFolderPrompt(null)}
                />
            )}
            {folderToDelete && (
                <ConfirmDialog
                    title="Delete folder"
                    message={`Delete the folder "${folderToDelete.name}" on the server? Its messages are removed from this device. This cannot be undone.`}
                    confirmLabel="Delete folder"
                    busy={folderBusy}
                    onConfirm={() => void confirmDeleteFolder()}
                    onCancel={() => setFolderToDelete(null)}
                />
            )}
        </div>
    )
}

export default App
