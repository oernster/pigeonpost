import {useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties} from 'react'
import './App.css'
import {AboutInfo, api, CalendarEvent, Contact, Folder, Message, MessageBody, Rule, Template, UnreadCountsResult} from './api'
import {OUTBOX_FOLDER_ID, isOutboxMessage, outboxItemToMessage} from './outbox'
import {UNIFIED_FOLDER_ID, accountChips, isUnifiedFolder} from './unified'
import {SNOOZED_FOLDER_ID, isSnoozedFolder} from './snooze'
import {applyTheme, loadTheme, Theme} from './theme'
import {Sidebar} from './components/Sidebar'
import {ErrorBar} from './components/ErrorBar'
import {MessageList, type SearchScope} from './components/MessageList'
import {MessageContextMenu} from './components/MessageContextMenu'
import {FolderContextMenu} from './components/FolderContextMenu'
import {Reader} from './components/Reader'
import {EmailViewerModal} from './components/EmailViewerModal'
import {ModalClose} from './components/ModalClose'
import {useBackdropDismiss} from './components/useBackdropDismiss'
import {TitleBar} from './components/TitleBar'
import {BottomBar} from './components/BottomBar'
import {WelcomeScreen} from './components/WelcomeScreen'
import {SelectionSummary} from './components/SelectionSummary'
import {DraftRecoveryDialog} from './components/DraftRecoveryDialog'
import {AboutModal} from './components/AboutModal'
import {UpdateModal} from './components/UpdateModal'
import {LicenceModal} from './components/LicenceModal'
import {arrangeByConversation, sortByDate} from './threads'
import {isJunkFolderMessage} from './folderPaths'
import {ComposeModal, type ComposeInitial} from './components/ComposeModal'
import {UndoSendToast} from './components/UndoSendToast'
import {EventsOn} from '../wailsjs/runtime'
import {MessagePickerDialog} from './components/MessagePickerDialog'
import {AccountSetupModal} from './components/AccountSetupModal'
import {ConfirmDialog} from './components/ConfirmDialog'
import {PromptDialog} from './components/PromptDialog'
import {RuleManagerModal} from './components/RuleManagerModal'
import {TemplateManagerModal} from './components/TemplateManagerModal'
import {ContactsModal} from './components/ContactsModal'
import {CalendarModal} from './components/CalendarModal'
import {ReminderNotifications} from './components/ReminderNotifications'
import {CloseChoiceDialog} from './components/CloseChoiceDialog'
import {Splash} from './components/Splash'
import {PaneSplitters} from './components/PaneSplitters'
import {sendersFor} from './replyDraft'
import {focusRingElements, focusRingRoot} from './focusRing'
import {useMessageStore} from './hooks/useMessageStore'
import {useMessageExport} from './hooks/useMessageExport'
import {useManagedCollection} from './hooks/useManagedCollection'
import {useUndoSend} from './hooks/useUndoSend'
import {rangeIds, toggleId, useSelection} from './hooks/useSelection'
import {useMessageActions} from './hooks/useMessageActions'
import {useBulkActions} from './hooks/useBulkActions'
import {useReaderTabs} from './hooks/useReaderTabs'
import {useConversation} from './hooks/useConversation'
import {ThreadView} from './components/ThreadView'
import {useOutbox} from './hooks/useOutbox'
import {FolderPrompt, useFolders} from './hooks/useFolders'
import {useAccounts} from './hooks/useAccounts'
import {useSync} from './hooks/useSync'
import {useTags} from './hooks/useTags'
import {useComposeLauncher} from './hooks/useComposeLauncher'
import {useAppEvents} from './hooks/useAppEvents'
import {useUpdateCheck} from './hooks/useUpdateCheck'
import {defaultUndoSendSeconds, undoSendChoices, useMenus} from './hooks/useMenus'
import {useMessageListKeyboard} from './hooks/useMessageListKeyboard'
import {usePaneWidths} from './hooks/usePaneWidths'
import {useFolderPagination} from './hooks/useFolderPagination'
import {isTypingTarget, useIdleRefocus} from './hooks/useIdleRefocus'
import {useSnooze} from './hooks/useSnooze'
import {useUndoRedo} from './hooks/useUndoRedo'
import {useEditContext} from './hooks/useEditContext'
import {useMessageClipboard} from './hooks/useMessageClipboard'
import {canCopy, canCut, canPaste, copySelection, cutSelection, pasteText} from './editClipboard'
import {ScheduleDialog} from './components/ScheduleDialog'
import {
    elsewhereCue, ensureWatermarks, loadWatermarks, saveWatermarks, touchWatermarks,
    type Watermarks,
} from './newMail'

// SELECTED_ACCOUNT_KEY holds the id of the account that was open when the app last closed, so a restart
// reopens it rather than always falling back to the first account.
const SELECTED_ACCOUNT_KEY = 'selectedAccount'

// folderPromptTitle names the folder dialog for what it is about to do: a create with a parent is a
// subfolder of that parent, a create without one lands at the top level.
function folderPromptTitle(prompt: FolderPrompt): string {
    if (prompt.mode === 'rename') {
        return 'Rename folder'
    }
    return prompt.parent ? `New folder in ${prompt.parent.name}` : 'New folder'
}

