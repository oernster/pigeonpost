import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'

interface ConfirmDialogProps {
    title: string
    message: string
    confirmLabel: string
    onConfirm: () => void
    onCancel: () => void
    busy?: boolean
    // defaultConfirm focuses the confirm (destructive) button instead of Cancel, so pressing Enter carries
    // out the action rather than cancelling it. Used where the user already invoked the action explicitly,
    // such as the Delete and Shift+Delete message shortcuts, so Enter completes the delete they asked for.
    defaultConfirm?: boolean
}

// ConfirmDialog is the shared modal for confirming a destructive action. It names what happens and
// defaults focus to Cancel, so Enter cancels; set defaultConfirm to focus the danger button instead so
// Enter carries out the action. The confirm button carries the danger styling either way.
export function ConfirmDialog({title, message, confirmLabel, onConfirm, onCancel, busy, defaultConfirm}: ConfirmDialogProps) {
    const dismiss = useBackdropDismiss(onCancel)
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal confirm" role="alertdialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">{title}</h2>
                <p className="confirm-message">{message}</p>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel} disabled={busy} autoFocus={!defaultConfirm}>Cancel</button>
                    <button className="btn danger" onClick={onConfirm} disabled={busy} autoFocus={defaultConfirm}>
                        {busy ? 'Removing...' : confirmLabel}
                    </button>
                </div>
            </div>
        </div>
    )
}
