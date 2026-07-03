interface ConfirmDialogProps {
    title: string
    message: string
    confirmLabel: string
    onConfirm: () => void
    onCancel: () => void
    busy?: boolean
}

// ConfirmDialog is the shared modal for confirming a destructive action. It names what happens and
// defaults focus to Cancel; the confirm button carries the danger styling.
export function ConfirmDialog({title, message, confirmLabel, onConfirm, onCancel, busy}: ConfirmDialogProps) {
    return (
        <div className="modal-backdrop" onClick={onCancel}>
            <div className="modal confirm" role="alertdialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
                <h2 className="modal-title">{title}</h2>
                <p className="confirm-message">{message}</p>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel} disabled={busy} autoFocus>Cancel</button>
                    <button className="btn danger" onClick={onConfirm} disabled={busy}>
                        {busy ? 'Removing...' : confirmLabel}
                    </button>
                </div>
            </div>
        </div>
    )
}
