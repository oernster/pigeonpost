import {useState} from 'react'
import {EditorContent, useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import {api, ComposeInput} from '../api'
import {ModalClose} from './ModalClose'

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
    to?: string
    cc?: string
    bcc?: string
    subject?: string
    bodyHtml?: string
    messageAttachments?: MessageAttachment[]
}

interface ComposeModalProps {
    accountId: string
    initial?: ComposeInitial
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

export function ComposeModal({accountId, initial, onClose}: ComposeModalProps) {
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

    const editor = useEditor({
        extensions: [StarterKit, Link.configure({openOnClick: false, autolink: true, linkOnPaste: true})],
        content: initial?.bodyHtml ?? '',
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
        setSending(true)
        setError('')
        try {
            await api.send(buildRequest())
            onClose()
        } catch (e) {
            setError(String(e))
            setSending(false)
        }
    }

    const saveDraft = async () => {
        setSavingDraft(true)
        setError('')
        try {
            await api.saveDraft(buildRequest())
            onClose()
        } catch (e) {
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
        <div className="modal-backdrop" onClick={onClose}>
            <div className="modal compose" role="dialog" aria-label="New message" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">New message</h2>
                {error && <div className="compose-error">{error}</div>}
                <label className="field">
                    <span>To</span>
                    <input value={to} onChange={(e) => setTo(e.target.value)} placeholder="name@example.com, other@example.com"/>
                </label>
                <label className="field">
                    <span>Cc</span>
                    <input value={cc} onChange={(e) => setCc(e.target.value)}/>
                </label>
                <label className="field">
                    <span>Bcc</span>
                    <input value={bcc} onChange={(e) => setBcc(e.target.value)}/>
                </label>
                <label className="field">
                    <span>Subject</span>
                    <input value={subject} onChange={(e) => setSubject(e.target.value)}/>
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
                        <button className="btn" onClick={() => void saveDraft()} disabled={sending || savingDraft}>
                            {savingDraft ? 'Saving...' : 'Save draft'}
                        </button>
                        <button className="btn primary" onClick={() => void send()} disabled={sending || savingDraft || to.trim() === ''}>
                            {sending ? 'Sending...' : 'Send'}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}
