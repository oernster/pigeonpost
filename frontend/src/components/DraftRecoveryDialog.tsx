import {Dispatch, SetStateAction} from 'react'
import {DraftRecoveryResult} from '../api'

export interface DraftRecoveryDialogProps {
    recovery: DraftRecoveryResult
    setRecovery: Dispatch<SetStateAction<DraftRecoveryResult | null>>
    discardDraft: () => void
    restoreDraft: () => void
}

// DraftRecoveryDialog is the inline modal offering to restore a compose snapshot autosaved in a previous
// session. App renders it while a recovery snapshot is present and the composer is not already open; the
// recovery state and the restore/discard handlers live in useComposeLauncher.
export function DraftRecoveryDialog({recovery, setRecovery, discardDraft, restoreDraft}: DraftRecoveryDialogProps) {
    return (
        <div className="modal-backdrop" onClick={() => setRecovery(null)}>
            <div className="modal confirm" role="alertdialog" aria-label="Restore unsent message"
                 onClick={(e) => e.stopPropagation()}>
                <h2 className="modal-title">Restore unsent message?</h2>
                <p className="confirm-message">
                    An unsent message{recovery.subject.trim() ? ` "${recovery.subject.trim()}"` : ''} was
                    left open when PigeonPost last closed. Restore it to keep writing, or discard it.
                </p>
                <div className="modal-actions spread">
                    <button className="btn danger" onClick={discardDraft}>Discard</button>
                    <button className="btn primary" onClick={restoreDraft} autoFocus>Restore</button>
                </div>
            </div>
        </div>
    )
}
