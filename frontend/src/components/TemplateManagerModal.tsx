import {useState} from 'react'
import {useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import {useBackdropDismiss} from './useBackdropDismiss'
import {api, Template, TemplateInput} from '../api'
import {formatBytes} from '../readerFormat'
import {
    pickedFiles,
    removeFileAt,
    saveFiles,
    storedFiles,
    STORED_ELSEWHERE,
    type TemplateEditorFile,
} from '../templateFiles'
import {EDITOR_LINK_OPTIONS, EDITOR_PASTE_PROPS} from '../richText'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {RichTextField} from './RichTextField'

interface TemplateManagerModalProps {
    templates: Template[]
    onChanged: () => void
    onClose: () => void
}

// TemplateManagerModal lists message templates and adds, edits or deletes them. A template is a reusable
// {name, subject, body} skeleton (the body is HTML), inserted into a new message while composing.
export function TemplateManagerModal({templates, onChanged, onClose}: TemplateManagerModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    // editingId is the id of the template being edited; empty when composing a new one.
    const [editingId, setEditingId] = useState('')
    const [name, setName] = useState('')
    const [subject, setSubject] = useState('')
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)
    // pendingDelete is the template awaiting delete confirmation; null when no prompt is open.
    const [pendingDelete, setPendingDelete] = useState<Template | null>(null)
    // files is the template's attachments as the editor holds them: stored ones named by position and
    // newly picked ones carrying a path. See templateFiles for the split the save request states.
    const [files, setFiles] = useState<TemplateEditorFile[]>([])

    // The body is edited as rich text (HTML), matching the composer. The editor is created once; loading a
    // template for editing sets its content through setContent; the reset clears it.
    const editor = useEditor({
        extensions: [StarterKit.configure({link: EDITOR_LINK_OPTIONS})],
        content: '',
        editorProps: EDITOR_PASTE_PROPS,
    })

    const reset = () => {
        setEditingId('')
        setName('')
        setSubject('')
        setFiles([])
        editor?.commands.setContent('')
    }

    // startEdit loads a template into the form so its fields can be changed and re-saved under its own id.
    const startEdit = (t: Template) => {
        setEditingId(t.id)
        setName(t.name)
        setSubject(t.subject)
        setFiles(storedFiles(t.attachments ?? []))
        editor?.commands.setContent(t.body || '')
    }

    // addFiles opens the native picker and appends what was chosen. The bytes are read by the backend at
    // save time, so nothing is loaded into the window here.
    const addFiles = async () => {
        setError('')
        try {
            const picked = await api.pickAttachments()
            if (picked.length > 0) {
                setFiles((prev) => [...prev, ...pickedFiles(picked, prev)])
            }
        } catch (e) {
            setError(String(e))
        }
    }

    const save = async () => {
        setBusy(true)
        setError('')
        try {
            // An empty editor serialises to "<p></p>"; store it as blank so the template carries no body.
            const body = editor && !editor.isEmpty ? editor.getHTML() : ''
            const req: TemplateInput = {id: editingId, name, subject, body, ...saveFiles(files)}
            await api.saveTemplate(req)
            reset()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const remove = async (id: string) => {
        setError('')
        try {
            await api.deleteTemplate(id)
            if (editingId === id) {
                reset()
            }
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setPendingDelete(null)
        }
    }

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal pinned-actions" role="dialog" aria-label="Message templates" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Message templates</h2>
                <div className="modal-body">
                <p className="setup-hint">Templates are reusable subjects and bodies you insert while composing.</p>
                {error && <div className="compose-error">{error}</div>}
                {templates.length === 0 ? (
                    <p className="empty-body">No templates yet.</p>
                ) : (
                    <ul className="list">
                        {templates.map((t) => (
                            <li key={t.id} className="list-item template-row">
                                <span className="item-text">
                                    <span className="item-title" title={t.name}>{t.name}</span>
                                    <span className="item-sub" title={t.subject}>{t.subject || '(no subject)'}</span>
                                </span>
                                <button
                                    className="account-action"
                                    aria-label={`Edit ${t.name}`}
                                    title="Edit template"
                                    onClick={() => startEdit(t)}
                                >
                                    &#9998;
                                </button>
                                <button
                                    className="account-action delete"
                                    aria-label={`Delete ${t.name}`}
                                    title="Delete template"
                                    onClick={() => setPendingDelete(t)}
                                >
                                    &times;
                                </button>
                            </li>
                        ))}
                    </ul>
                )}
                <div className="rule-form">
                    <input
                        className="tag-name-input"
                        placeholder="Template name"
                        value={name}
                        autoFocus
                        onChange={(e) => setName(e.target.value)}
                    />
                    <input
                        className="tag-name-input"
                        placeholder="Subject"
                        value={subject}
                        onChange={(e) => setSubject(e.target.value)}
                    />
                    <RichTextField editor={editor} full/>
                    <button type="button" className="btn" onClick={() => void addFiles()}>
                        Attach files
                    </button>
                    {files.length > 0 && (
                        <ul className="attachment-list">
                            {files.map((file, index) => (
                                <li key={`${file.position}-${file.path}-${file.name}`} className="attachment-chip">
                                    <span className="attachment-name" title={file.path || file.name}>{file.name}</span>
                                    {file.position !== STORED_ELSEWHERE && (
                                        <span className="attachment-size">{formatBytes(file.size)}</span>
                                    )}
                                    <button
                                        type="button"
                                        className="attachment-remove"
                                        aria-label={`Remove ${file.name}`}
                                        onClick={() => setFiles((prev) => removeFileAt(prev, index))}
                                    >
                                        &times;
                                    </button>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
                </div>
                <div className="modal-actions spread">
                    <button className="btn" onClick={editingId ? reset : onClose}>
                        {editingId ? 'Cancel edit' : 'Close'}
                    </button>
                    <button
                        className="btn primary"
                        onClick={() => void save()}
                        disabled={busy || name.trim() === ''}
                    >
                        {busy ? 'Saving...' : editingId ? 'Save template' : 'Add template'}
                    </button>
                </div>
            </div>
            {pendingDelete && (
                <ConfirmDialog
                    title="Delete template"
                    message={`Delete "${pendingDelete.name}"? This cannot be undone.`}
                    confirmLabel="Delete"
                    busy={busy}
                    defaultConfirm
                    onConfirm={() => void remove(pendingDelete.id)}
                    onCancel={() => setPendingDelete(null)}
                />
            )}
        </div>
    )
}
