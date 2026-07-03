import {useCallback, useEffect, useState} from 'react'
import './App.css'
import brandIcon from './assets/pigeonpost.png'
import {AboutInfo, Account, api, Folder, Message, MessageBody, Rule, Tag} from './api'
import {applyTheme, loadTheme, Theme} from './theme'
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
import {TagManagerModal} from './components/TagManagerModal'
import {RuleManagerModal} from './components/RuleManagerModal'
import {Splash} from './components/Splash'

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
    const [accounts, setAccounts] = useState<Account[]>([])
    const [selectedAccount, setSelectedAccount] = useState<string>('')
    const [folders, setFolders] = useState<Folder[]>([])
    const [selectedFolder, setSelectedFolder] = useState<string>('')
    const [messages, setMessages] = useState<Message[]>([])
    const [selectedMessage, setSelectedMessage] = useState<Message | null>(null)
    const [error, setError] = useState<string>('')
    const [syncing, setSyncing] = useState<boolean>(false)
    const [outboxCount, setOutboxCount] = useState<number>(0)
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
    const [managingTags, setManagingTags] = useState<boolean>(false)
    const [rules, setRules] = useState<Rule[]>([])
    const [managingRules, setManagingRules] = useState<boolean>(false)
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

    const searchActive = searchQuery.trim() !== ''
    const [appVersion, setAppVersion] = useState<string>('')
    const [appAuthor, setAppAuthor] = useState<string>('')
    const [splashVisible, setSplashVisible] = useState<boolean>(true)
    const [splashFading, setSplashFading] = useState<boolean>(false)

    useEffect(() => {
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

    const loadTags = useCallback(async () => {
        try {
            setTags(await api.listTags())
        } catch (e) {
            setError(String(e))
        }
    }, [])

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

    useEffect(() => {
        void loadTags()
    }, [loadTags])

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

    const onTagsChanged = useCallback(async () => {
        await loadTags()
        if (selectedMessage) {
            try {
                setMessageTags(await api.messageTags(selectedMessage.id))
            } catch (e) {
                setError(String(e))
            }
        }
    }, [loadTags, selectedMessage])

    const selectAccount = useCallback(async (id: string) => {
        setSelectedAccount(id)
        setSelectedFolder('')
        setMessages([])
        setSelectedMessage(null)
        try {
            setFolders(await api.listFolders(id))
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const selectFolder = useCallback(async (id: string) => {
        setSelectedFolder(id)
        setSelectedMessage(null)
        try {
            setMessages(await api.listMessages(id))
        } catch (e) {
            setError(String(e))
        }
    }, [])

    // refreshOutbox updates the count of outgoing operations waiting to be sent.
    const refreshOutbox = useCallback(async () => {
        try {
            setOutboxCount(await api.outboxCount())
        } catch {
            // A count read failing must not disrupt the UI; leave the last known value.
        }
    }, [])

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
        } catch (e) {
            setError(String(e))
        } finally {
            setSyncing(false)
        }
    }, [selectedAccount, selectedFolder, refreshOutbox])

    useEffect(() => {
        void refreshOutbox()
    }, [refreshOutbox])

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
        setDeletingMessage(true)
        setError('')
        try {
            await api.deleteMessage(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? null : prev))
            setMessageToDelete(null)
        } catch (e) {
            setError(String(e))
        } finally {
            setDeletingMessage(false)
        }
    }, [messageToDelete])

    // trashMessage moves a message to Trash with no confirmation, used by the Delete key when the
    // action is reversible. It advances the selection to the neighbouring message so repeated presses
    // clear a run of mail without a re-select.
    const trashMessage = useCallback(async (message: Message) => {
        const id = message.id
        const list = searchActive ? searchResults : messages
        const next = neighbourAfterRemoval(list, id)
        setError('')
        try {
            await api.deleteMessage(id)
            setMessages((prev) => prev.filter((m) => m.id !== id))
            setSearchResults((prev) => prev.filter((m) => m.id !== id))
            setTabs((prev) => prev.filter((m) => m.id !== id))
            setSelectedMessage((prev) => (prev?.id === id ? next : prev))
        } catch (e) {
            setError(String(e))
        }
    }, [searchActive, searchResults, messages])

    // deletePermanent is the confirmed, irreversible delete behind Shift+Delete: it removes the message
    // from the server without moving it to Trash, then advances the selection like trashMessage.
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
        } catch (e) {
            setError(String(e))
        } finally {
            setPurgingMessage(false)
        }
    }, [messageToPurge, searchActive, searchResults, messages])

    // requestDelete routes a (reversible) Delete: it trashes the message silently when it can be moved
    // to Trash, or opens the confirmation dialog when the delete would be permanent (already in Trash,
    // no Trash folder, or an unresolvable folder). Shared by the Delete key and the context menu.
    const requestDelete = useCallback((message: Message) => {
        const hasTrash = folders.some((f) => f.kind === 'trash')
        const folder = folders.find((f) => f.id === message.folderId)
        const permanent = !folder || folder.kind === 'trash' || !hasTrash
        if (permanent) {
            setMessageToDelete(message)
        } else {
            void trashMessage(message)
        }
    }, [folders, trashMessage])

    const closeContextMenu = useCallback(() => setContextMenu(null), [])

    // openInNewTab pins a message as a reader tab (if not already open) and shows it.
    const openInNewTab = useCallback((message: Message) => {
        setTabs((prev) => (prev.some((t) => t.id === message.id) ? prev : [...prev, message]))
        setSelectedMessage(message)
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

    const moveMessage = useCallback(async (message: Message, destFolderId: string) => {
        setError('')
        try {
            await api.moveMessage(message.id, destFolderId)
            setMessages((prev) => prev.filter((m) => m.id !== message.id))
            setSearchResults((prev) => prev.filter((m) => m.id !== message.id))
            setTabs((prev) => prev.filter((m) => m.id !== message.id))
            setSelectedMessage((prev) => (prev?.id === message.id ? null : prev))
        } catch (e) {
            setError(String(e))
        }
    }, [])

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

    // Keyboard control for the message list: Arrow Up/Down move the selection, Delete moves the
    // selected message to Trash (or asks for confirmation when the delete would be permanent), and
    // Shift+Delete always asks before deleting permanently. Handling is suppressed while any dialog is
    // open or while the user is typing in a field, so it never competes with text entry or a modal.
    useEffect(() => {
        const overlayOpen =
            splashVisible || composing || settingUp || Boolean(accountToEdit) || managingTags ||
            managingRules || Boolean(about) || Boolean(licence) || Boolean(folderPrompt) ||
            Boolean(messageToDelete) || Boolean(accountToDelete) || Boolean(folderToDelete) ||
            Boolean(messageToPurge) || Boolean(contextMenu)
        const list = searchActive ? searchResults : messages
        const onKeyDown = (e: KeyboardEvent) => {
            if (overlayOpen) {
                return
            }
            const target = e.target as HTMLElement | null
            if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' ||
                target.tagName === 'SELECT' || target.isContentEditable)) {
                return
            }
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
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
        splashVisible, composing, settingUp, accountToEdit, managingTags, managingRules, about,
        licence, folderPrompt, messageToDelete, accountToDelete, folderToDelete, messageToPurge,
        contextMenu,
    ])

    return (
        <div className="app">
            {splashVisible && <Splash version={appVersion} author={appAuthor} fading={splashFading}/>}
            <header className="titlebar">
                <span className="brand">PigeonPost</span>
                <div className="titlebar-right">
                    <button className="sync-btn" onClick={() => setSettingUp(true)}>
                        Add account
                    </button>
                    <button className="sync-btn" onClick={() => setManagingTags(true)}>
                        Tags
                    </button>
                    <button className="sync-btn" onClick={() => setManagingRules(true)}>
                        Rules
                    </button>
                    <button className="sync-btn" disabled={!selectedAccount} onClick={() => {
                        setComposeInitial(undefined)
                        setComposing(true)
                    }}>
                        Compose
                    </button>
                    <button className="sync-btn" disabled={!selectedAccount || syncing} onClick={() => void sync()}>
                        {syncing ? 'Syncing...' : 'Sync'}
                    </button>
                    {outboxCount > 0 && (
                        <span className="outbox-pill" title="Messages queued while offline; they send on the next sync">
                            {outboxCount} queued
                        </span>
                    )}
                    <MenuBar
                        theme={theme}
                        onToggleTheme={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
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
            <div className="panes">
                <Sidebar
                    accounts={accounts}
                    selectedAccount={selectedAccount}
                    folders={folders}
                    selectedFolder={selectedFolder}
                    onSelectAccount={(id) => void selectAccount(id)}
                    onSelectFolder={(id) => void selectFolder(id)}
                    onEditAccount={(account) => setAccountToEdit(account)}
                    onDeleteAccount={(account) => setAccountToDelete(account)}
                    onNewFolder={() => setFolderPrompt({mode: 'create'})}
                    onRenameFolder={(folder) => setFolderPrompt({mode: 'rename', folder})}
                    onDeleteFolder={(folder) => setFolderToDelete(folder)}
                />
                <MessageList
                    messages={searchActive ? searchResults : messages}
                    selectedMessage={selectedMessage}
                    folderSelected={Boolean(selectedFolder)}
                    searchQuery={searchQuery}
                    searchActive={searchActive}
                    onSearchChange={setSearchQuery}
                    onSelectMessage={setSelectedMessage}
                    onToggleFlag={(m) => void toggleFlag(m)}
                    onContextMenu={openContextMenu}
                    onOpenInNewTab={openInNewTab}
                />
                <Reader
                    message={selectedMessage}
                    onToggleRead={(m) => void toggleRead(m)}
                    onReply={openReply}
                    onReplyAll={openReplyAll}
                    onForward={openForward}
                    onDelete={(m) => setMessageToDelete(m)}
                    folders={folders}
                    onMove={(m, dest) => void moveMessage(m, dest)}
                    onCopy={(m, dest) => void copyMessage(m, dest)}
                    tags={tags}
                    messageTags={messageTags}
                    onToggleTag={(tagId, assigned) => void toggleTag(tagId, assigned)}
                    body={messageBody}
                    bodyLoading={bodyLoading}
                    tabs={tabs}
                    onSelectTab={setSelectedMessage}
                    onCloseTab={closeTab}
                />
            </div>
            )}
            <AboutModal about={about} onClose={() => setAbout(null)}/>
            <LicenceModal text={licence} onClose={() => setLicence(null)}/>
            {composing && selectedAccount && (
                <ComposeModal
                    accountId={selectedAccount}
                    initial={composeInitial}
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
            {managingTags && (
                <TagManagerModal tags={tags} onChanged={() => void onTagsChanged()} onClose={() => setManagingTags(false)}/>
            )}
            {managingRules && (
                <RuleManagerModal rules={rules} onChanged={() => void loadRules()} onClose={() => setManagingRules(false)}/>
            )}
            {messageToDelete && (
                <ConfirmDialog
                    title="Delete message"
                    message={`Delete "${messageToDelete.subject || '(no subject)'}"? It is moved to Trash, or deleted permanently if it is already in Trash or the account has no Trash folder.`}
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
                    onToggleRead={(m) => void toggleRead(m)}
                    onToggleFlag={(m) => void toggleFlag(m)}
                    onMove={(m, dest) => void moveMessage(m, dest)}
                    onCopy={(m, dest) => void copyMessage(m, dest)}
                    onSetTag={(id, tagId, assigned) => void setMessageTagById(id, tagId, assigned)}
                    onOpenInNewTab={openInNewTab}
                    onSaveAs={(m) => void saveMessageAs(m)}
                    onPrint={(m) => void printMessage(m)}
                    onAttachToNew={attachToNewMessage}
                    onDelete={requestDelete}
                    onDeletePermanent={(m) => setMessageToPurge(m)}
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
