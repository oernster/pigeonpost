import {useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState} from 'react'
import './App.css'
import brandIcon from './assets/pigeonpost.png'
import {AboutInfo, Account, api, CalendarEvent, Contact, DraftRecoveryResult, Folder, Message, MessageBody, OutboxItem, Rule, Tag, UnreadCountsResult} from './api'
import {OUTBOX_FOLDER_ID, isOutboxMessage, outboxItemToMessage} from './outbox'
import {applyTheme, loadTheme, Theme} from './theme'
import {TAG_PALETTE, colourTagId} from './tagColours'
import {Sidebar} from './components/Sidebar'
import {MessageList} from './components/MessageList'
import {MessageContextMenu} from './components/MessageContextMenu'
import {Reader} from './components/Reader'
import {Menu, MenuItem} from './components/Menu'
import {AboutModal} from './components/AboutModal'
import {LicenceModal} from './components/LicenceModal'
import {arrangeByConversation, sortByDate} from './threads'
import {ComposeInitial, ComposeModal} from './components/ComposeModal'
import {AccountSetupModal} from './components/AccountSetupModal'
import {ConfirmDialog} from './components/ConfirmDialog'
import {PromptDialog} from './components/PromptDialog'
import {RuleManagerModal} from './components/RuleManagerModal'
import {ContactsModal} from './components/ContactsModal'
import {CalendarModal} from './components/CalendarModal'
import {ReminderNotifications} from './components/ReminderNotifications'
import {CloseChoiceDialog} from './components/CloseChoiceDialog'
import {Splash} from './components/Splash'
import {useEscapeToClose} from './components/useBackdropDismiss'
import {EventsOn} from '../wailsjs/runtime'

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
        // Collapse the roving lists (messages, folders, accounts) to a single stop each: the row itself
        // is the stop, so skip the action buttons nested inside a row (Up/Down move within the list).
        const row = el.closest('.message-row, .list-item.folder, .list-item.account')
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

