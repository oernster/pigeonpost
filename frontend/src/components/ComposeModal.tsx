import {Fragment, useRef, useState} from 'react'
import {useBackdropDismiss} from './useBackdropDismiss'
import {EditorContent, useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import Image from '@tiptap/extension-image'
import {api, ComposeInput, Template} from '../api'
import {DataAttachment} from '../composeIntake'
import {useComposeIntake} from '../hooks/useComposeIntake'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {basename, isValidAddress, normaliseUrl, splitAddresses} from '../composeAddresses'
import {useLinkEditor} from '../hooks/useLinkEditor'
import {useDraftAutosave} from '../hooks/useDraftAutosave'
import {useSeparatorCorrection} from '../hooks/useSeparatorCorrection'
import {bodyMentionsAttachment} from '../composeAttachment'
import {fromDatetimeLocal, isSchedulable, sendLaterChoices} from '../schedule'
import {ToolButton} from './ToolButton'
import {useToolbarNav} from '../hooks/useToolbarNav'

// ComposeTool is one entry in the formatting strip: its button face, its editor action and its
// place in the strip's visual grouping.
interface ComposeTool {
    glyph: string
    name: string
    shortcut?: string
    active: boolean
    run: () => void
    sepAfter?: boolean
    hasPopup?: boolean
}

// ComposeInitial pre-fills the compose window, used by reply, reply-all and forward.
// MessageAttachment is an existing email attached to a new message: its id (fetched and rendered as a
// message/rfc822 part at send time) and a display name for its chip.
export interface MessageAttachment {
    id: string
    name: string
}

export interface ComposeInitial {
    // accountId names the account this compose sends from when it is not the selected one: a reply or
    // forward opened from a unified-mailbox row must send from the row's own account. Unset falls back
    // to the selected account.
    accountId?: string
    // from preselects the sender address (used by reply to send as the address the message was delivered
    // to). Empty falls back to the account's primary address.
    from?: string
    to?: string
    cc?: string
    bcc?: string
    subject?: string
    bodyHtml?: string
    messageAttachments?: MessageAttachment[]
    // attachmentPaths pre-attaches files by path, used when the Attach button picks files before opening a
    // fresh compose so the chosen files are already attached.
    attachmentPaths?: string[]
    // attachmentData pre-attaches in-memory files (pasted or dropped, so the webview holds bytes with no
    // path), used when an undone send reopens the compose exactly as it was.
    attachmentData?: DataAttachment[]
    // inReplyToId and replyKind mark this compose as a reply or forward of an existing message, so once it is
    // sent the original can be flagged \Answered (reply / reply-all) or $Forwarded (forward). Both are unset
    // for a fresh compose, a restored draft or an attach-to-new-message.
    inReplyToId?: string
    replyKind?: 'reply' | 'forward'
}

// Sender is one address the account may send from, offered in the From dropdown.
interface Sender {
    name: string
    address: string
}

interface ComposeModalProps {
    accountId: string
    // senders are the account's sendable addresses (primary first, then identities). When there is more
    // than one, the compose window shows a From dropdown.
    senders: Sender[]
    initial?: ComposeInitial
    // canSaveDraft is false for POP3 accounts, which have no server-side Drafts mailbox to append to.
    canSaveDraft: boolean
    // onMarkReplied / onMarkForwarded record a sent reply or forward on its original message (by id), so the
    // row shows the replied / forwarded glyph at once. They own the server flag, the local cache and the
    // in-memory list update; the composer just reports which original was acted on. Called best-effort after a
    // successful send, so a failure never disrupts the send. For a held send that marking moves to the
    // undo toast's expiry (via onHeld), so undoing a reply never leaves a wrong answered flag.
    onMarkReplied: (id: string) => void
    onMarkForwarded: (id: string) => void
    // holdSeconds is the user's undo-send window, passed through to the send request; zero sends
    // immediately. onHeld reports a held send: the queued item's id (for Undo) and the full compose
    // state, so an undone send reopens exactly as it was.
    holdSeconds: number
    onHeld: (outboxId: string, reopen: ComposeInitial) => void
    onClose: () => void
}


export function ComposeModal({accountId, senders, initial, canSaveDraft, onMarkReplied, onMarkForwarded, holdSeconds, onHeld, onClose}: ComposeModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    // The chosen From address. It defaults to the reply's delivered-to address when given, otherwise the
    // account's primary (first) sender. The backend validates it against the account's owned addresses.
    const [from, setFrom] = useState(initial?.from || senders[0]?.address || '')
    const [to, setTo] = useState(initial?.to ?? '')
    const [cc, setCc] = useState(initial?.cc ?? '')
    const [bcc, setBcc] = useState(initial?.bcc ?? '')
    const [subject, setSubject] = useState(initial?.subject ?? '')
    const [attachments, setAttachments] = useState<string[]>(initial?.attachmentPaths ?? [])
    const [messageAttachments, setMessageAttachments] = useState<MessageAttachment[]>(initial?.messageAttachments ?? [])
    const [sending, setSending] = useState(false)
    const [savingDraft, setSavingDraft] = useState(false)
    const [error, setError] = useState('')
    // The message-template picker: the loaded templates and whether its dropdown is open. Templates are
    // fetched the first time the picker is opened so an unused compose window makes no call.
    const [templates, setTemplates] = useState<Template[]>([])
    const [templatePicker, setTemplatePicker] = useState(false)

    // attemptSendRef lets the editor's key handler call the latest attemptSend without recreating the
    // editor: the editor is built once, but attemptSend closes over state that changes each render.
    const attemptSendRef = useRef<() => void>(() => {})
    // noteEditRef bridges the editor's onUpdate to the autosave (created below), for the same reason: the
    // editor is built once, but the autosave's noteEdit is recreated each render.
    const noteEditRef = useRef<() => void>(() => {})
    // intakeRef bridges the editor's paste and drop handlers to the latest intakeFiles, again because the
    // editor is built once while intakeFiles closes over per-render state.
    const intakeRef = useRef<(dt: DataTransfer | null) => boolean>(() => false)

    const editor = useEditor({
        extensions: [
            StarterKit,
            Link.configure({openOnClick: false, autolink: true, linkOnPaste: true}),
            // allowBase64 keeps a pasted image's data: URI in the document; the backend lifts it into a
            // proper inline MIME part at send time.
            Image.configure({allowBase64: true}),
        ],
        content: initial?.bodyHtml ?? '',
        // Initial focus is the top of the body, not the To field: a reply, reply-all or forward opens
        // ready to type above the quoted text, and a fresh compose reaches To with one Shift+Tab.
        autofocus: 'start',
        onUpdate: () => noteEditRef.current(),
        editorProps: {
            // Ctrl+Enter (Cmd+Enter on macOS) sends. Returning true stops TipTap from also inserting its
            // default Mod-Enter hard break.
            handleKeyDown: (_view, event) => {
                if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                    event.preventDefault()
                    attemptSendRef.current()
                    return true
                }
                return false
            },
            // Pasted or dropped files follow the intake rule: images embed at the cursor, other files
            // attach. Returning true stops ProseMirror's own handling for the ones taken.
            handlePaste: (_view, event) => intakeRef.current(event.clipboardData),
            handleDrop: (_view, event) => intakeRef.current(event.dataTransfer),
        },
    })

    // The local draft autosave watches the recipient fields and the editor; noteEdit is the editor's
    // onUpdate, bridged through noteEditRef because the editor is built once (above) before this runs.
    const autosave = useDraftAutosave({accountId, to, cc, bcc, subject, editor})
    noteEditRef.current = autosave.noteEdit

    // The inline link-editing row is the shared hook, seeded from the current selection's link.
    const link = useLinkEditor(editor, normaliseUrl)

    // The separator-correction safeguard offers to fix a wrong address separator on a send attempt. It reads
    // the recipient fields; when the fix is approved it writes the corrected addresses back and marks the
    // draft dirty.
    const correction = useSeparatorCorrection({
        to, cc, bcc, setTo, setCc, setBcc, setError, markDirty: autosave.markDirty,
    })

    // The paste and drop intake (images embed, other files attach) is the shared hook; its take is
    // bridged through intakeRef because the editor is built once (above) before this runs.
    const intake = useComposeIntake({
        editor, setError, markDirty: autosave.markDirty, initial: initial?.attachmentData,
    })
    intakeRef.current = intake.take

    // buildRequest packs the compose state for the backend. at is the send-later instant (null for an
    // immediate or undo-held send); it takes precedence over the undo window server-side.
    const buildRequest = (at: Date | null = null): ComposeInput => {
        const text = editor?.getText() ?? ''
        const html = editor?.getHTML() ?? ''
        return {
            accountId,
            from,
            to: splitAddresses(to),
            cc: splitAddresses(cc),
            bcc: splitAddresses(bcc),
            subject,
            body: text,
            // Only carry an HTML alternative when the body is non-empty, so an empty message stays plain.
            htmlBody: text.trim() === '' ? '' : html,
            attachmentPaths: attachments,
            attachmentData: intake.dataAttachments,
            attachmentMessageIds: messageAttachments.map((m) => m.id),
            holdSeconds,
            sendAtMs: at === null ? 0 : at.getTime(),
        }
    }

    // reopenInitial captures the whole compose state, so an undone send reopens exactly as it was,
    // including the reply/forward marking intent the toast applies once the window elapses.
    const reopenInitial = (): ComposeInitial => ({
        accountId,
        from,
        to,
        cc,
        bcc,
        subject,
        bodyHtml: editor?.getHTML() ?? '',
        attachmentPaths: attachments,
        attachmentData: intake.dataAttachments,
        messageAttachments,
        inReplyToId: initial?.inReplyToId,
        replyKind: initial?.replyKind,
    })

    const removeMessageAttachment = (id: string) => {
        setMessageAttachments((prev) => prev.filter((m) => m.id !== id))
    }

    // addAttachments opens the native file picker and appends the chosen files, skipping any already
    // attached so the same file is not added twice.
    const addAttachments = async () => {
        try {
            const picked = await api.pickAttachments()
            if (picked.length > 0) {
                setAttachments((prev) => [...prev, ...picked.filter((p) => !prev.includes(p))])
            }
        } catch (e) {
            setError(String(e))
        }
    }

    const removeAttachment = (path: string) => {
        setAttachments((prev) => prev.filter((p) => p !== path))
    }

    // markOriginalOnSend reports a sent reply or forward on its original message so the row shows the
    // replied/forwarded indicator. The handlers own the server flag, the local cache and the in-memory list
    // update; this only says which original was acted on. It is fire-and-forget: it never blocks or fails the
    // send, so composing offline just leaves the indicator for the next sync.
    const markOriginalOnSend = () => {
        if (!initial?.inReplyToId) {
            return
        }
        if (initial.replyKind === 'reply') {
            onMarkReplied(initial.inReplyToId)
        } else if (initial.replyKind === 'forward') {
            onMarkForwarded(initial.inReplyToId)
        }
    }

    // send delivers the message now (at null) or schedules it for the chosen instant. A scheduled send
    // waits in the Outbox with Cancel send: it shows no undo toast, and it does not mark a reply or
    // forward's original (a schedule cancelled days later must not have already flagged it; the glyph is
    // an accepted gap for scheduled sends).
    const send = async (at: Date | null) => {
        autosave.stopAutosave()
        setSending(true)
        setError('')
        try {
            const outboxId = await api.send(buildRequest(at))
            if (at === null) {
                if (outboxId === '') {
                    // Sent immediately: mark the original now, exactly as before undo-send existed.
                    markOriginalOnSend()
                } else {
                    // Held behind the undo window: hand the queued id and the compose state to the toast,
                    // which marks the original only if the window elapses without an undo.
                    onHeld(outboxId, reopenInitial())
                }
            }
            void api.clearDraftRecovery()
            onClose()
        } catch (e) {
            autosave.resumeAutosave()
            setError(String(e))
            setSending(false)
        }
    }

    const [attachWarn, setAttachWarn] = useState(false)
    // sendLaterOpen shows the schedule row; sendAtValue is its datetime-local field. scheduleAtRef
    // carries a chosen moment through the attachment-reminder dialog, so "Send anyway" schedules
    // rather than sending now.
    const [sendLaterOpen, setSendLaterOpen] = useState(false)
    const [sendAtValue, setSendAtValue] = useState('')
    const scheduleAtRef = useRef<Date | null>(null)

    // canSend mirrors the Send button's enabled state, so Ctrl+Enter behaves exactly like the button.
    const canSend = () => !sending && !savingDraft && to.trim() !== ''
    const hasAttachments = () =>
        attachments.length > 0 || intake.dataAttachments.length > 0 || messageAttachments.length > 0
    // mentionsAttachment reports whether the message the user actually wrote talks about attaching
    // something, so it can prompt a reminder before sending. It passes the editor HTML (not its plain
    // text) so bodyMentionsAttachment can strip the quoted reply or forward chain: an attachment mentioned
    // only earlier in the thread must not trigger the reminder.
    const mentionsAttachment = () => bodyMentionsAttachment(subject, editor?.getHTML() ?? '')

    // attemptSend is the single entry point for the Send button, Ctrl+Enter and the send-later choices
    // (which pass their instant). It offers to fix a wrong address separator, then warns once when the
    // message mentions an attachment but none is attached, otherwise it sends or schedules straight away.
    const attemptSend = (at: Date | null = null) => {
        if (!canSend()) return
        if (correction.offer()) return
        if (mentionsAttachment() && !hasAttachments()) {
            scheduleAtRef.current = at
            setAttachWarn(true)
            return
        }
        void send(at)
    }
    attemptSendRef.current = () => attemptSend()

    const saveDraft = async () => {
        autosave.stopAutosave()
        setSavingDraft(true)
        setError('')
        try {
            await api.saveDraft(buildRequest(null))
            void api.clearDraftRecovery()
            onClose()
        } catch (e) {
            autosave.resumeAutosave()
            setError(String(e))
            setSavingDraft(false)
        }
    }

    // openTemplatePicker toggles the template dropdown, loading the templates on first open so the list is
    // current without fetching until it is wanted.
    const openTemplatePicker = async () => {
        if (templatePicker) {
            setTemplatePicker(false)
            return
        }
        try {
            setTemplates(await api.listTemplates())
            setTemplatePicker(true)
        } catch (e) {
            setError(String(e))
        }
    }

    // insertTemplate applies a chosen template: it fills the subject when it is still empty (so a template
    // never overwrites a subject already typed) and inserts the template body HTML at the cursor, then marks
    // the draft dirty so the change is autosaved.
    const insertTemplate = (t: Template) => {
        setTemplatePicker(false)
        if (subject.trim() === '' && t.subject !== '') {
            setSubject(t.subject)
        }
        if (t.body !== '') {
            editor?.chain().focus().insertContent(t.body).run()
        }
        autosave.markDirty()
    }

    // The formatting strip is one focus-ring stop (roving tabindex; see useToolbarNav): the tools
    // are data so the toolbar renders and navigates from one list. A separator follows the tools
    // that end a visual group.
    const tools: ComposeTool[] = [
        {glyph: 'B', name: 'Bold', shortcut: 'Ctrl+B', active: editor?.isActive('bold') ?? false, run: () => editor?.chain().focus().toggleBold().run()},
        {glyph: 'I', name: 'Italic', shortcut: 'Ctrl+I', active: editor?.isActive('italic') ?? false, run: () => editor?.chain().focus().toggleItalic().run()},
        {glyph: 'S', name: 'Strikethrough', shortcut: 'Ctrl+Shift+X', active: editor?.isActive('strike') ?? false, run: () => editor?.chain().focus().toggleStrike().run(), sepAfter: true},
        {glyph: 'H', name: 'Heading', shortcut: 'Ctrl+Alt+2', active: editor?.isActive('heading', {level: 2}) ?? false, run: () => editor?.chain().focus().toggleHeading({level: 2}).run()},
        {glyph: '•', name: 'Bullet list', shortcut: 'Ctrl+Shift+8', active: editor?.isActive('bulletList') ?? false, run: () => editor?.chain().focus().toggleBulletList().run()},
        {glyph: '1.', name: 'Numbered list', shortcut: 'Ctrl+Shift+7', active: editor?.isActive('orderedList') ?? false, run: () => editor?.chain().focus().toggleOrderedList().run()},
        {glyph: '”', name: 'Quote', shortcut: 'Ctrl+Shift+B', active: editor?.isActive('blockquote') ?? false, run: () => editor?.chain().focus().toggleBlockquote().run(), sepAfter: true},
        {glyph: '🔗', name: 'Link', active: editor?.isActive('link') ?? false, run: link.openLink, sepAfter: true},
        {glyph: 'Template', name: 'Insert template', active: templatePicker, run: () => void openTemplatePicker(), hasPopup: true},
    ]
    const toolbar = useToolbarNav(tools.length)

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal compose" role="dialog" aria-label="New message" onClick={(e) => e.stopPropagation()}
                 onDragOver={(e) => e.preventDefault()}
                 onDrop={(e) => {
                     // Files dropped anywhere on the compose window follow the intake rule (images
                     // embed, other files attach). A drop on the editor itself is handled by the
                     // editor's own handleDrop, so it is skipped here rather than taken twice.
                     if ((e.target as HTMLElement).closest('.ProseMirror')) {
                         return
                     }
                     e.preventDefault()
                     intakeRef.current(e.dataTransfer)
                 }}
                 onKeyDownCapture={(e) => {
                     // The editor handles Ctrl+Enter itself (see editorProps); this covers the
                     // To, Cc, Bcc and Subject fields, where there is no editor to intercept it.
                     if ((e.ctrlKey || e.metaKey) && e.key === 'Enter' &&
                         !(e.target as HTMLElement).closest('.ProseMirror')) {
                         e.preventDefault()
                         attemptSend()
                     }
                 }}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">New message</h2>
                {error && <div className="compose-error">{error}</div>}
                {correction.pending && (
                    <div className="compose-correction">
                        <div>Addresses should be separated by a comma or semicolon. Did you mean:</div>
                        <div className="compose-correction-value">{correction.pending.preview}</div>
                        <div className="compose-correction-actions">
                            <button type="button" className="btn" onClick={correction.apply}>Use this</button>
                            <button type="button" className="btn" onClick={correction.dismiss}>Dismiss</button>
                        </div>
                    </div>
                )}
                {senders.length > 1 && (
                    <label className="field">
                        <span>From</span>
                        <select value={from} onChange={(e) => setFrom(e.target.value)}>
                            {senders.map((s) => (
                                <option key={s.address} value={s.address}>
                                    {s.name ? `${s.name} <${s.address}>` : s.address}
                                </option>
                            ))}
                        </select>
                    </label>
                )}
                <label className="field">
                    <span>To</span>
                    <input value={to} onChange={(e) => {
                        autosave.markDirty()
                        setTo(e.target.value)
                    }}
                           placeholder="name@example.com, other@example.com"/>
                </label>
                <label className="field">
                    <span>Cc</span>
                    <input value={cc} onChange={(e) => {
                        autosave.markDirty()
                        setCc(e.target.value)
                    }}/>
                </label>
                <label className="field">
                    <span>Bcc</span>
                    <input value={bcc} onChange={(e) => {
                        autosave.markDirty()
                        setBcc(e.target.value)
                    }}/>
                </label>
                <label className="field">
                    <span>Subject</span>
                    <input value={subject} onChange={(e) => {
                        autosave.markDirty()
                        setSubject(e.target.value)
                    }}/>
                </label>

                <div className="compose-toolbar" aria-label="Formatting" {...toolbar.toolbarProps}>
                    {tools.map((tool, index) => (
                        <Fragment key={tool.name}>
                            <ToolButton
                                active={tool.active}
                                glyph={tool.glyph}
                                name={tool.name}
                                shortcut={tool.shortcut}
                                tabIndex={toolbar.toolTabIndex(index)}
                                onActivate={tool.run}
                                hasPopup={tool.hasPopup}
                                expanded={tool.hasPopup ? templatePicker : undefined}
                            />
                            {tool.sepAfter && <span className="compose-tool-sep"/>}
                        </Fragment>
                    ))}
                </div>
                {templatePicker && (
                    <div className="compose-template-picker" role="menu" aria-label="Message templates">
                        {templates.length === 0 ? (
                            <div className="compose-template-empty">No templates yet.</div>
                        ) : (
                            templates.map((t) => (
                                <button
                                    key={t.id}
                                    type="button"
                                    role="menuitem"
                                    className="compose-template-option"
                                    onMouseDown={(e) => e.preventDefault()}
                                    onClick={() => insertTemplate(t)}
                                >
                                    <span className="compose-template-name">{t.name}</span>
                                    <span className="compose-template-subject">{t.subject || '(no subject)'}</span>
                                </button>
                            ))
                        )}
                    </div>
                )}
                {link.open && (
                    <div className="compose-link-row">
                        <input
                            className="tag-name-input"
                            value={link.url}
                            autoFocus
                            placeholder="https://example.com"
                            onChange={(e) => link.setUrl(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter') {
                                    e.preventDefault()
                                    link.applyLink()
                                }
                            }}
                        />
                        <button className="btn primary" onClick={link.applyLink}>Apply</button>
                        <button className="btn" onClick={link.removeLink}>Remove</button>
                    </div>
                )}
                <EditorContent editor={editor} className="compose-editor"/>

                <div className="compose-attachments">
                    <button type="button" className="btn" onClick={() => void addAttachments()}>
                        Attach files
                    </button>
                    {hasAttachments() && (
                        <ul className="attachment-list">
                            {attachments.map((path) => (
                                <li key={path} className="attachment-chip">
                                    <span className="attachment-name" title={path}>{basename(path)}</span>
                                    <button
                                        type="button"
                                        className="attachment-remove"
                                        aria-label={`Remove ${basename(path)}`}
                                        onClick={() => removeAttachment(path)}
                                    >
                                        &times;
                                    </button>
                                </li>
                            ))}
                            {intake.dataAttachments.map((a, index) => (
                                <li key={`${index}-${a.name}`} className="attachment-chip">
                                    <span className="attachment-name" title={a.name}>{a.name}</span>
                                    <button
                                        type="button"
                                        className="attachment-remove"
                                        aria-label={`Remove ${a.name}`}
                                        onClick={() => intake.remove(index)}
                                    >
                                        &times;
                                    </button>
                                </li>
                            ))}
                            {messageAttachments.map((m) => (
                                <li key={m.id} className="attachment-chip">
                                    <span className="attachment-icon" aria-hidden="true">{'✉'}</span>
                                    <span className="attachment-name" title={m.name}>{m.name}</span>
                                    <button
                                        type="button"
                                        className="attachment-remove"
                                        aria-label={`Remove ${m.name}`}
                                        onClick={() => removeMessageAttachment(m.id)}
                                    >
                                        &times;
                                    </button>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>

                {sendLaterOpen && (
                    <div className="compose-schedule-row" role="menu" aria-label="Send later">
                        {sendLaterChoices(new Date()).map((choice) => (
                            <button
                                key={choice.label}
                                type="button"
                                role="menuitem"
                                className="btn"
                                onClick={() => {
                                    setSendLaterOpen(false)
                                    attemptSend(choice.at)
                                }}
                            >
                                {choice.label}
                            </button>
                        ))}
                        <input
                            type="datetime-local"
                            className="compose-schedule-input"
                            aria-label="Send at"
                            value={sendAtValue}
                            onChange={(e) => setSendAtValue(e.target.value)}
                        />
                        <button
                            type="button"
                            className="btn primary"
                            disabled={!isSchedulable(fromDatetimeLocal(sendAtValue), new Date())}
                            onClick={() => {
                                const at = fromDatetimeLocal(sendAtValue)
                                if (at) {
                                    setSendLaterOpen(false)
                                    attemptSend(at)
                                }
                            }}
                        >
                            Schedule
                        </button>
                        <div className="compose-schedule-note">
                            Sends at the chosen time while PigeonPost is running, or at the next launch after it.
                            Cancel any time from the Outbox.
                        </div>
                    </div>
                )}
                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose} disabled={sending || savingDraft}>Cancel</button>
                    <div className="compose-send-group">
                        {canSaveDraft && (
                            <button className="btn" onClick={() => void saveDraft()} disabled={sending || savingDraft}>
                                {savingDraft ? 'Saving...' : 'Save draft'}
                            </button>
                        )}
                        <button
                            className="btn"
                            onClick={() => setSendLaterOpen((open) => !open)}
                            disabled={sending || savingDraft || to.trim() === ''}
                            aria-haspopup="menu"
                            aria-expanded={sendLaterOpen}
                            title="Send at a chosen time"
                        >
                            Send later
                        </button>
                        <button className="btn primary" onClick={() => attemptSend()} disabled={sending || savingDraft || to.trim() === ''}
                                title="Send (Ctrl+Enter)">
                            {sending ? 'Sending...' : 'Send'}
                        </button>
                    </div>
                </div>
            </div>

            {attachWarn && (
                <ConfirmDialog
                    title="Attachment reminder"
                    message="Did you want to attach anything before sending?"
                    confirmLabel="Send anyway"
                    busy={sending}
                    onConfirm={() => {
                        setAttachWarn(false)
                        void send(scheduleAtRef.current)
                    }}
                    onCancel={() => setAttachWarn(false)}
                />
            )}
        </div>
    )
}
