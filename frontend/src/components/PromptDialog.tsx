import {useState} from 'react'
import {ModalClose} from './ModalClose'

interface PromptDialogProps {
    title: string
    label: string
    initialValue?: string
    confirmLabel: string
    onSubmit: (value: string) => void
    onCancel: () => void
    busy?: boolean
}

// PromptDialog is the shared single-input modal used for folder create and rename. It submits the
// trimmed value on Enter or the confirm button, and never submits an empty value.
export function PromptDialog({title, label, initialValue, confirmLabel, onSubmit, onCancel, busy}: PromptDialogProps) {
    const [value, setValue] = useState(initialValue ?? '')
    const submit = () => {
        const trimmed = value.trim()
        if (trimmed !== '') {
            onSubmit(trimmed)
        }
    }
    return (
        <div className="modal-backdrop" onClick={onCancel}>
            <div className="modal" role="dialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">{title}</h2>
                <label className="field">
                    <span>{label}</span>
                    <input
                        value={value}
                        autoFocus
                        disabled={busy}
                        onChange={(e) => setValue(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                submit()
                            }
                        }}
                    />
                </label>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel} disabled={busy}>Cancel</button>
                    <button className="btn primary" onClick={submit} disabled={busy || value.trim() === ''}>
                        {busy ? 'Working...' : confirmLabel}
                    </button>
                </div>
            </div>
        </div>
    )
}