function App() {
    const [selectedAccount, setSelectedAccount] = useState<string>('')
    const [unreadCounts, setUnreadCounts] = useState<UnreadCountsResult>(
        {total: 0, byAccount: {}, newestByAccount: {}},
    )
    // watermarks records, per account, the last instant it was the selected account. The elsewhere
    // cue on the account picker lights only for unread mail newer than an account's watermark, so a
    // standing backlog never lights it; only arrivals since you last looked do.
    const [watermarks, setWatermarks] = useState<Watermarks>(() => loadWatermarks(localStorage))
    // The coupled core of the mail views (the folder list, the search results, the reader tabs and the
    // active message) lives in one hook, so an action updates a message wherever it appears. The three
    // lists are never split apart.
    const store = useMessageStore()
    const {
        messages, setMessages,
        searchResults, setSearchResults,
        tabs, setTabs,
        selectedMessage, setSelectedMessage,
    } = store
    // messagesRef mirrors the loaded folder list so loadMoreMessages can read the current ids (to skip
    // duplicates) without being rebuilt whenever the list changes.
    const messagesRef = useRef<Message[]>(messages)
    messagesRef.current = messages
    // Flat-view keyset pagination: the folder listing loads one page at a time so a huge folder (a real
    // Trash of tens of thousands of messages) opens without loading every row at once. Conversation view
    // and search load the whole set instead (threading and search each need it), bounded by the virtualised
    // list so the render does not freeze.
    const pagination = useFolderPagination()
    const [error, setError] = useState<string>('')
    // The folder list, the selected folder, the folder create/rename/delete/reparent flow and the
    // selected-folder ref live in useFolders. loadFolderMessages and selectFolder stay in App (below): they
    // coordinate folder navigation with the outbox view; selectFolder's Outbox branch reads the queue.
    const {
        folders, setFolders, selectedFolder, setSelectedFolder, selectedFolderRef,
        selectedAccountRef, applyFolders,
        folderPrompt, setFolderPrompt, folderToDelete, setFolderToDelete, folderBusy,
        refreshFolders, submitFolderPrompt, confirmDeleteFolder, reparentFolder,
    } = useFolders({selectedAccount, store, setError})
    // The outbox queue (surfaced as a per-account synthetic Outbox folder), the cancel-send confirm flow
    // and the folder list including that synthetic folder live in useOutbox. The effect that keeps the open
    // Outbox view in step with the queue stays in App below, because it drives folder navigation.
    const {
        outbox, outboxForAccount, refreshOutbox, sidebarFolders,
        messageToCancelSend, setMessageToCancelSend, cancellingSend, cancelSend,
    } = useOutbox({selectedAccount, folders, setError})
    // The account list, the add/edit/remove dialog state and the load/remove operations live in
    // useAccounts. selectAccount and the auto-select effect stay in App (below): account selection cascades
    // into the folders, the store, the selection and the reader; it also needs loadFolderMessages.
    const {
        accounts, settingUp, setSettingUp, accountToEdit, setAccountToEdit,
        accountToDelete, setAccountToDelete, deleting,
        loadAccounts, removeAccount,
    } = useAccounts({selectedAccount, setSelectedAccount, store, setFolders, setSelectedFolder, setError})
    const [theme, setTheme] = useState<Theme>(loadTheme())
    const [about, setAbout] = useState<AboutInfo | null>(null)
    const [licence, setLicence] = useState<string | null>(null)
    // The four backend-backed collections the menus manage share one shape (load on mount, reload after
    // the manager dialog changes them, a flag for whether that dialog is open), so they share one hook.
    const {items: rules, reload: loadRules, managing: managingRules, setManaging: setManagingRules} =
        useManagedCollection<Rule>(api.listRules, setError)
    const {items: templates, reload: loadTemplates, managing: managingTemplates, setManaging: setManagingTemplates} =
        useManagedCollection<Template>(api.listTemplates, setError)
    const {items: contacts, reload: loadContacts, managing: managingContacts, setManaging: setManagingContacts} =
        useManagedCollection<Contact>(api.listContacts, setError)
    const {items: events, reload: loadEvents, managing: managingCalendar, setManaging: setManagingCalendar} =
        useManagedCollection<CalendarEvent>(api.listEvents, setError)
    // calendarInitialEvent is the event whose dialog the calendar opens with, set when a reminder toast is
    // clicked so it lands on that event. Null for a normal calendar open from the menu.
    const [calendarInitialEvent, setCalendarInitialEvent] = useState<string | null>(null)
    const [messageBody, setMessageBody] = useState<MessageBody | null>(null)
    const [bodyLoading, setBodyLoading] = useState<boolean>(false)
    const [searchQuery, setSearchQuery] = useState<string>('')
    // The search scope selector, the per-hit matched-text snippets (keyed by message id) and whether the
    // last query degraded to plain text (its operators could not be parsed).
    const [searchScope, setSearchScope] = useState<SearchScope>('all')
    const [searchSnippets, setSearchSnippets] = useState<Map<string, string>>(new Map())
    const [searchDegraded, setSearchDegraded] = useState<boolean>(false)
    // searchInputRef lets Edit > Search (Ctrl+K) focus the search box from anywhere.
    const searchInputRef = useRef<HTMLInputElement>(null)
    const focusSearch = useCallback(() => searchInputRef.current?.focus(), [])
    const [contextMenu, setContextMenu] = useState<{message: Message; x: number; y: number} | null>(null)
    // The multi-selection built by Ctrl and Shift gestures (the marked ids and the Shift-range anchor)
    // lives in its own hook. Empty marks mean single-select mode, where selectedMessage alone is selected.
    const selection = useSelection()
    const {markedIds, setMarkedIds, anchorId, setAnchorId, clear: clearSelection} = selection
    // The reading-pane mode, the full-width reader and the reader-tab interaction (open, close and the
    // focus choreography when an email is opened or closed) live in useReaderTabs. The reader tabs and the
    // active message themselves stay in the message store; this hook drives how they are shown.
    const {
        previewEnabled, readingFull, setReadingFull,
        readerBodyRef, readerSinkRef,
        selectMessage, openInNewTab, closeTab, togglePreview,
        popoutOpen, openPopout, closePopout,
    } = useReaderTabs({store})
    // Clicking the popout's backdrop closes it like any other dialog.
    const popoutDismiss = useBackdropDismiss(closePopout)
    // A neutral, offscreen focus anchor. It takes focus on launch so nothing is highlighted, yet the very
    // first Tab has a starting point and enters the ring. The WebView often does not hold keyboard focus
    // at the instant the window first appears, so focusing once on mount is not enough: the window focus
    // listener reclaims the anchor when the WebView gains focus (and only while nothing else holds it, so
    // it never steals focus once the user has started navigating).
    const neutralFocusRef = useRef<HTMLSpanElement>(null)
    useEffect(() => {
        const claimNeutralFocus = () => {
            // While any dialog is open its own focus management rules: grabbing the anchor here
            // would pull focus out of a compose or confirm the user is mid-way through.
            if (document.querySelector('.modal')) {
                return
            }
            const active = document.activeElement
            if (!active || active === document.body || active === neutralFocusRef.current) {
                neutralFocusRef.current?.focus()
            }
        }
        claimNeutralFocus()
        window.addEventListener('focus', claimNeutralFocus)
        return () => window.removeEventListener('focus', claimNeutralFocus)
    }, [])

    // Opening the composer (reply, reply-all, forward, attach) and the draft-recovery prompt shown on launch
    // live in useComposeLauncher. The ComposeModal render, the Compose buttons and the recovery dialog stay in
    // App and consume the state, the setters and signatureHtml it returns.
    const {
        composing, setComposing,
        composeInitial, setComposeInitial,
        attachPickerOpen, setAttachPickerOpen,
        recovery, setRecovery,
        signatureHtml,
        openDraft, openReply, openReplyAll, openForward,
        attachToNewMessage, attachFiles, attachEmails,
        restoreDraft, discardDraft,
    } = useComposeLauncher({accounts, selectedAccount, setSelectedAccount, messageBody, setError})

    const searchActive = searchQuery.trim() !== ''
    // undoSendSeconds is the undo-send window: how long a sent message is held (cancellable) before it
    // actually leaves. Zero disables the hold. Chosen from the Mail menu, remembered across launches.
    const [undoSendSeconds, setUndoSendSecondsState] = useState<number>(() => {
        // An absent or blank key is not a choice of zero. Number(null) and Number('') are both 0, which is
        // itself a legal choice (Off), so reading the value straight through Number made every fresh
        // install accept it and put the default out of reach: undo send shipped off.
        const stored = localStorage.getItem('undoSendSeconds')
        const chosen = stored === null || stored.trim() === '' ? Number.NaN : Number(stored)
        return (undoSendChoices as readonly number[]).includes(chosen) ? chosen : defaultUndoSendSeconds
    })
    const setUndoSendSeconds = useCallback((seconds: number) => {
        setUndoSendSecondsState(seconds)
        localStorage.setItem('undoSendSeconds', String(seconds))
    }, [])
    // conversationView groups the folder's messages into conversations; it does not apply to search
    // results, which stay ranked by relevance. The choice is remembered across launches.
    const [conversationView, setConversationView] = useState<boolean>(() => localStorage.getItem('conversationView') === '1')
    // threadHeadId is the conversation the reader is showing whole, named by any message of it; null
    // when the reader is showing a single message. The list's conversation header opens one; picking any
    // message, changing folder or pressing Back closes it, so the thread never lingers over unrelated mail.
    const [threadHeadId, setThreadHeadId] = useState<string | null>(null)
    // The tick governs the whole feature rather than the list alone: turning it off flattens the list,
    // takes the strip out from under the open message and closes any thread being read. Someone who has
    // said they do not want conversations should see none of them.
    const toggleConversationView = useCallback(() => {
        setConversationView((on) => {
            const next = !on
            localStorage.setItem('conversationView', next ? '1' : '0')
            if (!next) {
                setThreadHeadId(null)
            }
            return next
        })
    }, [])
    // unifiedMailbox is the View tick that shows the sidebar's All-inboxes entry: every account's inbox
    // merged into one list. Off by default, remembered across launches. Its toggle (toggleUnifiedMailbox)
    // is defined below, after selectFolder, because turning it on opens the combined view and turning it
    // off while that view is open falls back to the inbox.
    const [unifiedMailbox, setUnifiedMailbox] = useState<boolean>(() => localStorage.getItem('unifiedMailbox') === '1')
    // autoLoadImages, when on, loads a message's remote images immediately instead of holding them behind the
    // Load images bar. It is off by default to protect privacy (a remote image can report that the message was
    // opened) and is remembered across launches. The View menu toggles it.
    const [autoLoadImages, setAutoLoadImages] = useState<boolean>(() => localStorage.getItem('autoLoadImages') === '1')
    const toggleAutoLoadImages = useCallback(() => {
        setAutoLoadImages((on) => {
            const next = !on
            localStorage.setItem('autoLoadImages', next ? '1' : '0')
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
    // sortAscending's toggle (toggleSort) is defined below, after loadFolderMessages, because flipping the
    // order reloads the flat view's first page in the new direction.
    const [sortAscending, setSortAscending] = useState<boolean>(() => localStorage.getItem('sortAscending') === '1')
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

    // openReminderEvent opens the calendar with the reminder's event dialog on top, so a clicked reminder
    // shows what it is about. Events are refreshed first so the calendar can find and jump to the event.
    const openReminderEvent = useCallback((eventId: string) => {
        setCalendarInitialEvent(eventId)
        setManagingCalendar(true)
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

    // Persist the watermarks whenever they change, so "last looked" survives a restart.
    useEffect(() => {
        saveWatermarks(localStorage, watermarks)
    }, [watermarks])

    // Reconcile the watermarks with the account list: an account seen for the first time starts as
    // looked-at now (its pre-existing backlog must not light the cue) and a removed account's entry
    // is dropped. ensureWatermarks returns the same object when nothing changed, so this cannot loop.
    useEffect(() => {
        setWatermarks((marks) => ensureWatermarks(marks, accounts.map((a) => a.id), Date.now()))
    }, [accounts])

    // elsewhereMail summarises unread mail newly arrived on the accounts the picker's closed trigger
    // hides (newer than each account's watermark); the sidebar badges it beside the caret.
    const elsewhereMail = useMemo(
        () => elsewhereCue(
            selectedAccount, unreadCounts.byAccount, unreadCounts.newestByAccount ?? {}, watermarks,
        ),
        [selectedAccount, unreadCounts, watermarks],
    )

    // The Edit-menu undo and redo stacks live in useUndoRedo; every action hook below records its
    // completed actions through undoRedo.recorder.
    const undoRedo = useUndoRedo({store, loadUnread, refreshFolders, setError})

    // The colour-tag palette, the selected message's tags and the tag-toggle handlers (on the open message
    // and on any message via the context menu) live in useTags.
    const {tags, messageTags, toggleTag, setMessageTagById} = useTags({store, setError, undo: undoRedo.recorder})

    // The snooze actions (hide until a moment, bring back, the custom picker) and the Snoozed entry's
    // count live in useSnooze. Snooze is local-only state: the backend hides the message from the
    // visible listings until it comes due.
    const {
        snoozedCount, refreshSnoozedCount, snoozeTo, unsnooze, snoozePickerFor, setSnoozePickerFor,
    } = useSnooze({store, loadUnread, setError})

    // The single-message actions (delete, move, flag, read, junk, copy) and their single-delete confirm
    // state live in useMessageActions, wired to the message store and the loaders they need.
    const {
        messageToDelete, setMessageToDelete, deletingMessage,
        messageToPurge, setMessageToPurge, purgingMessage,
        requestDelete, deleteMessage, deletePermanent, toggleFlag, moveMessage, markJunk, markNotJunk, copyMessage,
        setReadState, toggleRead, markReadOnView, markReplied, markForwarded,
    } = useMessageActions({
        store, displayMessages, searchActive, folders, loadUnread, refreshFolders, setError,
        undo: undoRedo.recorder,
    })

    // The undo-send window (the queued item, its countdown, the cancel and the marking deferred to its
    // expiry) lives in useUndoSend. Reopening the composer is the one thing it hands back here, since
    // the compose state belongs to this component.
    const {held: undoToast, onHeldSend, undoHeldSend, heldSendElapsed} = useUndoSend({
        holdSeconds: undoSendSeconds,
        refreshOutbox,
        setError,
        reopenCompose: (initial) => {
            setComposeInitial(initial)
            setComposing(true)
        },
        markReplied,
        markForwarded,
    })

    // The backend dispatcher announces a held send leaving, so the outbox count and views refresh the
    // moment it goes rather than on the next manual action.
    useEffect(() => EventsOn('outbox:changed', () => {
        void refreshOutbox()
        void loadUnread()
    }), [refreshOutbox, loadUnread])

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

    // A scope whose anchor disappears (the folder deselected or synthetic, the account removed) falls
    // back to all mail visibly, so the selector never claims a narrower scope than the search actually
    // runs with. The unified mailbox is not a real folder, so it cannot anchor a folder scope.
    useEffect(() => {
        if ((searchScope === 'folder' && (!selectedFolder || isUnifiedFolder(selectedFolder) || isSnoozedFolder(selectedFolder)))
            || (searchScope === 'account' && !selectedAccount)) {
            setSearchScope('all')
        }
    }, [searchScope, selectedFolder, selectedAccount])

    // Debounced full-text search: results replace the folder listing while a query is active. The scope
    // selector narrows it to the selected folder or account; changing either re-runs the live query. The
    // stale flag discards a slow response that lands after the query has changed, so an older search can
    // never overwrite a newer one's results.
    useEffect(() => {
        const q = searchQuery.trim()
        if (q === '') {
            setSearchResults([])
            setSearchSnippets(new Map())
            setSearchDegraded(false)
            return
        }
        let stale = false
        const folderId = searchScope === 'folder' ? selectedFolder : ''
        const accountId = searchScope === 'account' ? selectedAccount : ''
        const handle = window.setTimeout(() => {
            void api.searchMessages(q, folderId, accountId).then((result) => {
                if (stale) {
                    return
                }
                setSearchResults(result.hits.map((hit) => hit.message))
                setSearchSnippets(new Map(result.hits.map((hit) => [hit.message.id, hit.snippet])))
                setSearchDegraded(result.degraded)
            }).catch((e) => setError(String(e)))
        }, 250)
        return () => {
            stale = true
            window.clearTimeout(handle)
        }
    }, [searchQuery, searchScope, selectedFolder, selectedAccount])

    // loadFolderMessages shows a folder's cached messages immediately (so it opens instantly), then
    // refreshes it from the server and updates the list if the user is still on that folder. This is
    // what makes a message moved or deleted into a folder appear when the folder is opened. A sync
    // failure (offline) simply leaves the cached view in place.
    //
    // The flat view (the default) loads only the first page and lets loadMoreMessages append the rest as
    // the user scrolls, so a folder of tens of thousands of messages opens without loading every row.
    // Conversation view needs the whole folder to thread it, so it loads every row (the virtualised list
    // renders only what is on screen, so even a 48k folder does not freeze the render). Every (re)load
    // resets pagination. opts overrides the direction or the view mode when a toggle drives the reload
    // before its state has settled; skipSync loads once without re-syncing for a caller that has just
    // synced (a mailbox sync or the background poll).
    const loadFolderMessages = useCallback(async (
        id: string, opts?: {ascending?: boolean; conversation?: boolean; skipSync?: boolean},
    ) => {
        const ascending = opts?.ascending ?? sortAscending
        const grouped = opts?.conversation ?? conversationView
        pagination.reset()
        const loadInto = grouped
            ? async () => setMessages(await api.listMessages(id))
            : async () => setMessages(await pagination.loadFirst(id, ascending))
        try {
            await loadInto()
        } catch (e) {
            setError(String(e))
        }
        if (opts?.skipSync) {
            return
        }
        try {
            await api.syncFolder(id)
            if (selectedFolderRef.current === id) {
                await loadInto()
            }
            await loadUnread()
        } catch {
            // Offline or a transient failure: the cached view stands.
        }
    }, [sortAscending, conversationView, loadUnread, pagination])

    // loadMoreMessages appends the next page to the flat folder view as the list nears its end. It is a
    // no-op in conversation view and in search (both hold the whole set), on the synthetic Outbox and when
    // no page remains or a load is already running (the pagination hook guards those). A late page from a
    // folder the user has since left is discarded; duplicate ids are never appended.
    const loadMoreMessages = useCallback(async () => {
        if (conversationView || searchActive) {
            return
        }
        const folderId = selectedFolderRef.current
        if (!folderId || folderId === OUTBOX_FOLDER_ID || !pagination.hasMore()) {
            return
        }
        try {
            const loadedIds = new Set(messagesRef.current.map((m) => m.id))
            const additions = await pagination.loadNext(folderId, loadedIds)
            if (additions.length === 0 || selectedFolderRef.current !== folderId) {
                return
            }
            setMessages((prev) => {
                const seen = new Set(prev.map((m) => m.id))
                const fresh = additions.filter((m) => !seen.has(m.id))
                return fresh.length > 0 ? [...prev, ...fresh] : prev
            })
        } catch (e) {
            setError(String(e))
        }
    }, [conversationView, searchActive, pagination])

    // toggleSort flips the date order and persists it. Only a prefix of the folder is loaded in the flat
    // view, so a client-side re-sort would reorder just that prefix and hide the true first rows of the new
    // direction; instead pagination is reset and page one is reloaded in the new order. Conversation view
    // and search hold the whole set, so the derived list re-sorts them client-side without a reload.
    const toggleSort = useCallback(() => {
        const next = !sortAscending
        localStorage.setItem('sortAscending', next ? '1' : '0')
        setSortAscending(next)
        const folderId = selectedFolderRef.current
        if (folderId && folderId !== OUTBOX_FOLDER_ID && !conversationView && !searchActive) {
            void loadFolderMessages(folderId, {ascending: next})
        }
    }, [sortAscending, conversationView, searchActive, loadFolderMessages])

    // Reload the open folder when the conversation view is toggled, because the two modes load different
    // amounts: flat loads a page at a time, conversation loads the whole folder to thread it. Skipped on the
    // first render (the account auto-select already opens the inbox) and on the synthetic Outbox.
    const conversationViewMounted = useRef(false)
    useEffect(() => {
        if (!conversationViewMounted.current) {
            conversationViewMounted.current = true
            return
        }
        const folderId = selectedFolderRef.current
        if (folderId && folderId !== OUTBOX_FOLDER_ID) {
            void loadFolderMessages(folderId, {conversation: conversationView})
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [conversationView])

    const selectAccount = useCallback(async (id: string) => {
        // Leaving an account stamps it as looked-at up to this instant and entering one clears its
        // cue, so the elsewhere badge lights only for mail arriving after you were last there.
        setWatermarks((marks) => touchWatermarks(marks, [selectedAccountRef.current, id], Date.now()))
        setSelectedAccount(id)
        // Claim the folder list for this account before the fetch, so a slower fetch still running for the
        // account being left cannot land afterwards and put its folders back under the new account's
        // selection. That mismatch leaves the sidebar showing the previous account's folders, with no row
        // matching the newly opened Inbox and so no highlight and none of its unread badges.
        selectedAccountRef.current = id
        localStorage.setItem(SELECTED_ACCOUNT_KEY, id)
        setSelectedMessage(null)
        setMarkedIds(new Set())
        setAnchorId(null)
        setReadingFull(false)
        try {
            const fetched = await api.listFolders(id)
            if (selectedAccountRef.current !== id) {
                return
            }
            applyFolders(id, fetched)
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
    }, [loadFolderMessages, applyFolders, selectedAccountRef])

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

    // After a stretch of no user activity the app returns to its resting view: the active account's
    // Inbox becomes the selected folder (highlighted in the sidebar, its list shown) and keyboard
    // focus lands on its row. Skipped while any dialog or menu is open (they own their focus), while
    // the caret sits in a text field (a pause mid-entry must not lose the caret) and while a message
    // is open in the reader (returning to the Inbox would close it mid-read); an idle stretch
    // already on the Inbox only re-settles the focus.
    const idleReturnToInbox = useCallback(() => {
        if (document.querySelector('.modal-backdrop, .context-menu, [role="menu"]')) {
            return
        }
        if (isTypingTarget(document.activeElement)) {
            return
        }
        if (selectedMessage) {
            return
        }
        const inbox = folders.find((f) => f.kind === 'inbox')
        if (!inbox) {
            return
        }
        if (selectedFolderRef.current !== inbox.id) {
            void selectFolder(inbox.id)
        }
        document.querySelector<HTMLElement>(`[data-folder-id="${CSS.escape(inbox.id)}"]`)?.focus()
    }, [folders, selectFolder, selectedFolderRef, selectedMessage])
    useIdleRefocus(idleReturnToInbox)

    // toggleUnifiedMailbox shows or hides the sidebar's All-inboxes entry and persists the choice.
    // Turning it on opens the combined view immediately (that is the point of the tick); turning it off
    // while the combined view is open falls back to the selected account's inbox, mirroring the Outbox's
    // empty-queue fallback.
    const toggleUnifiedMailbox = useCallback(() => {
        const next = !unifiedMailbox
        localStorage.setItem('unifiedMailbox', next ? '1' : '0')
        setUnifiedMailbox(next)
        if (next) {
            void selectFolder(UNIFIED_FOLDER_ID)
            return
        }
        if (selectedFolder === UNIFIED_FOLDER_ID) {
            const fallback = folders.find((f) => f.kind === 'inbox') ?? folders[0]
            if (fallback) {
                void selectFolder(fallback.id)
            } else {
                selectedFolderRef.current = ''
                setSelectedFolder('')
                setMessages([])
            }
        }
    }, [unifiedMailbox, selectedFolder, folders, selectFolder])

    // On first load (or after the account list changes) open an account automatically, so the app lands
    // on a populated inbox rather than an empty pane. The account open at the last close is reopened;
    // one that has since been removed (or a first ever run) falls back to the first account.
    useEffect(() => {
        if (!selectedAccount && accounts.length > 0) {
            const remembered = localStorage.getItem(SELECTED_ACCOUNT_KEY) ?? ''
            const restored = accounts.find((a) => a.id === remembered) ?? accounts[0]
            void selectAccount(restored.id)
        }
    }, [accounts, selectedAccount, selectAccount])

    // Keep the Outbox view live while it is open: re-map the rows when the queue changes, drop a
    // selection whose item was cancelled or sent, then fall back to the inbox once the queue is empty
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

    // The backend scheduler announces resurfaced snoozes, so the badges and the open view (the inbox a
    // message returns to, else the Snoozed view it leaves) refresh the moment it comes back.
    useEffect(() => EventsOn('snooze:changed', () => {
        void refreshSnoozedCount()
        void loadUnread()
        const folderId = selectedFolderRef.current
        if (folderId && folderId !== OUTBOX_FOLDER_ID) {
            void loadFolderMessages(folderId, {skipSync: true})
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }), [refreshSnoozedCount, loadUnread, loadFolderMessages])

    // The Snoozed view empties as messages resurface or are unsnoozed; once nothing is hidden its
    // sidebar entry disappears, so fall back to the inbox, mirroring the Outbox's empty-queue fallback.
    useEffect(() => {
        if (selectedFolder !== SNOOZED_FOLDER_ID || snoozedCount > 0) {
            return
        }
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
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [selectedFolder, snoozedCount, folders, loadFolderMessages])

    // The mailbox sync (a manual full-account sync and the periodic light refresh of the open folder) and
    // the per-account "is syncing" state live in useSync.
    const {syncingAccounts, sync, accountSyncing} = useSync({
        selectedAccount, selectedFolder, selectedFolderRef, applyFolders,
        reloadFolder: loadFolderMessages,
        refreshFolders, refreshOutbox, loadUnread, setError,
    })

    // syncAfterSave holds the id of an account just saved, so the effect below can sync it once the
    // selection has settled. sync() is bound to selectedAccount, so calling it directly inside
    // onAccountSaved would sync the previously selected account (or nothing, on first run).
    const [syncAfterSave, setSyncAfterSave] = useState<string>('')

    const onAccountSaved = useCallback(async (email: string) => {
        setSettingUp(false)
        setAccountToEdit(null)
        await loadAccounts()
        await selectAccount(email)
        setSyncAfterSave(email)
    }, [loadAccounts, selectAccount])

    // A saved account syncs straight away, so adding an account is the whole switch: the first-run user
    // never has to discover the Sync control to see their mail; an edited account picks up its
    // changed server settings immediately.
    useEffect(() => {
        if (syncAfterSave && selectedAccount === syncAfterSave) {
            setSyncAfterSave('')
            void sync()
        }
    }, [syncAfterSave, selectedAccount, sync])

    const closeContextMenu = useCallback(() => setContextMenu(null), [])

    // activateRow applies the standard list-selection gestures to a row click. A plain click selects the
    // one row and opens it; Ctrl (or Cmd) click toggles the row in or out of the selection; Shift click
    // selects the contiguous range from the anchor. The clicked row always becomes the active one shown in
    // the reader; a Shift range keeps the existing anchor so successive Shift clicks re-range from it.
    const activateRow = useCallback((message: Message, mods: {ctrl: boolean; shift: boolean}) => {
        setThreadHeadId(null)
        const list = searchActive ? searchResults : displayMessages
        if (mods.shift && anchorId) {
            setMarkedIds(rangeIds(list, anchorId, message.id))
            setSelectedMessage(message)
            setReadingFull(false)
            return
        }
        if (mods.ctrl) {
            setMarkedIds((prev) => toggleId(prev, message.id, selectedMessage?.id ?? null))
            setAnchorId(message.id)
            setSelectedMessage(message)
            setReadingFull(false)
            return
        }
        setMarkedIds(new Set())
        setAnchorId(message.id)
        selectMessage(message)
    }, [searchActive, searchResults, displayMessages, anchorId, selectedMessage, selectMessage])

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

    // Saving a message as .eml and printing one both live in useMessageExport: message in, document
    // out, with no state here beyond an error to report.
    const {saveMessageAs, printMessage} = useMessageExport({setError})

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

    // The update check: an automatic pass shortly after launch and daily thereafter surfaces the
    // modal only for a newer, non-skipped release; checkUpdates is the manual Help menu path and
    // reports every outcome.
    const {updateStatus, setUpdateStatus, checkUpdates, skipUpdate} = useUpdateCheck()

    // The backend-event wiring (the Windows tray menu and app:close-request, an OS-handed .eml plus the poll
    // events that refresh the unread counts and the open folder or the calendar) plus the Windows platform
    // detect live in useAppEvents. It owns launchedEmail, isWindows and closeChoice, which App's render feeds
    // to the EmailViewer, the Mail menu and the CloseChoice dialog.
    const {
        launchedEmail, setLaunchedEmail,
        isWindows,
        closeChoice, setCloseChoice,
    } = useAppEvents({
        showAbout, showLicence, checkUpdates,
        selectedFolder, reloadFolder: loadFolderMessages, refreshFolders,
        loadUnread, loadEvents, setError,
    })

    // The bulk actions over a multi-selection (bulk delete, permanent delete, move, read, flag), the
    // drag-and-drop-onto-folder handler and the bulk-delete confirm state live in useBulkActions, wired to
    // the message store, the selection and the loaders they need.
    const {
        bulkToDelete, setBulkToDelete, bulkDeleting,
        bulkToPurge, setBulkToPurge, bulkPurging,
        runBulkDelete, bulkSetRead, bulkSetFlag, bulkMove, dropMessageOnFolder,
    } = useBulkActions({store, selection, folders, loadUnread, refreshFolders, setError, undo: undoRedo.recorder})

    // The message-level half of Edit > Cut / Copy / Paste: cut or copy takes the selected messages
    // onto an internal clipboard and paste files them into the folder being viewed (a cut moves
    // optimistically, so the rows appear at once and the server settles behind them; a copy
    // duplicates). pasteFolderId is the paste target: the open folder, else '' in the views that are
    // not one real folder (the outbox, the unified mailbox and Snoozed), which disables the paste.
    const messageClipboard = useMessageClipboard({
        store, selectedFolderId: selectedFolder, undo: undoRedo.recorder, loadUnread, refreshFolders, setError,
    })
    // folderContextMenu is the folder row's right-click menu (Paste onto that folder), the
    // folder-side counterpart of contextMenu.
    const [folderContextMenu, setFolderContextMenu] = useState<{folder: Folder; x: number; y: number} | null>(null)
    const pasteFolderId = selectedFolder && selectedFolder !== OUTBOX_FOLDER_ID &&
        !isUnifiedFolder(selectedFolder) && !isSnoozedFolder(selectedFolder) ? selectedFolder : ''
    const pasteMessages = useCallback(() => {
        if (pasteFolderId !== '') {
            void messageClipboard.pasteInto(pasteFolderId)
        }
    }, [messageClipboard, pasteFolderId])

    // A message shown in the reader (the preview pane or the full-width reader when the pane is off)
    // counts as read, so viewing or double-clicking a message un-bolds it. Auto-read fires once per
    // selection, keyed by message id: without this guard, explicitly marking the open message unread
    // re-runs this effect (its read flag changed) and immediately re-reads it. The id is unchanged on
    // that re-run, so nothing happens; selecting a different message later reads it as expected.
    const autoReadIdRef = useRef<string | null>(null)
    useEffect(() => {
        if (!(previewEnabled || readingFull || popoutOpen) || !selectedMessage) {
            return
        }
        if (autoReadIdRef.current === selectedMessage.id) {
            return
        }
        autoReadIdRef.current = selectedMessage.id
        if (!selectedMessage.read) {
            void markReadOnView(selectedMessage)
        }
    }, [selectedMessage, previewEnabled, readingFull, popoutOpen, markReadOnView])

    // A message in the Drafts mailbox is unfinished work rather than correspondence, so it opens in the
    // composer instead of the reader. The folder kind is the test, so it holds for a draft reached from a
    // search result as well as from the Drafts folder itself. It reads the loaded folder list, which is the
    // selected account's, so a draft row belonging to another account in the unified mailbox is not
    // recognised as one; that row still opens in the reader, exactly as it did before.
    const isDraftMessage = (message: Message): boolean =>
        folders.some((f) => f.id === message.folderId && f.kind === 'drafts')

    // openMessageRow is what opening a row does, from a double-click or from Enter. A message in the Drafts
    // mailbox is unfinished work rather than correspondence, so it opens in the composer; everything else
    // opens in the popout reader as before.
    const openMessageRow = (message: Message) => {
        if (isDraftMessage(message)) {
            void openDraft(message)
            return
        }
        openPopout(message)
    }

    // The window keydown handler for the message list and the main-window focus ring lives in
    // useMessageListKeyboard. It reads the current view and its selection, every overlay state (list
    // handling is suppressed while any is open) and the handlers a key fires (open, delete, folder delete).
    useMessageListKeyboard({
        searchActive, searchResults, displayMessages, selectedMessage, setSelectedMessage,
        markedIds, setMarkedIds, anchorId, setAnchorId, setReadingFull,
        splashVisible, composing, settingUp, accountToEdit, managingRules, managingTemplates, managingContacts, managingCalendar,
        about, licence, folderPrompt, messageToCancelSend, messageToDelete, accountToDelete, folderToDelete,
        messageToPurge, contextMenu, folderContextMenu, bulkToDelete, bulkToPurge, snoozePickerFor, folders,
        requestDelete, openMessage: openMessageRow,
        onCutMessages: messageClipboard.cutMessages,
        onCopyMessages: messageClipboard.copyMessages,
        onPasteMessages: pasteMessages,
        setMessageToPurge, setBulkToPurge, setBulkToDelete, setFolderToDelete,
        togglePreview,
    })

    // A POP3 account has a single downloaded inbox with no server-side folders, message moves or draft
    // mailbox, so those actions are hidden and a delete is permanent rather than a move to Trash.
    const activeAccount = accounts.find((a) => a.id === selectedAccount)
    const isPop3 = activeAccount?.protocol === 'pop3'
    // The unified mailbox and the Snoozed view span accounts. Move and Copy (and Junk) are unavailable
    // in both: the folder targets belong to one account while the rows span them all, so those actions
    // live in the message's real folder instead.
    const unifiedSelected = isUnifiedFolder(selectedFolder)
    const snoozedSelected = isSnoozedFolder(selectedFolder)
    const canMoveCopy = !isPop3 && !unifiedSelected && !snoozedSelected
    // messagePop3 resolves a message's own account (a unified row can belong to any account, other rows
    // fall back to the selected one) to word its delete confirmation honestly: POP3 has no Trash.
    const messagePop3 = (message: Message | null): boolean => {
        const owner = accounts.find((a) => a.id === (message?.accountId || selectedAccount))
        return owner?.protocol === 'pop3'
    }
    // supersedeDraft drops the stored draft a compose window was reopened from, once its replacement has
    // been sent or saved. The row goes at once, then the copy itself. The delete is permanent by design: the
    // copy is stale the moment its replacement exists and routing every intermediate save through Trash
    // would fill it with versions of one message. A failed delete is swallowed, so the worst case is a
    // duplicate draft rather than a lost message.
    const supersedeDraft = (id: string) => {
        store.removeFromAllLists(new Set([id]))
        void api.deleteMessagePermanent(id).then(() => refreshFolders()).catch(() => {})
    }

    // accountDots colours each account for the unified list's per-account row dot, by sidebar order.
    const accountDots = useMemo(() => accountChips(accounts), [accounts])
    // The composer sends from the account the launcher resolved (a reply to a unified row must send from
    // that row's own account), falling back to the selected one for a fresh compose.
    const composeAccountId = composeInitial?.accountId || selectedAccount
    const composeAccount = accounts.find((a) => a.id === composeAccountId)

    // Derived selection: the visible list, the set of highlighted rows (the Ctrl/Shift selection, else just
    // the active message when there is none), the messages any bulk action operates on and whether more
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

    // Edit > Select all marks every row in the current view, the same gesture as Ctrl+A on the list.
    const selectAll = useCallback(() => {
        if (visibleList.length === 0) {
            return
        }
        setMarkedIds(new Set(visibleList.map((m) => m.id)))
        setAnchorId(visibleList[0].id)
    }, [visibleList])

    // Edit > Cut / Copy / Paste dispatch by context, text first: with a text selection (or a
    // focused field for paste) they are the ordinary text commands; the menus never steal
    // focus from a text field so that selection survives the click. Otherwise they act at the
    // message level through the message clipboard: cut or copy takes the selected email(s) and
    // paste files them into the open folder. Text paste reads the clipboard through the Wails
    // runtime, best-effort: an unreadable clipboard pastes nothing.
    const editContext = useEditContext()
    const clipboardTargets = selectedMessages.filter((m) => !isOutboxMessage(m))
    const canCutNow = canCut(editContext) || clipboardTargets.length > 0
    const canCopyNow = canCopy(editContext) || clipboardTargets.length > 0
    const canPasteNow = canPaste(editContext) || (messageClipboard.hasClip && pasteFolderId !== '')
    const cut = () => {
        if (canCut(editContext)) {
            cutSelection(document)
            return
        }
        messageClipboard.cutMessages(clipboardTargets)
    }
    const copy = () => {
        if (canCopy(editContext)) {
            copySelection(document)
            return
        }
        messageClipboard.copyMessages(clipboardTargets)
    }
    const paste = () => {
        if (canPaste(editContext)) {
            api.clipboardText().then((text) => pasteText(document, text)).catch(() => {})
            return
        }
        pasteMessages()
    }

    // openThread shows a whole conversation in the reader. With the reading pane off the reader is not on
    // screen at all, so it is switched to as well; Back returns to the list exactly as it does for a
    // single message.
    const openThread = useCallback((headMessageId: string) => {
        setThreadHeadId(headMessageId)
        setReadingFull(true)
    }, [setReadingFull])

    // The message list and reader are extracted so the reading-pane layout can place them side by side
    // (pane on) or swap between them (pane off: the list, else the full-width reader when a message is opened).
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
            searchScope={searchScope}
            onScopeChange={setSearchScope}
            canScopeFolder={Boolean(selectedFolder) && !unifiedSelected && !snoozedSelected}
            canScopeAccount={Boolean(selectedAccount)}
            searchDegraded={searchDegraded}
            matchSnippets={searchSnippets}
            accountChips={accountDots}
            cutIds={messageClipboard.cutIds}
            searchInputRef={searchInputRef}
            onActivate={activateRow}
            onOpenConversation={openThread}
            onClearSelection={clearSelection}
            onToggleFlag={(m) => void toggleFlag(m)}
            onContextMenu={openContextMenu}
            onPopout={openMessageRow}
            onLoadMore={() => void loadMoreMessages()}
        />
    )
    // paneWidths drives the draggable splitters between the sidebar, the message list and the reader:
    // the widths land on the .panes grid as CSS variables and persist across restarts.
    const paneWidths = usePaneWidths()
    const paneStyle = {
        ['--sidebar-w']: `${paneWidths.widths.sidebar}px`,
        ['--list-w']: `${paneWidths.widths.list}px`,
    } as CSSProperties

    // conversation is the open message's whole thread, gathered across the account's folders so the
    // reader can offer the messages the list cannot show beside it.
    const conversation = useConversation(conversationView && selectedMessage ? selectedMessage.id : null)

    // readerProps is the reader surface shared by the pane (or full-width) reader and the popout
    // dialog, so both render the selected message identically.
    const readerProps = {
        message: selectedMessage,
        bodyRef: readerBodyRef,
        sinkRef: readerSinkRef,
        onToggleRead: (m: Message) => void toggleRead(m),
        onReply: openReply,
        onReplyAll: openReplyAll,
        onForward: openForward,
        // A draft shows Edit draft in place of the reply and forward actions: answering your own unsent
        // message is never what was meant.
        isDraft: selectedMessage !== null && isDraftMessage(selectedMessage),
        onEditDraft: (m: Message) => void openDraft(m),
        onDelete: (m: Message) => setMessageToDelete(m),
        onCancelSend: (m: Message) => setMessageToCancelSend(m),
        folders,
        onMove: (m: Message, dest: string) => void moveMessage(m, dest),
        onCopy: (m: Message, dest: string) => void copyMessage(m, dest),
        canMoveCopy,
        autoLoadImages,
        dark: theme === 'dark',
        tags,
        messageTags,
        onToggleTag: (tagId: string, assigned: boolean) => void toggleTag(tagId, assigned),
        body: messageBody,
        bodyLoading,
        conversation,
        // A thread entry opens in its own reader tab: it usually lives in another folder; switching
        // the list out from under the reader to show it would lose the message being read.
        onOpenConversationEntry: (m: Message) => openInNewTab(m),
    }
    // A whole conversation wins the reader when one is open: it was asked for explicitly; it also carries
    // the message that would otherwise be shown, expanded, among its own rows.
    const readerEl = threadHeadId && conversationView ? (
        <ThreadView
            headMessageId={threadHeadId}
            subject={selectedMessage?.subject ?? ''}
            onClose={() => {
                setThreadHeadId(null)
                setReadingFull(false)
            }}
            onOpenMessage={(m: Message) => {
                setThreadHeadId(null)
                openInNewTab(m)
            }}
            autoLoadImages={autoLoadImages}
            dark={theme === 'dark'}
        />
    ) : multiSelected ? (
        <SelectionSummary
            markedIds={markedIds}
            selectedMessages={selectedMessages}
            setBulkToDelete={setBulkToDelete}
            bulkSetRead={bulkSetRead}
            clearSelection={clearSelection}
        />
    ) : (
        <Reader
            {...readerProps}
            tabs={tabs}
            onSelectTab={setSelectedMessage}
            onCloseTab={closeTab}
            onBack={previewEnabled ? undefined : () => setReadingFull(false)}
        />
    )

    // The title-bar menus are defined here so one item list drives both the dropdown and the global
    // accelerator handler above, keeping each item's shortcut hint and its wired key in step. The Mail menu
    // opens with Add account and Sync, then mirrors the right-click actions on the active message.
    const activeMessage = selectedMessage
    const activeOutbox = activeMessage ? isOutboxMessage(activeMessage) : false
    // canMailAct gates the actions that need a real, non-outbox message on screen (reply, mark, move and
    // the rest). A queued outbox item only supports Cancel send.
    const canMailAct = Boolean(activeMessage) && !activeOutbox
    const canReplyAll = canMailAct && activeMessage
        ? ((activeMessage.to?.length ?? 0) + (activeMessage.cc?.length ?? 0)) > 0
        : false
    // The five menu-bar definitions and the keyboard accelerators that fire the same items from anywhere in
    // the window live in useMenus. It takes the derived gating flags (shared with the titlebar) plus every
    // action handler, then returns the arrays the Menu components render. mailMoveTargets and the applied-tag
    // set are computed inside it.
    const {fileMenu, editMenu, viewMenu, mailMenu, helpMenu} = useMenus({
        activeMessage, activeOutbox, canMailAct, canReplyAll, canMoveCopy, selectedAccount, accountSyncing,
        isWindows, conversationView, previewEnabled, autoLoadImages, unifiedMailbox, undoSendSeconds, setUndoSendSeconds,
        folders, messageTags,
        saveMessageAs, printMessage,
        undoText: undoRedo.undoText, redoText: undoRedo.redoText,
        undoAction: undoRedo.undo, redoAction: undoRedo.redo,
        canCutNow, canCopyNow, canPasteNow, cut, copy, paste, selectAll, canSelectAll: visibleList.length > 0,
        setManagingRules, setManagingTemplates, focusSearch,
        toggleConversationView, togglePreview, toggleAutoLoadImages, toggleUnifiedMailbox,
        signatureHtml, setComposeInitial, setComposing, setSettingUp, sync, openInNewTab, openThread,
        openReply, openReplyAll, openForward, attachToNewMessage, setReadState, toggleFlag, toggleTag,
        attachFiles, setAttachPickerOpen, displayMessages,
        moveMessage, copyMessage, markJunk, markNotJunk, snoozeTo, unsnooze, setSnoozePickerFor,
        setMessageToCancelSend, requestDelete, setMessageToPurge,
        showAbout, showLicence, checkUpdates,
    })

    return (
        <div className="app">
            <span
                ref={neutralFocusRef}
                tabIndex={-1}
                style={{position: 'absolute', width: 1, height: 1, overflow: 'hidden', opacity: 0, pointerEvents: 'none', outline: 'none'}}
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
            {undoToast && (
                <UndoSendToast
                    expiresAt={undoToast.expiresAt}
                    onUndo={() => void undoHeldSend()}
                    onExpired={heldSendElapsed}
                />
            )}
            <TitleBar
                unreadCounts={unreadCounts}
                fileMenu={fileMenu}
                editMenu={editMenu}
                viewMenu={viewMenu}
                mailMenu={mailMenu}
                helpMenu={helpMenu}
                selectedAccount={selectedAccount}
                accountSyncing={accountSyncing}
                theme={theme}
                signatureHtml={signatureHtml}
                setComposeInitial={setComposeInitial}
                setComposing={setComposing}
                setSettingUp={setSettingUp}
                sync={sync}
                setManagingContacts={setManagingContacts}
                setManagingCalendar={setManagingCalendar}
                setTheme={setTheme}
            />
            <ErrorBar message={error} onDismiss={() => setError('')}/>
            {accounts.length === 0 && !splashVisible ? (
                <WelcomeScreen setSettingUp={setSettingUp}/>
            ) : (
            <div className={'panes' + (previewEnabled ? '' : ' no-preview')} style={paneStyle}>
                <Sidebar
                    accounts={accounts}
                    selectedAccount={selectedAccount}
                    unifiedEnabled={unifiedMailbox}
                    unifiedSelected={unifiedSelected}
                    unifiedUnread={unreadCounts.total}
                    onSelectUnified={() => void selectFolder(UNIFIED_FOLDER_ID)}
                    snoozedCount={snoozedCount}
                    snoozedSelected={snoozedSelected}
                    onSelectSnoozed={() => void selectFolder(SNOOZED_FOLDER_ID)}
                    syncingAccountIds={syncingAccounts}
                    unreadByAccount={unreadCounts.byAccount}
                    elsewhereCue={elsewhereMail}
                    folders={sidebarFolders}
                    selectedFolder={selectedFolder}
                    onSelectAccount={(id) => void selectAccount(id)}
                    onSelectFolder={(id) => void selectFolder(id)}
                    onEditAccount={(account) => setAccountToEdit(account)}
                    onDeleteAccount={(account) => setAccountToDelete(account)}
                    onNewFolder={() => setFolderPrompt({mode: 'create'})}
                    onRenameFolder={(folder) => setFolderPrompt({mode: 'rename', folder})}
                    onNewSubfolder={(folder) => setFolderPrompt({mode: 'create', parent: folder})}
                    onReparentFolder={(folderId, newParentId) => void reparentFolder(folderId, newParentId)}
                    onDeleteFolder={(folder) => setFolderToDelete(folder)}
                    onDropMessage={dropMessageOnFolder}
                    onFolderContextMenu={(folder, x, y) => setFolderContextMenu({folder, x, y})}
                    canManageFolders={!isPop3}
                />
                {previewEnabled ? (
                    <>
                        {messageListEl}
                        {readerEl}
                    </>
                ) : readingFull && (selectedMessage || (threadHeadId && conversationView)) ? (
                    readerEl
                ) : (
                    messageListEl
                )}
                <PaneSplitters control={paneWidths} showListSplitter={previewEnabled}/>
            </div>
            )}
            <BottomBar/>
            {attachPickerOpen && (
                <MessagePickerDialog
                    messages={displayMessages}
                    onAttach={attachEmails}
                    onCancel={() => setAttachPickerOpen(false)}
                />
            )}
            <AboutModal about={about} onClose={() => setAbout(null)}/>
            <UpdateModal
                status={updateStatus}
                onClose={() => setUpdateStatus(null)}
                onDownload={(url) => void api.openExternal(url)}
                onSkip={skipUpdate}
            />
            <LicenceModal text={licence} onClose={() => setLicence(null)}/>
            {launchedEmail && <EmailViewerModal email={launchedEmail} autoLoadImages={autoLoadImages} dark={theme === 'dark'} onClose={() => setLaunchedEmail(null)}/>}
            {popoutOpen && selectedMessage && !multiSelected && (
                <div className="modal-backdrop" {...popoutDismiss}>
                    <div
                        className="modal message-popout"
                        role="dialog"
                        aria-label={selectedMessage.subject || '(no subject)'}
                        onClick={(e) => e.stopPropagation()}
                    >
                        <ModalClose onClose={closePopout}/>
                        <Reader {...readerProps} tabs={[]} onSelectTab={setSelectedMessage} onCloseTab={closeTab}/>
                    </div>
                </div>
            )}
            {/* Gated on composing alone: an account-state blip must never silently unmount a compose
                the user is writing. With no resolvable account the modal still shows and a send simply
                fails with an error, which preserves the message. */}
            {composing && (
                <ComposeModal
                    accountId={composeAccountId}
                    senders={sendersFor(composeAccount)}
                    initial={composeInitial}
                    canSaveDraft={composeAccount?.protocol !== 'pop3'}
                    onMarkReplied={(id) => void markReplied(id)}
                    onMarkForwarded={(id) => void markForwarded(id)}
                    onDraftSuperseded={supersedeDraft}
                    holdSeconds={undoSendSeconds}
                    onHeld={onHeldSend}
                    onClose={() => {
                        setComposing(false)
                        setComposeInitial(undefined)
                        // A message composed while offline is queued: reflect that in the count.
                        void refreshOutbox()
                    }}
                />
            )}
            {recovery && !composing && (
                <DraftRecoveryDialog
                    recovery={recovery}
                    setRecovery={setRecovery}
                    discardDraft={discardDraft}
                    restoreDraft={restoreDraft}
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
                <RuleManagerModal
                    accounts={accounts}
                    rules={rules}
                    onChanged={() => void loadRules()}
                    onClose={() => setManagingRules(false)}
                />
            )}
            {managingTemplates && (
                <TemplateManagerModal
                    templates={templates}
                    onChanged={() => void loadTemplates()}
                    onClose={() => setManagingTemplates(false)}
                />
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
                    message={messagePop3(messageToDelete)
                        ? `Delete "${messageToDelete.subject || '(no subject)'}"? POP3 has no Trash, so it is permanently removed from the server and cannot be recovered.`
                        : `Delete "${messageToDelete.subject || '(no subject)'}"? It is moved to Trash; it is deleted permanently if it is already in Trash or the account has no Trash folder.`}
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
                        : `Delete ${bulkToDelete.length} messages? They are moved to Trash; where the account has no Trash folder they are deleted permanently.`}
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
                    canMoveCopy={canMoveCopy}
                    onSetTag={(id, tagId, assigned) => void setMessageTagById(id, tagId, assigned)}
                    onCutMessages={(msgs) => messageClipboard.cutMessages(msgs.filter((m) => !isOutboxMessage(m)))}
                    onCopyMessages={(msgs) => messageClipboard.copyMessages(msgs.filter((m) => !isOutboxMessage(m)))}
                    onPaste={pasteMessages}
                    canPaste={messageClipboard.hasClip && pasteFolderId !== ''}
                    onOpenInNewTab={openInNewTab}
                    onSaveAs={(m) => void saveMessageAs(m)}
                    onPrint={(m) => void printMessage(m)}
                    onAttachToNew={attachToNewMessage}
                    onMarkJunk={(m) => void markJunk(m)}
                    onMarkNotJunk={(m) => void markNotJunk(m)}
                    isJunk={(m) => isJunkFolderMessage(m, folders)}
                    onSnooze={(m, at) => void snoozeTo(m, at)}
                    onSnoozeCustom={(m) => setSnoozePickerFor(m)}
                    onUnsnooze={(m) => void unsnooze(m)}
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
            {folderContextMenu && (
                <FolderContextMenu
                    folder={folderContextMenu.folder}
                    x={folderContextMenu.x}
                    y={folderContextMenu.y}
                    canPaste={messageClipboard.hasClip}
                    onPaste={(folder) => void messageClipboard.pasteInto(folder.id)}
                    canManageFolders={!isPop3}
                    onNewSubfolder={(folder) => setFolderPrompt({mode: 'create', parent: folder})}
                    onRenameFolder={(folder) => setFolderPrompt({mode: 'rename', folder})}
                    onDeleteFolder={(folder) => setFolderToDelete(folder)}
                    onClose={() => setFolderContextMenu(null)}
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
                    title={folderPromptTitle(folderPrompt)}
                    label={folderPrompt.mode === 'create' ? 'Folder name' : 'New name'}
                    initialValue={folderPrompt.mode === 'rename' ? folderPrompt.folder?.name : ''}
                    confirmLabel={folderPrompt.mode === 'create' ? 'Create' : 'Rename'}
                    busy={folderBusy}
                    onSubmit={(value) => void submitFolderPrompt(value)}
                    onCancel={() => setFolderPrompt(null)}
                />
            )}
            {snoozePickerFor && (
                <ScheduleDialog
                    title="Snooze until"
                    label="Bring this message back at"
                    confirmLabel="Snooze"
                    onSubmit={(at) => {
                        const target = snoozePickerFor
                        setSnoozePickerFor(null)
                        void snoozeTo(target, at)
                    }}
                    onCancel={() => setSnoozePickerFor(null)}
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
            {/* The close choice renders after every other overlay, so it surfaces on top of whatever
                dialog is open when the window close button is pressed, never buried beneath one. */}
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
        </div>
    )
}

export default App