// matchesShortcut reports whether a keyboard event is the accelerator named by a shortcut string such as
// "Ctrl+N", "F9" or "Ctrl+Shift+K". Ctrl matches the Cmd key too, so the same strings work on macOS.
function matchesShortcut(e: KeyboardEvent, shortcut: string): boolean {
    const parts = shortcut.toLowerCase().split('+').map((part) => part.trim())
    const key = parts[parts.length - 1]
    return parts.includes('ctrl') === (e.ctrlKey || e.metaKey) &&
        parts.includes('shift') === e.shiftKey &&
        parts.includes('alt') === e.altKey &&
        e.key.toLowerCase() === key
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
    const [closeChoice, setCloseChoice] = useState(false)
    const [composing, setComposing] = useState<boolean>(false)
    const [composeInitial, setComposeInitial] = useState<ComposeInitial | undefined>(undefined)
    // recovery is a locally autosaved compose snapshot from a previous session, offered for restore once
    // accounts have loaded; recoveryCheckedRef makes that offer happen once per launch.
    const [recovery, setRecovery] = useState<DraftRecoveryResult | null>(null)
    const recoveryCheckedRef = useRef<boolean>(false)
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
    // calendarInitialEvent is the event whose dialog the calendar opens with, set when a reminder toast is
    // clicked so it lands on that event. Null for a normal calendar open from the menu.
    const [calendarInitialEvent, setCalendarInitialEvent] = useState<string | null>(null)
    const [messageBody, setMessageBody] = useState<MessageBody | null>(null)
    const [bodyLoading, setBodyLoading] = useState<boolean>(false)
    const [searchQuery, setSearchQuery] = useState<string>('')
    const [searchResults, setSearchResults] = useState<Message[]>([])
    const [messageToDelete, setMessageToDelete] = useState<Message | null>(null)
    const [deletingMessage, setDeletingMessage] = useState<boolean>(false)
    const [messageToPurge, setMessageToPurge] = useState<Message | null>(null)
    const [purgingMessage, setPurgingMessage] = useState<boolean>(false)
    const [contextMenu, setContextMenu] = useState<{message: Message; x: number; y: number} | null>(null)
    // markedIds is the multi-selection built by Ctrl and Shift clicking. Empty means single-select mode,
    // where selectedMessage alone is the selection. anchorId is the pivot a Shift-click ranges from.
    const [markedIds, setMarkedIds] = useState<Set<string>>(new Set())
    const [anchorId, setAnchorId] = useState<string | null>(null)
    const [bulkToDelete, setBulkToDelete] = useState<Message[] | null>(null)
    const [bulkDeleting, setBulkDeleting] = useState<boolean>(false)
    const [bulkToPurge, setBulkToPurge] = useState<Message[] | null>(null)
    const [bulkPurging, setBulkPurging] = useState<boolean>(false)
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
    // menuShortcutsRef holds the current menu items so the global accelerator handler always sees the
    // latest labels, enabled state and callbacks without re-binding its listener on every render.
    const menuShortcutsRef = useRef<MenuItem[]>([])
    useEffect(() => {
        neutralFocusRef.current?.focus()
    }, [])

    // Close the draft-recovery prompt on Escape, matching the other dialogs. It is a plain inline modal, so
    // it does not use the shared backdrop hook; the active flag registers it only while it is showing.
    useEscapeToClose(() => setRecovery(null), Boolean(recovery) && !composing)

    const searchActive = searchQuery.trim() !== ''
    // conversationView groups the folder's messages into conversations; it does not apply to search
    // results, which stay ranked by relevance. The choice is remembered across launches.
    const [conversationView, setConversationView] = useState<boolean>(() => localStorage.getItem('conversationView') === '1')
    const toggleConversationView = useCallback(() => {
        setConversationView((on) => {
            const next = !on
            localStorage.setItem('conversationView', next ? '1' : '0')
            return next
        })
    }, [])
    // displayMessages is the folder message list in the order the list renders it: conversation-grouped
    // when the view is on, otherwise as loaded. conversationHeads labels the first row of each multi-message
    // conversation. Both selection and keyboard navigation read displayMessages, so ranges and arrow keys
    // follow exactly what the user sees.
    // sortAscending flips the folder list between newest-first (default) and oldest-first, driven by the
    // Date column header. The choice is remembered across launches. It also sets the order conversations
    // are listed in when the conversation view is on.
    const [sortAscending, setSortAscending] = useState<boolean>(() => localStorage.getItem('sortAscending') === '1')
    const toggleSort = useCallback(() => {
        setSortAscending((asc) => {
            const next = !asc
            localStorage.setItem('sortAscending', next ? '1' : '0')
            return next
        })
    }, [])
    const {ordered: displayMessages, heads: conversationHeads} = useMemo(
        () => (conversationView && !searchActive
            ? arrangeByConversation(messages, sortAscending)
            : {ordered: sortByDate(messages, sortAscending), heads: new Map()}),
        [conversationView, searchActive, messages, sortAscending],
    )
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

    // reorderAccounts applies the new sidebar order optimistically (so the move is instant) and persists
    // it. On failure it shows the error and reloads the canonical order from the store, so a rejected
    // reorder does not leave the UI out of step with what is saved.
    const reorderAccounts = useCallback(
        async (orderedIds: string[]) => {
            const byId = new Map(accounts.map((a) => [a.id, a]))
            const next = orderedIds
                .map((id) => byId.get(id))
                .filter((a): a is Account => a !== undefined)
            setAccounts(next)
            try {
                await api.reorderAccounts(orderedIds)
            } catch (e) {
                setError(String(e))
                await loadAccounts()
            }
        },
        [accounts, loadAccounts],
    )

    // Once accounts have loaded, check for a compose snapshot autosaved in a previous session and offer to
    // restore it. This runs once per launch. A snapshot whose account has since been removed is stale, so
    // it is cleared silently rather than offered against an account that no longer exists.
    useEffect(() => {
        if (recoveryCheckedRef.current || accounts.length === 0) return
        recoveryCheckedRef.current = true
        void (async () => {
            try {
                const snapshot = await api.draftRecovery()
                if (!snapshot.present) return
                if (accounts.some((account) => account.id === snapshot.accountId)) {
                    setRecovery(snapshot)
                } else {
                    void api.clearDraftRecovery()
                }
            } catch {
                // A recovery check failure is non-fatal; the composer still works without it.
            }
        })()
    }, [accounts])

    // restoreDraft reopens the composer pre-filled from the autosaved snapshot, switching to its account
    // first so the message is sent from the identity it was written under. The composer's own autosave
    // then keeps the snapshot current, so it is not cleared here.
    const restoreDraft = () => {
        if (!recovery) return
        if (accounts.some((account) => account.id === recovery.accountId)) {
            setSelectedAccount(recovery.accountId)
        }
        setComposeInitial({
            to: recovery.to,
            cc: recovery.cc,
            bcc: recovery.bcc,
            subject: recovery.subject,
            bodyHtml: recovery.bodyHtml,
        })
        setComposing(true)
        setRecovery(null)
    }

    // discardDraft drops the autosaved snapshot when the user chooses not to restore it.
    const discardDraft = () => {
        void api.clearDraftRecovery()
        setRecovery(null)
    }

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

    // openReminderEvent opens the calendar with the reminder's event dialog on top, so a clicked reminder
    // shows what it is about. Events are refreshed first so the calendar can find and jump to the event.
    const openReminderEvent = useCallback((eventId: string) => {
        setCalendarInitialEvent(eventId)
        setManagingCalendar(true)
        void loadEvents()
    }, [loadEvents])

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
            setMessageBody({plain: item?.body ?? '', html: '', hasInvite: false, attachments: []})
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
        setMarkedIds(new Set())
        setAnchorId(null)
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
        setMarkedIds(new Set())
        setAnchorId(null)
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

    // clearSelection drops the multi-selection back to single-select mode. The active message (shown in
    // the reader) is left as it is, so clearing a selection does not close the message on screen.
    const clearSelection = useCallback(() => {
        setMarkedIds(new Set())
        setAnchorId(null)
    }, [])

    // activateRow applies the standard list-selection gestures to a row click. A plain click selects the
    // one row and opens it; Ctrl (or Cmd) click toggles the row in or out of the selection; Shift click
    // selects the contiguous range from the anchor. The clicked row always becomes the active one shown in
    // the reader, and a Shift range keeps the existing anchor so successive Shift clicks re-range from it.
    const activateRow = useCallback((message: Message, mods: {ctrl: boolean; shift: boolean}) => {
        const list = searchActive ? searchResults : displayMessages
        if (mods.shift && anchorId) {
            const from = list.findIndex((m) => m.id === anchorId)
            const to = list.findIndex((m) => m.id === message.id)
            if (from !== -1 && to !== -1) {
                const [lo, hi] = from <= to ? [from, to] : [to, from]
                setMarkedIds(new Set(list.slice(lo, hi + 1).map((m) => m.id)))
            } else {
                setMarkedIds(new Set([message.id]))
            }
            setSelectedMessage(message)
            setReadingFull(false)
            return
        }
        if (mods.ctrl) {
            setMarkedIds((prev) => {
                const base = prev.size ? new Set(prev) : new Set<string>(selectedMessage ? [selectedMessage.id] : [])
                if (base.has(message.id)) {
                    base.delete(message.id)
                } else {
                    base.add(message.id)
                }
                return base
            })
            setAnchorId(message.id)
            setSelectedMessage(message)
            setReadingFull(false)
            return
        }
        setMarkedIds(new Set())
        setAnchorId(message.id)
        selectMessage(message)
    }, [searchActive, searchResults, displayMessages, anchorId, selectedMessage, selectMessage])

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

    // openContextMenu opens the action menu at the cursor without selecting the message. A right-click is
    // not "reading" the message, so it is not shown in the reader and not auto-marked read, which would
    // otherwise leave the menu's read/unread state (and the message) wrong. Every menu action receives
    // the message directly, so it does not need to be the selected one.
    const openContextMenu = useCallback((message: Message, x: number, y: number) => {
        // Right-clicking a row that is not part of a multi-selection collapses the selection to that row, so
        // the menu acts on what was clicked rather than on an unrelated set. Right-clicking within the
        // selection keeps it, so its bulk actions apply to the whole set.
        setMarkedIds((prev) => (prev.size > 1 && !prev.has(message.id) ? new Set() : prev))
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

    // signatureHtml is the selected account's signature as HTML, inserted into a new message and above the
    // quoted text on a reply or forward. Empty when the account has no signature, so nothing is added.
    const signatureHtml = (): string => accounts.find((a) => a.id === selectedAccount)?.signature ?? ''

    // sendersFor returns the addresses an account may send from: its primary address first, then its
    // identities. The compose window offers these in its From dropdown.
    const sendersFor = (account?: Account): {name: string; address: string}[] =>
        account ? [{name: account.displayName, address: account.email}, ...account.identities] : []

    // replyFrom picks which of the account's own addresses a reply should be sent from: the one the
    // original message was delivered to (its To or Cc), so a message to an alias is answered as that
    // alias. It returns empty (the primary) when none of the account's addresses received it.
    const replyFrom = (message: Message): string => {
        const mine = new Set(sendersFor(accounts.find((a) => a.id === selectedAccount)).map((s) => s.address.toLowerCase()))
        const hit = [...(message.to || []), ...(message.cc || [])].find((a) => mine.has(a.address.toLowerCase()))
        return hit ? hit.address : ''
    }

    const openReply = (message: Message) => {
        const when = message.date ? new Date(message.date).toLocaleString() : ''
        const who = message.fromName || message.fromAddress || 'the sender'
        const header = when ? `On ${when}, ${who} wrote:` : `${who} wrote:`
        setComposeInitial({
            from: replyFrom(message),
            to: message.fromAddress,
            subject: subjectWithPrefix('Re:', message.subject),
            bodyHtml: `<p></p>${signatureHtml()}<p>${escapeHtml(header)}</p><blockquote>${quoteFor(message)}</blockquote>`,
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
            from: replyFrom(message),
            to: toList.join(', '),
            cc: ccList.join(', '),
            subject: subjectWithPrefix('Re:', message.subject),
            bodyHtml: `<p></p>${signatureHtml()}<p>${escapeHtml(header)}</p><blockquote>${quoteFor(message)}</blockquote>`,
        })
        setComposing(true)
    }

    const openForward = (message: Message) => {
        const who = message.fromName || message.fromAddress || 'unknown sender'
        setComposeInitial({
            to: '',
            subject: subjectWithPrefix('Fwd:', message.subject),
            bodyHtml:
                `<p></p>${signatureHtml()}<p>---------- Forwarded message ----------</p>` +
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
            bodyHtml: signatureHtml() ? `<p></p>${signatureHtml()}` : undefined,
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

    // The Windows tray context menu mirrors the Help menu: its items emit these events from the backend,
    // which open the same dialogs the in-window Help menu does. The close button emits close-request so
    // the choice of minimise-to-tray or quit uses the app's own themed dialog rather than a native one.
    useEffect(() => {
        const off = [
            EventsOn('menu:about', () => void showAbout()),
            EventsOn('menu:licence', () => void showLicence()),
            EventsOn('menu:check-updates', () => checkUpdates()),
            EventsOn('app:close-request', () => setCloseChoice(true)),
        ]
        return () => off.forEach((unsubscribe) => unsubscribe())
    }, [showAbout, showLicence, checkUpdates])

    // The background mail poller emits mail:new when it brings in newly arrived messages (the same event
    // that raises the desktop notification), so refresh the unread counts and the folder on screen to show
    // the arrivals without waiting for the next on-screen sync.
    useEffect(() => {
        const off = EventsOn('mail:new', () => {
            void loadUnread()
            if (selectedFolder) {
                void api.listMessages(selectedFolder).then(setMessages).catch(() => undefined)
            }
        })
        return () => off()
    }, [selectedFolder, loadUnread])

    // The poller emits calendar:changed after auto-applying an incoming meeting reply or cancellation, so
    // reload the events for the calendar view to reflect the updated attendee status or removed meeting.
    useEffect(() => {
        const off = EventsOn('calendar:changed', () => void loadEvents())
        return () => off()
    }, [loadEvents])

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

    // removeIdsFromLists drops a set of message ids from every on-screen list and the selection after a
    // bulk delete or move, and clears the active message if it was among them. All the setters are stable,
    // so it needs no dependencies.
    const removeIdsFromLists = useCallback((ids: Set<string>) => {
        setMessages((prev) => prev.filter((m) => !ids.has(m.id)))
        setSearchResults((prev) => prev.filter((m) => !ids.has(m.id)))
        setTabs((prev) => prev.filter((m) => !ids.has(m.id)))
        setSelectedMessage((prev) => (prev && ids.has(prev.id) ? null : prev))
        setMarkedIds(new Set())
        setAnchorId(null)
    }, [])

    // bulkMoveIds moves several messages into a folder in ONE batched backend call (grouped by source
    // folder on the server), rather than a request per message, so a large Gmail selection stays under
    // its simultaneous-connection cap. Shared by drag-and-drop and the bulk "Move to" menu.
    const bulkMoveIds = useCallback(async (ids: string[], destFolderId: string) => {
        if (ids.length === 0 || destFolderId === OUTBOX_FOLDER_ID) {
            return
        }
        setError('')
        try {
            const result = await api.moveMessages(ids, destFolderId)
            removeIdsFromLists(new Set(result.ids))
            if (result.error) {
                setError(`${result.failed} of ${ids.length} messages could not be moved: ${result.error}`)
            }
        } catch (e) {
            setError(`Move failed: ${String(e)}`)
        }
        await loadUnread()
        await refreshFolders()
    }, [removeIdsFromLists, loadUnread, refreshFolders])

    // dropMessageOnFolder is the drag-and-drop target handler. Dropping a row that is part of the
    // multi-selection moves the whole selection; dropping any other row moves just that one. Messages
    // already in the target folder and synthetic outbox items are skipped. The move is batched, so a
    // large drop stays under Gmail's connection cap.
    const dropMessageOnFolder = useCallback((messageId: string, folderId: string) => {
        if (folderId === OUTBOX_FOLDER_ID) {
            return
        }
        const ids = markedIds.has(messageId) && markedIds.size > 1 ? [...markedIds] : [messageId]
        const movable = ids.filter((id) => {
            const source = messages.find((m) => m.id === id) ?? searchResults.find((m) => m.id === id)
            return source !== undefined && source.folderId !== folderId && !isOutboxMessage(source)
        })
        setMarkedIds(new Set())
        setAnchorId(null)
        void bulkMoveIds(movable, folderId)
    }, [markedIds, messages, searchResults, bulkMoveIds])

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
        try {
            const result = permanent
                ? await api.deleteMessagesPermanent(ids)
                : await api.deleteMessages(ids)
            removeIdsFromLists(new Set(result.ids))
            if (result.error) {
                setError(`${result.failed} of ${targets.length} messages could not be deleted: ${result.error}`)
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
    }, [removeIdsFromLists, loadUnread, refreshFolders])

    // bulkSetRead sets the read flag on every selected message, updating the lists at once and then
    // persisting each. bulkSetFlag does the same for the star. Both take an explicit value rather than
    // toggling, so a mixed selection ends up uniform.
    const bulkSetRead = useCallback(async (targets: Message[], read: boolean) => {
        const ids = new Set(targets.map((t) => t.id))
        const apply = (m: Message): Message => (ids.has(m.id) ? {...m, read} : m)
        setMessages((prev) => prev.map(apply))
        setSearchResults((prev) => prev.map(apply))
        setTabs((prev) => prev.map(apply))
        setSelectedMessage((prev) => (prev && ids.has(prev.id) ? {...prev, read} : prev))
        let failed = 0
        for (const t of targets) {
            try {
                await api.markRead(t.id, read)
            } catch {
                failed += 1
            }
        }
        try {
            await loadUnread()
        } catch {
            // A count refresh is best effort; the optimistic list update already reflects the change.
        }
        if (failed > 0) {
            setError(`${failed} of ${targets.length} messages could not be updated on the server.`)
        }
    }, [loadUnread])

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
        if (failed > 0) {
            setError(`${failed} of ${targets.length} messages could not be updated on the server.`)
        }
    }, [])

    // bulkMove moves every selected message into the destination folder in one batched call, skipping any
    // already there and any synthetic outbox item.
    const bulkMove = useCallback((targets: Message[], destFolderId: string) => {
        const ids = targets
            .filter((t) => t.folderId !== destFolderId && !isOutboxMessage(t))
            .map((t) => t.id)
        void bulkMoveIds(ids, destFolderId)
    }, [bulkMoveIds])

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
    // counts as read, so viewing or double-clicking a message un-bolds it. Auto-read fires once per
    // selection, keyed by message id: without this guard, explicitly marking the open message unread
    // re-runs this effect (its read flag changed) and immediately re-reads it. The id is unchanged on
    // that re-run, so nothing happens; selecting a different message later reads it as expected.
    const autoReadIdRef = useRef<string | null>(null)
    useEffect(() => {
        if (!(previewEnabled || readingFull) || !selectedMessage) {
            return
        }
        if (autoReadIdRef.current === selectedMessage.id) {
            return
        }
        autoReadIdRef.current = selectedMessage.id
        if (!selectedMessage.read) {
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

            if (isText) {
                return
            }
            // The neutral start sink owns the very first Tab (its onKeyDown enters the ring). This is the
            // fallback for when focus is instead on the body: route Tab into the ring when focus is outside
            // it. A context menu owns its own keys, so it is the only overlay that blocks this; the splash
            // does not, so the first Tab on launch still enters the ring. Once focus is on a real control,
            // native Tab moves between elements as usual.
            if (e.key === 'Tab') {
                if (contextMenu) {
                    return
                }
                const ring = focusRingElements(focusRingRoot())
                if (ring.length > 0 && ring.indexOf(document.activeElement as HTMLElement) === -1) {
                    e.preventDefault()
                    stepFocusRing(e.shiftKey ? -1 : 1)
                }
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
                // Keep DOM focus on the row the cursor moves to when navigating within the list, so the
                // focus ring's current stop stays in step and a following Right/Left steps out correctly.
                // Only when a message row already holds focus, so an arrow pressed elsewhere in the window
                // still moves the selection without stealing focus.
                const focusedRow = document.activeElement as HTMLElement | null
                if (focusedRow && focusedRow.classList.contains('message-row')) {
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
            if (e.key === 'Delete') {
                // Delete acts on the whole selection: the Ctrl/Shift set if there is one, else the active
                // message. One target uses the single confirm; several use the count confirm.
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
        contextMenu, messageToCancelSend, bulkToDelete, bulkToPurge, togglePreview,
    ])

    // Menu accelerators (Compose, Sync, the reading pane and any others defined on the menus) fire from
    // anywhere in the main window, driven by the same item definitions the menus render so an item's hint
    // and its wired key never drift. They are suppressed while a dialog or the context menu is open, so a
    // shortcut never acts behind one. A disabled item (Compose with no account selected, say) is skipped.
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (document.querySelector('.modal, .context-menu') !== null) {
                return
            }
            for (const item of menuShortcutsRef.current) {
                if (item.shortcut && !item.disabled && item.onClick && matchesShortcut(e, item.shortcut)) {
                    e.preventDefault()
                    item.onClick()
                    return
                }
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [])

    // A POP3 account has a single downloaded inbox with no server-side folders, message moves or draft
    // mailbox, so those actions are hidden and a delete is permanent rather than a move to Trash.
    const activeAccount = accounts.find((a) => a.id === selectedAccount)
    const isPop3 = activeAccount?.protocol === 'pop3'

    // Derived selection: the visible list, the set of highlighted rows (the Ctrl/Shift selection, or just
    // the active message when there is none), the messages any bulk action operates on, and whether more
    // than one is selected. menuSelection is what a right-click menu acts on: the whole set when the
    // clicked row is within a multi-selection, otherwise just that row.
    const visibleList = searchActive ? searchResults : displayMessages
    const selectionIds = markedIds.size
        ? markedIds
        : (selectedMessage ? new Set<string>([selectedMessage.id]) : new Set<string>())
    const selectedMessages = visibleList.filter((m) => selectionIds.has(m.id))
    const multiSelected = markedIds.size > 1
    const menuSelection = contextMenu
        ? (markedIds.size > 1 && markedIds.has(contextMenu.message.id) ? selectedMessages : [contextMenu.message])
        : []

    // The message list and reader are extracted so the reading-pane layout can place them side by side
    // (pane on) or swap between them (pane off: list, or the full-width reader when a message is opened).
    const messageListEl = (
        <MessageList
            messages={visibleList}
            conversationHeads={conversationHeads}
            sortAscending={sortAscending}
            onToggleSort={toggleSort}
            selectedIds={selectionIds}
            activeId={selectedMessage?.id ?? null}
            folderSelected={Boolean(selectedFolder)}
            searchQuery={searchQuery}
            searchActive={searchActive}
            onSearchChange={setSearchQuery}
            onActivate={activateRow}
            onClearSelection={clearSelection}
            onToggleFlag={(m) => void toggleFlag(m)}
            onContextMenu={openContextMenu}
            onOpenInNewTab={openInNewTab}
        />
    )
    const readerEl = multiSelected ? (
        <section className="pane reader">
            <div className="empty-state selection-summary">
                <p className="empty-body">{markedIds.size} messages selected</p>
                <div className="selection-actions">
                    <button className="btn danger" onClick={() => setBulkToDelete(selectedMessages)}>Delete</button>
                    <button className="btn" onClick={() => void bulkSetRead(selectedMessages, true)}>Mark read</button>
                    <button className="btn" onClick={() => void bulkSetRead(selectedMessages, false)}>Mark unread</button>
                    <button className="btn" onClick={clearSelection}>Clear selection</button>
                </div>
            </div>
        </section>
    ) : (
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

    // The title-bar menus are defined here so one item list drives both the dropdown and the global
    // accelerator handler above, keeping each item's shortcut hint and its wired key in step. The Mail menu
    // mirrors the right-click actions; both act on the active message.
    const activeMessage = selectedMessage
    const activeOutbox = activeMessage ? isOutboxMessage(activeMessage) : false
    // canMailAct gates the actions that need a real, non-outbox message on screen (reply, mark, move and
    // the rest). A queued outbox item only supports Cancel send.
    const canMailAct = Boolean(activeMessage) && !activeOutbox
    const canReplyAll = canMailAct && activeMessage
        ? ((activeMessage.to?.length ?? 0) + (activeMessage.cc?.length ?? 0)) > 0
        : false
    const mailMoveTargets = activeMessage ? folders.filter((f) => f.id !== activeMessage.folderId) : []
    const appliedTagIds = new Set(messageTags.map((t) => t.id))
    const fileMenu: MenuItem[] = [
        {
            label: syncing ? 'Syncing…' : 'Sync',
            icon: '\u{267B}\u{FE0F}',
            shortcut: 'F9',
            disabled: !selectedAccount || syncing,
            onClick: () => void sync(),
        },
        {
            label: 'Add account',
            icon: '\u{2795}',
            onClick: () => setSettingUp(true),
        },
        {label: '', separator: true},
        {
            label: 'Save as...',
            disabled: !canMailAct,
            onClick: () => activeMessage && void saveMessageAs(activeMessage),
        },
        {
            label: 'Print...',
            disabled: !canMailAct,
            onClick: () => activeMessage && void printMessage(activeMessage),
        },
    ]
    const editMenu: MenuItem[] = [
        {
            label: 'Compose',
            icon: '\u{1F58A}\u{FE0F}',
            shortcut: 'Ctrl+N',
            disabled: !selectedAccount,
            onClick: () => {
                const sig = signatureHtml()
                setComposeInitial(sig ? {bodyHtml: `<p></p>${sig}`} : undefined)
                setComposing(true)
            },
        },
        {
            label: 'Rules',
            icon: '\u{1F4CF}',
            onClick: () => setManagingRules(true),
        },
    ]
    const viewMenu: MenuItem[] = [
        {
            label: 'Conversation view',
            checked: conversationView,
            onClick: toggleConversationView,
        },
        {
            label: 'Reading pane',
            shortcut: 'F8',
            checked: previewEnabled,
            onClick: togglePreview,
        },
    ]
    const mailMenu: MenuItem[] = [
        {label: 'Open in new tab', disabled: !canMailAct, onClick: () => activeMessage && openInNewTab(activeMessage)},
        {label: '', separator: true},
        {label: 'Reply', disabled: !canMailAct, onClick: () => activeMessage && openReply(activeMessage)},
        {label: 'Reply all', disabled: !canReplyAll, onClick: () => activeMessage && openReplyAll(activeMessage)},
        {label: 'Forward', disabled: !canMailAct, onClick: () => activeMessage && openForward(activeMessage)},
        {
            label: 'Attach to new message',
            disabled: !canMailAct,
            onClick: () => activeMessage && attachToNewMessage(activeMessage),
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
        {label: '', separator: true},
        {
            label: 'Move to',
            disabled: !canMailAct || isPop3 || mailMoveTargets.length === 0,
            submenu: mailMoveTargets.map((f) => ({
                label: f.name,
                onClick: () => activeMessage && void moveMessage(activeMessage, f.id),
            })),
        },
        {
            label: 'Copy to',
            disabled: !canMailAct || isPop3 || mailMoveTargets.length === 0,
            submenu: mailMoveTargets.map((f) => ({
                label: f.name,
                onClick: () => activeMessage && void copyMessage(activeMessage, f.id),
            })),
        },
        {
            label: 'Mark as junk',
            disabled: !canMailAct || isPop3,
            onClick: () => activeMessage && void markJunk(activeMessage),
        },
        {label: '', separator: true},
        {
            label: 'Cancel send',
            disabled: !activeOutbox,
            onClick: () => activeMessage && setMessageToCancelSend(activeMessage),
        },
        {label: 'Delete', disabled: !canMailAct, onClick: () => activeMessage && requestDelete(activeMessage)},
        {
            label: 'Delete permanently',
            disabled: !canMailAct,
            onClick: () => activeMessage && setMessageToPurge(activeMessage),
        },
    ]
    const helpMenu: MenuItem[] = [
        {label: 'About PigeonPost', onClick: () => void showAbout()},
        {label: 'Licence', onClick: () => void showLicence()},
        {label: 'Check for Updates', onClick: checkUpdates},
    ]
    menuShortcutsRef.current = [...fileMenu, ...editMenu, ...viewMenu, ...mailMenu, ...helpMenu]

    return (
        <div className="app">
            <span
                ref={neutralFocusRef}
                tabIndex={-1}
                aria-hidden="true"
                style={{position: 'absolute', width: 0, height: 0, overflow: 'hidden', outline: 'none'}}
                onKeyDown={(e) => {
                    // Neutral start: the first Tab (or Shift+Tab) from this sink enters the focus ring, the
                    // way the keeb reference has the sink own its own Tab. Owning it here (rather than the
                    // window handler) means it works on launch even while the splash is still up.
                    if (e.key !== 'Tab') {
                        return
                    }
                    const items = focusRingElements(focusRingRoot())
                    if (items.length === 0) {
                        return
                    }
                    e.preventDefault()
                    e.stopPropagation()
                    ;(e.shiftKey ? items[items.length - 1] : items[0]).focus()
                }}
            />
            {splashVisible && <Splash version={appVersion} author={appAuthor} fading={splashFading}/>}
            <ReminderNotifications onOpen={openReminderEvent}/>
            <header className="titlebar">
                <div className="titlebar-left">
                    <span className="brand">
                        PigeonPost
                        {unreadCounts.total > 0 && (
                            <span className="titlebar-unread" title={`${unreadCounts.total} unread across all accounts`}>
                                {unreadCounts.total}
                            </span>
                        )}
                    </span>
                    <Menu title="File" icon={'\u{1F4C1}'} items={fileMenu} align="left"/>
                    <Menu title="Edit" icon={'\u{270F}\u{FE0F}'} items={editMenu} align="left"/>
                    <Menu title="View" icon={'\u{1F441}\u{FE0F}'} items={viewMenu} align="left"/>
                    <Menu title="Mail" icon={'\u{1F4EC}'} items={mailMenu} align="left"/>
                </div>
                <div className="titlebar-right">
                    <button className="sync-btn" onClick={() => setManagingContacts(true)}>
                        {'\u{1F4C7}'} Contacts
                    </button>
                    <button className="sync-btn" onClick={() => setManagingCalendar(true)}>
                        {'\u{1F4C5}'} Calendar
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
                    <Menu title="Help" icon={'\u{2139}\u{FE0F}'} items={helpMenu}/>
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
                    onReorderAccounts={(ids) => void reorderAccounts(ids)}
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
            {closeChoice && (
                <CloseChoiceDialog
                    onMinimise={() => {
                        setCloseChoice(false)
                        void api.minimiseToTray()
                    }}
                    onQuit={() => {
                        setCloseChoice(false)
                        void api.requestQuit()
                    }}
                    onCancel={() => setCloseChoice(false)}
                />
            )}
            {composing && selectedAccount && (
                <ComposeModal
                    accountId={selectedAccount}
                    senders={sendersFor(activeAccount)}
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
            {recovery && !composing && (
                <div className="modal-backdrop" onClick={() => setRecovery(null)}>
                    <div className="modal confirm" role="alertdialog" aria-label="Restore unsent message"
                         onClick={(e) => e.stopPropagation()}>
                        <h2 className="modal-title">Restore unsent message?</h2>
                        <p className="confirm-message">
                            An unsent message{recovery.subject.trim() ? ` "${recovery.subject.trim()}"` : ''} was
                            left open when PigeonPost last closed. Restore it to keep writing, or discard it.
                        </p>
                        <div className="modal-actions spread">
                            <button className="btn danger" onClick={discardDraft}>Discard</button>
                            <button className="btn primary" onClick={restoreDraft} autoFocus>Restore</button>
                        </div>
                    </div>
                </div>
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
                    accountId={selectedAccount}
                    accountEmail={activeAccount?.email ?? ''}
                    accountName={activeAccount?.displayName ?? ''}
                    initialEventId={calendarInitialEvent ?? undefined}
                    onChanged={() => void loadEvents()}
                    onClose={() => {
                        setManagingCalendar(false)
                        setCalendarInitialEvent(null)
                    }}
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
                    defaultConfirm
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
                    defaultConfirm
                    onConfirm={() => void deletePermanent()}
                    onCancel={() => setMessageToPurge(null)}
                />
            )}
            {bulkToDelete && (
                <ConfirmDialog
                    title="Delete messages"
                    message={isPop3
                        ? `Delete ${bulkToDelete.length} messages? POP3 has no Trash, so they are permanently removed from the server and cannot be recovered.`
                        : `Delete ${bulkToDelete.length} messages? They are moved to Trash, or deleted permanently where the account has no Trash folder.`}
                    confirmLabel={`Delete ${bulkToDelete.length}`}
                    busy={bulkDeleting}
                    defaultConfirm
                    onConfirm={() => void runBulkDelete(bulkToDelete, false)}
                    onCancel={() => setBulkToDelete(null)}
                />
            )}
            {bulkToPurge && (
                <ConfirmDialog
                    title="Delete permanently"
                    message={`Permanently delete ${bulkToPurge.length} messages? They are removed from the server and cannot be recovered.`}
                    confirmLabel={`Delete ${bulkToPurge.length} permanently`}
                    busy={bulkPurging}
                    defaultConfirm
                    onConfirm={() => void runBulkDelete(bulkToPurge, true)}
                    onCancel={() => setBulkToPurge(null)}
                />
            )}
            {contextMenu && (
                <MessageContextMenu
                    message={contextMenu.message}
                    x={contextMenu.x}
                    y={contextMenu.y}
                    folders={folders}
                    tags={tags}
                    selection={menuSelection}
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
                    onMarkJunk={(m) => void markJunk(m)}
                    onDelete={requestDelete}
                    onDeletePermanent={(m) => setMessageToPurge(m)}
                    onCancelSend={(m) => setMessageToCancelSend(m)}
                    onBulkSetRead={(msgs, read) => void bulkSetRead(msgs, read)}
                    onBulkSetFlag={(msgs, flagged) => void bulkSetFlag(msgs, flagged)}
                    onBulkMove={(msgs, dest) => void bulkMove(msgs, dest)}
                    onBulkDelete={(msgs) => setBulkToDelete(msgs)}
                    onBulkDeletePermanent={(msgs) => setBulkToPurge(msgs)}
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
