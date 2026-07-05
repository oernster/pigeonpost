import {EventScope} from '../api'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'

interface ScopeChooserProps {
    title: string
    message: string
    // danger styles the choices as a destructive action (used for delete).
    danger?: boolean
    busy?: boolean
    onChoose: (scope: EventScope) => void
    onCancel: () => void
}

// ScopeChooser asks how far an edit or delete of a recurring occurrence should reach: this occurrence,
// this and every later one, or the whole series. It is the confirmation step for a destructive delete of
// a recurring event, so it names the consequence and defaults focus to Cancel.
export function ScopeChooser({title, message, danger, busy, onChoose, onCancel}: ScopeChooserProps) {
    const dismiss = useBackdropDismiss(onCancel)
    const choiceClass = 'btn scope-choice' + (danger ? ' danger' : ' primary')
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal confirm" role="alertdialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">{title}</h2>
                <p className="confirm-message">{message}</p>
                <div className="scope-choices">
                    <button className={choiceClass} disabled={busy} onClick={() => onChoose(EventScope.This)}>
                        This event
                    </button>
                    <button className={choiceClass} disabled={busy} onClick={() => onChoose(EventScope.Future)}>
                        This and following events
                    </button>
                    <button className={choiceClass} disabled={busy} onClick={() => onChoose(EventScope.All)}>
                        All events
                    </button>
                </div>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel} disabled={busy} autoFocus>Cancel</button>
                </div>
            </div>
        </div>
    )
}
