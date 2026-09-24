import {useState} from 'react'
import {useBackdropDismiss} from './useBackdropDismiss'
import {api, Template} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {TemplateEditorModal} from './TemplateEditorModal'

interface TemplateManagerModalProps {
    templates: Template[]
    onChanged: () => void
    onClose: () => void
}

// editing names the template open in the editor: one of the list's; NEW_TEMPLATE for a new one; null
// when the editor is closed.
const NEW_TEMPLATE = 'new'
type Editing = Template | typeof NEW_TEMPLATE | null

// TemplateManagerModal lists message templates, opens the editor for a new or existing one and deletes
// them. A template is a reusable {name, subject, body} skeleton (the body is HTML), inserted into a new
// message while composing. The editor is its own dialog stacked on this one; see TemplateEditorModal.
export function TemplateManagerModal({templates, onChanged, onClose}: TemplateManagerModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const [editing, setEditing] = useState<Editing>(null)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)
    // pendingDelete is the template awaiting delete confirmation; null when no prompt is open.
    const [pendingDelete, setPendingDelete] = useState<Template | null>(null)

    const remove = async (id: string) => {
        setError('')
        setBusy(true)
        try {
            await api.deleteTemplate(id)
            if (editing !== null && editing !== NEW_TEMPLATE && editing.id === id) {
                setEditing(null)
            }
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
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
                                    onClick={() => setEditing(t)}
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
                </div>
                {error && <div className="compose-error">{error}</div>}
                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose}>Close</button>
                    <button className="btn primary" onClick={() => setEditing(NEW_TEMPLATE)}>New template</button>
                </div>
            </div>
            {editing !== null && (
                <TemplateEditorModal
                    key={editing === NEW_TEMPLATE ? NEW_TEMPLATE : editing.id}
                    template={editing === NEW_TEMPLATE ? null : editing}
                    onSaved={() => {
                        setEditing(null)
                        onChanged()
                    }}
                    onCancel={() => setEditing(null)}
                />
            )}
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
