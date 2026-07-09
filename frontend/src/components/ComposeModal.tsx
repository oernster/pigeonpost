import {useEffect, useRef, useState} from 'react'
import {useBackdropDismiss} from './useBackdropDismiss'
import {EditorContent, useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import {api, ComposeInput} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'

// autosaveDelayMs debounces local draft-recovery autosaves, so a snapshot is written a short pause after
// the user stops typing rather than on every keystroke.
const autosaveDelayMs = 1500

// normaliseUrl gives a bare host a scheme so the link is absolute rather than treated as relative.
function normaliseUrl(url: string): string {
    const trimmed = url.trim()
    if (trimmed === '' || /^(https?:|mailto:)/i.test(trimmed)) {
        return trimmed
    }
    return `https://${trimmed}`
}

// ComposeInitial pre-fills the compose window, used by reply, reply-all and forward.
// MessageAttachment is an existing email attached to a new message: its id (fetched and rendered as a
// message/rfc822 part at send time) and a display name for its chip.
export interface MessageAttachment {
    id: string
    name: string
}

export interface ComposeInitial {
    // from preselects the sender address (used by reply to send as the address the message was delivered
    // to). Empty falls back to the account's primary address.
    from?: string
    to?: string
    cc?: string
    bcc?: string
    subject?: string
    bodyHtml?: string
    messageAttachments?: MessageAttachment[]
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
    onClose: () => void
}

function splitAddresses(value: string): string[] {
    return value.split(',').map((part) => part.trim()).filter(Boolean)
}

// basename returns the final path segment of a file path, handling both Windows and POSIX separators,
// so an attachment chip shows the filename rather than the full path.
function basename(path: string): string {
    const parts = path.split(/[\\/]/)
    return parts[parts.length - 1] || path
}

export function ComposeModal({accountId, senders, initial, canSaveDraft, onClose}: ComposeModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    // The chosen From address. It defaults to the reply's delivered-to address when given, otherwise the
    // account's primary (first) sender. The backend validates it against the account's owned addresses.
    const [from, setFrom] = useState(initial?.from || senders[0]?.address || '')
    const [to, setTo] = useState(initial?.to ?? '')
    const [cc, setCc] = useState(initial?.cc ?? '')
    const [bcc, setBcc] = useState(initial?.bcc ?? '')
    const [subject, setSubject] = useState(initial?.subject ?? '')
    const [attachments, setAttachments] = useState<string[]>([])
    const [messageAttachments, setMessageAttachments] = useState<MessageAttachment[]>(initial?.messageAttachments ?? [])
    const [sending, setSending] = useState(false)
    const [savingDraft, setSavingDraft] = useState(false)
    const [error, setError] = useState('')
    const [linkOpen, setLinkOpen] = useState(false)
    const [linkUrl, setLinkUrl] = useState('')

    // attemptSendRef lets the editor's key handler call the latest attemptSend without recreating the
    // editor: the editor is built once, but attemptSend closes over state that changes each render.
    const attemptSendRef = useRef<() => void>(() => {})

    // bodyTick bumps on each editor edit so the autosave effect re-runs; dirtyRef gates it to real user
    // edits so an untouched reply or forward template is never snapshotted; stopAutosaveRef halts it once
    // the message has been sent or saved to the server, so a pending debounce cannot re-save a stale copy.
    const [bodyTick, setBodyTick] = useState(0)
    const dirtyRef = useRef(false)
    const stopAutosaveRef = useRef(false)
    const markDirty = () => {
        dirtyRef.current = true
    }

    const editor = useEditor({
        extensions: [StarterKit, Link.configure({openOnClick: false, autolink: true, linkOnPaste: true})],
        content: initial?.bodyHtml ?? '',
        onUpdate: () => {
            markDirty()
            setBodyTick((tick) => tick + 1)
        },
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
        },
    })

    const openLinkEditor = () => {
        setLinkUrl((editor?.getAttributes('link').href as string) ?? '')
        setLinkOpen(true)
    }

    const applyLink = () => {
        const href = normaliseUrl(linkUrl)
        if (href === '') {
            editor?.chain().focus().extendMarkRange('link').unsetLink().run()
        } else {
            editor?.chain().focus().extendMarkRange('link').setLink({href}).run()
        }
        setLinkOpen(false)
        setLinkUrl('')
    }

    const removeLink = () => {
        editor?.chain().focus().extendMarkRange('link').unsetLink().run()
        setLinkOpen(false)
        setLinkUrl('')
    }

    const buildRequest = (): ComposeInput => {
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
            attachmentMessageIds: messageAttachments.map((m) => m.id),
        }
    }

    // Autosave the in-progress compose to the local recovery slot, debounced, so an accidental close or a
    // crash does not lose it. This is local only and never touches the server; sending or saving a server
    // draft clears it. It runs only after a real edit (dirtyRef), so an untouched template is not stored,
    // and clears the slot when the compose is emptied back out.
    useEffect(() => {
        if (!dirtyRef.current || stopAutosaveRef.current) return
        const bodyText = editor?.getText() ?? ''
        const hasContent = to.trim() !== '' || cc.trim() !== '' || bcc.trim() !== '' ||
            subject.trim() !== '' || bodyText.trim() !== ''
        const timer = window.setTimeout(() => {
            if (stopAutosaveRef.current) return
            if (!hasContent) {
                void api.clearDraftRecovery()
                return
            }
            void api.saveDraftRecovery({
                accountId,
                to,
                cc,
                bcc,
                subject,
                bodyHtml: editor?.getHTML() ?? '',
            })
        }, autosaveDelayMs)
        return () => window.clearTimeout(timer)
    }, [accountId, to, cc, bcc, subject, bodyTick, editor])

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

    const send = async () => {
        stopAutosaveRef.current = true
        setSending(true)
        setError('')
        try {
            await api.send(buildRequest())
            void api.clearDraftRecovery()
            onClose()
        } catch (e) {
            stopAutosaveRef.current = false
            setError(String(e))
            setSending(false)
        }
    }

    const [attachWarn, setAttachWarn] = useState(false)

    // canSend mirrors the Send button's enabled state, so Ctrl+Enter behaves exactly like the button.
    const canSend = () => !sending && !savingDraft && to.trim() !== ''
    const hasAttachments = () => attachments.length > 0 || messageAttachments.length > 0
    // mentionsAttachment matches the whole words "attach" or "attached" in the subject or body, so a
    // message that talks about attaching something can prompt a reminder before it is sent.
    const mentionsAttachment = () => /\battach(ed)?\b/i.test(subject + ' ' + (editor?.getText() ?? ''))

    // attemptSend is the single entry point for both the Send button and Ctrl+Enter. It warns once when
    // the message mentions an attachment but none is attached, otherwise it sends straight away.
    const attemptSend = () => {
        if (!canSend()) return
        if (mentionsAttachment() && !hasAttachments()) {
            setAttachWarn(true)
            return
        }
        void send()
    }
    attemptSendRef.current = attemptSend

    const saveDraft = async () => {
        stopAutosaveRef.current = true
        setSavingDraft(true)
        setError('')
        try {
            await api.saveDraft(buildRequest())
            void api.clearDraftRecovery()
            onClose()
        } catch (e) {
            stopAutosaveRef.current = false
            setError(String(e))
            setSavingDraft(false)
        }
    }

    const btn = (active: boolean, label: string, title: string, onClick: () => void) => (
        <button
            type="button"
            className={'compose-tool' + (active ? ' active' : '')}
            title={title}
            aria-label={title}
            aria-pressed={active}
            onMouseDown={(e) => e.preventDefault()}
            onClick={onClick}
        >
            {label}
        </button>
    )

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal compose" role="dialog" aria-label="New message" onClick={(e) => e.stopPropagation()}
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
                        markDirty()
                        setTo(e.target.value)
                    }} autoFocus
                           placeholder="name@example.com, other@example.com"/>
                </label>
                <label className="field">
                    <span>Cc</span>
                    <input value={cc} onChange={(e) => {
                        markDirty()
                        setCc(e.target.value)
                    }}/>
                </label>
                <label className="field">
                    <span>Bcc</span>
                    <input value={bcc} onChange={(e) => {
                        markDirty()
                        setBcc(e.target.value)
                    }}/>
                </label>
                <label className="field">
                    <span>Subject</span>
                    <input value={subject} onChange={(e) => {
                        markDirty()
                        setSubject(e.target.value)
                    }}/>
                </label>

                <div className="compose-toolbar">
                    {btn(editor?.isActive('bold') ?? false, 'B', 'Bold', () => editor?.chain().focus().toggleBold().run())}
                    {btn(editor?.isActive('italic') ?? false, 'I', 'Italic', () => editor?.chain().focus().toggleItalic().run())}
                    {btn(editor?.isActive('strike') ?? false, 'S', 'Strikethrough', () => editor?.chain().focus().toggleStrike().run())}
                    <span className="compose-tool-sep"/>
                    {btn(editor?.isActive('heading', {level: 2}) ?? false, 'H', 'Heading', () => editor?.chain().focus().toggleHeading({level: 2}).run())}
                    {btn(editor?.isActive('bulletList') ?? false, '•', 'Bullet list', () => editor?.chain().focus().toggleBulletList().run())}
                    {btn(editor?.isActive('orderedList') ?? false, '1.', 'Numbered list', () => editor?.chain().focus().toggleOrderedList().run())}
                    {btn(editor?.isActive('blockquote') ?? false, '”', 'Quote', () => editor?.chain().focus().toggleBlockquote().run())}
                    <span className="compose-tool-sep"/>
                    {btn(editor?.isActive('link') ?? false, '🔗', 'Link', openLinkEditor)}
                </div>
                {linkOpen && (
                    <div className="compose-link-row">
                        <input
                            className="tag-name-input"
                            value={linkUrl}
                            autoFocus
                            placeholder="https://example.com"
                            onChange={(e) => setLinkUrl(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter') {
                                    e.preventDefault()
                                    applyLink()
                                }
                            }}
                        />
                        <button className="btn primary" onClick={applyLink}>Apply</button>
                        <button className="btn" onClick={removeLink}>Remove</button>
                    </div>
                )}
                <EditorContent editor={editor} className="compose-editor"/>

                <div className="compose-attachments">
                    <button type="button" className="btn" onClick={() => void addAttachments()}>
                        Attach files
                    </button>
                    {(attachments.length > 0 || messageAttachments.length > 0) && (
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

                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose} disabled={sending || savingDraft}>Cancel</button>
                    <div className="compose-send-group">
                        {canSaveDraft && (
                            <button className="btn" onClick={() => void saveDraft()} disabled={sending || savingDraft}>
                                {savingDraft ? 'Saving...' : 'Save draft'}
                            </button>
                        )}
                        <button className="btn primary" onClick={attemptSend} disabled={sending || savingDraft || to.trim() === ''}
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
                        void send()
                    }}
                    onCancel={() => setAttachWarn(false)}
                />
            )}
        </div>
    )
}
