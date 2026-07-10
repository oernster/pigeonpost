import {useState} from 'react'
import {Message} from '../api'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'

interface MessagePickerDialogProps {
    messages: Message[]
    onAttach: (selected: Message[]) => void
    onCancel: () => void
}

// senderOf renders a message's sender for a picker row, preferring the display name over the address.
function senderOf(m: Message): string {
    return m.fromName || m.fromAddress || '(unknown sender)'
}

// MessagePickerDialog lets the user pick one or more messages from the open folder to attach to a new
// message (each becomes a .eml). A search box filters by subject or sender and a multi-selection is kept;
// the Attach button stays disabled until at least one is chosen. It is a .modal, so the window key handler
// traps Tab within it while the backdrop click or Escape dismisses it.
export function MessagePickerDialog({messages, onAttach, onCancel}: MessagePickerDialogProps) {
    const dismiss = useBackdropDismiss(onCancel)
    const [query, setQuery] = useState('')
    const [selected, setSelected] = useState<Set<string>>(new Set())

    const needle = query.trim().toLowerCase()
    const shown = needle === ''
        ? messages
        : messages.filter((m) =>
            (m.subject || '').toLowerCase().includes(needle) || senderOf(m).toLowerCase().includes(needle))

    const toggle = (id: string) => {
        setSelected((prev) => {
            const next = new Set(prev)
            if (next.has(id)) {
                next.delete(id)
            } else {
                next.add(id)
            }
            return next
        })
    }

    const confirm = () => {
        const picked = messages.filter((m) => selected.has(m.id))
        if (picked.length > 0) {
            onAttach(picked)
        }
    }

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div
                className="modal attach-picker"
                role="dialog"
                aria-label="Attach email"
                onClick={(e) => e.stopPropagation()}
            >
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">Attach email</h2>
                <input
                    className="attach-picker-search"
                    type="search"
                    placeholder="Search subject or sender"
                    aria-label="Search messages"
                    autoFocus
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                />
                <ul className="attach-picker-list">
                    {shown.length === 0 ? (
                        <li className="attach-picker-empty">No messages match.</li>
                    ) : (
                        shown.map((m) => (
                            <li key={m.id} className="attach-picker-item">
                                <label>
                                    <input
                                        type="checkbox"
                                        checked={selected.has(m.id)}
                                        onChange={() => toggle(m.id)}
                                    />
                                    <span className="attach-picker-subject">{m.subject || '(no subject)'}</span>
                                    <span className="attach-picker-from">{senderOf(m)}</span>
                                </label>
                            </li>
                        ))
                    )}
                </ul>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel}>Cancel</button>
                    <button className="btn primary" onClick={confirm} disabled={selected.size === 0}>
                        {selected.size > 0 ? `Attach ${selected.size}` : 'Attach'}
                    </button>
                </div>
            </div>
        </div>
    )
}
