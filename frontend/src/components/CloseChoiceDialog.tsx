import {useState} from 'react'
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
// click. Minimise is the default (focused) choice; when another dialog is still open behind it (a
// message being written, a contact being edited), the dialog says so plainly, offers Go back as the
// focused default and leaves Quit available, so unsaved work is never lost silently.
export function CloseChoiceDialog({onMinimise, onQuit, onCancel}: CloseChoiceDialogProps) {
    const dismiss = useBackdropDismiss(onCancel)
    // Sampled once as this dialog opens, before its own DOM exists: any modal already present is a
    // work surface whose unsaved content a quit would take down with it.
    const [workOpen] = useState(() => document.querySelector('.modal') !== null)
    return (
        <div className="modal-backdrop top" {...dismiss}>
            <div className="modal confirm" role="alertdialog" aria-label="Close PigeonPost"
                 onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">Close PigeonPost</h2>
                <p className="confirm-message">Keep PigeonPost running in the system tray, or quit the
                    application? While it runs in the tray, reminders and new mail keep coming through.</p>
                {workOpen && (
                    <p className="confirm-message close-work-warning">A window you were working in is
                        still open and anything unsaved in it may be lost if you quit. Go back to finish
                        or save it first.</p>
                )}
                <div className="modal-actions spread">
                    <button className="btn" onClick={onQuit}>Quit</button>
                    <div className="action-group">
                        {workOpen && (
                            <button className="btn" onClick={onCancel} autoFocus>Go back</button>
                        )}
                        <button className="btn primary" onClick={onMinimise} autoFocus={!workOpen}>
                            Minimise to tray
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}
