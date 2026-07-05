import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'

interface CloseChoiceDialogProps {
    onMinimise: () => void
    onQuit: () => void
    onCancel: () => void
}

// CloseChoiceDialog is shown when the window close button is pressed. It asks, in the app's own theme,
// whether to keep PigeonPost running in the system tray or to quit. Dismissing it (backdrop, Escape or
// the close cross) cancels the close and leaves the window open, so nothing is lost by an accidental
// click. Minimise is the default (focused) choice.
export function CloseChoiceDialog({onMinimise, onQuit, onCancel}: CloseChoiceDialogProps) {
    const dismiss = useBackdropDismiss(onCancel)
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal confirm" role="alertdialog" aria-label="Close PigeonPost"
                 onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">Close PigeonPost</h2>
                <p className="confirm-message">Keep PigeonPost running in the system tray, or quit the
                    application? While it runs in the tray, reminders and new mail keep coming through.</p>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onQuit}>Quit</button>
                    <button className="btn primary" onClick={onMinimise} autoFocus>Minimise to tray</button>
                </div>
            </div>
        </div>
    )
}
