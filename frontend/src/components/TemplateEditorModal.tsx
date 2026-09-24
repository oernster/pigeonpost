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
import {RichTextField} from './RichTextField'

interface TemplateEditorModalProps {
    // template is the one being edited; null starts a new one.
    template: Template | null
    onSaved: () => void
    onCancel: () => void
}

// TemplateEditorModal edits one message template, a dialog stacked on the template list the way the
// contact editor stacks on the address book. The name and subject are pinned above the scrolling body and
// attachments, the actions below them, so what is being edited and the Save button stay on screen however
// long the body grows. It mounts fresh for each template opened, so nothing carries over between them.
export function TemplateEditorModal({template, onSaved, onCancel}: TemplateEditorModalProps) {
    const dismiss = useBackdropDismiss(onCancel)
    const [name, setName] = useState(template?.name ?? '')
    const [subject, setSubject] = useState(template?.subject ?? '')
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)
    // files is the template's attachments as the editor holds them: stored ones named by position and
    // newly picked ones carrying a path. See templateFiles for the split the save request states.
    const [files, setFiles] = useState<TemplateEditorFile[]>(() => storedFiles(template?.attachments ?? []))

    // The body is edited as rich text (HTML), matching the composer.
    const editor = useEditor({
        extensions: [StarterKit.configure({link: EDITOR_LINK_OPTIONS})],
        content: template?.body ?? '',
        editorProps: EDITOR_PASTE_PROPS,
    })

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
            const req: TemplateInput = {id: template?.id ?? '', name, subject, body, ...saveFiles(files)}
            await api.saveTemplate(req)
            onSaved()
        } catch (e) {
            setError(String(e))
            setBusy(false)
        }
    }

    const title = template ? 'Edit template' : 'New template'
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal template-editor pinned-actions" role="dialog" aria-label={title}
                 onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">{title}</h2>
                <div className="rule-form pinned-form-header">
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
                </div>
                <div className="modal-body">
                    <div className="rule-form">
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
                {error && <div className="compose-error">{error}</div>}
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel}>Cancel</button>
                    <button className="btn primary" onClick={() => void save()} disabled={busy || name.trim() === ''}>
                        {busy ? 'Saving...' : template ? 'Save template' : 'Add template'}
                    </button>
                </div>
            </div>
        </div>
    )
}
