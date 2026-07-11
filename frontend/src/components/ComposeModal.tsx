import {useRef, useState} from 'react'
import {useBackdropDismiss} from './useBackdropDismiss'
import {EditorContent, useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import {api, ComposeInput, Template} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {basename, isValidAddress, normaliseUrl, splitAddresses} from '../composeAddresses'
import {useLinkEditor} from '../hooks/useLinkEditor'
import {useDraftAutosave} from '../hooks/useDraftAutosave'
import {useSeparatorCorrection} from '../hooks/useSeparatorCorrection'

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
    // attachmentPaths pre-attaches files by path, used when the Attach button picks files before opening a
    // fresh compose so the chosen files are already attached.
    attachmentPaths?: string[]
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


export function ComposeModal({accountId, senders, initial, canSaveDraft, onClose}: ComposeModalProps) {
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

    const editor = useEditor({
        extensions: [StarterKit, Link.configure({openOnClick: false, autolink: true, linkOnPaste: true})],
        content: initial?.bodyHtml ?? '',
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
        autosave.stopAutosave()
        setSending(true)
        setError('')
        try {
            await api.send(buildRequest())
            void api.clearDraftRecovery()
            onClose()
        } catch (e) {
            autosave.resumeAutosave()
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

    // attemptSend is the single entry point for both the Send button and Ctrl+Enter. It offers to fix a wrong
    // address separator, then warns once when the message mentions an attachment but none is attached,
    // otherwise it sends straight away.
    const attemptSend = () => {
        if (!canSend()) return
        if (correction.offer()) return
        if (mentionsAttachment() && !hasAttachments()) {
            setAttachWarn(true)
            return
        }
        void send()
    }
    attemptSendRef.current = attemptSend

    const saveDraft = async () => {
        autosave.stopAutosave()
        setSavingDraft(true)
        setError('')
        try {
            await api.saveDraft(buildRequest())
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
                    }} autoFocus
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
                    {btn(editor?.isActive('link') ?? false, '🔗', 'Link', link.openLink)}
                    <span className="compose-tool-sep"/>
                    <button
                        type="button"
                        className={'compose-tool' + (templatePicker ? ' active' : '')}
                        title="Insert template"
                        aria-label="Insert template"
                        aria-pressed={templatePicker}
                        aria-haspopup="menu"
                        aria-expanded={templatePicker}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => void openTemplatePicker()}
                    >
                        Template
                    </button>
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
