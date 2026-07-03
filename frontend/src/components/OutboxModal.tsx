import {useEffect, useState} from 'react'
import {api, OutboxItem} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'

function formatDate(ms: number): string {
    const date = new Date(ms)
    return isNaN(date.getTime()) ? '' : date.toLocaleString()
}

interface OutboxModalProps {
    onClose: () => void
    onChanged: () => void
}

// OutboxModal lists the outgoing operations queued while offline and lets the user discard one before
// it is sent. It loads its own items and refreshes them after a cancellation.
export function OutboxModal({onClose, onChanged}: OutboxModalProps) {
    const [items, setItems] = useState<OutboxItem[]>([])
    const [error, setError] = useState('')
    const [itemToCancel, setItemToCancel] = useState<OutboxItem | null>(null)
    const [cancelling, setCancelling] = useState(false)

    const load = () => {
        void api.listOutbox().then(setItems).catch((e) => setError(String(e)))
    }
    useEffect(() => {
        load()
    }, [])

    const confirmCancel = async () => {
        if (!itemToCancel) {
            return
        }
        setCancelling(true)
        setError('')
        try {
            await api.cancelOutboxItem(itemToCancel.id)
            setItemToCancel(null)
            load()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setCancelling(false)
        }
    }

    return (
        <>
            <div className="modal-backdrop" onClick={onClose}>
                <div className="modal" role="dialog" aria-label="Outbox" onClick={(e) => e.stopPropagation()}>
                    <ModalClose onClose={onClose}/>
                    <h2 className="modal-title">Outbox</h2>
                    {error && <div className="compose-error">{error}</div>}
                    {items.length === 0 ? (
                        <p className="empty-body">
                            Nothing is queued. Messages composed while offline wait here until the next sync.
                        </p>
                    ) : (
                        <ul className="outbox-list">
                            {items.map((item) => (
                                <li key={item.id} className="outbox-item">
                                    <div className="outbox-item-main">
                                        <span className="outbox-item-subject">{item.subject || '(no subject)'}</span>
                                        <span className="outbox-item-meta">
                                            {item.kind === 'draft'
                                                ? 'Draft'
                                                : `To: ${item.to.join(', ') || '(no recipients)'}`}
                                            {formatDate(item.createdMs) ? ` · ${formatDate(item.createdMs)}` : ''}
                                        </span>
                                    </div>
                                    <button className="btn danger-outline" onClick={() => setItemToCancel(item)}>
                                        Cancel
                                    </button>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            </div>
            {itemToCancel && (
                <ConfirmDialog
                    title="Cancel queued message"
                    message={`Discard the queued message "${itemToCancel.subject || '(no subject)'}"? It will not be sent.`}
                    confirmLabel="Discard"
                    busy={cancelling}
                    onConfirm={() => void confirmCancel()}
                    onCancel={() => setItemToCancel(null)}
                />
            )}
        </>
    )
}
