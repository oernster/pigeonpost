import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'
import {useAutoScroll} from '../hooks/useAutoScroll'

interface LicenceModalProps {
    text: string | null
    onClose: () => void
}

export function LicenceModal({text, onClose}: LicenceModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    // A full licence is far longer than the pane, so it reads itself down rather than asking anyone who
    // wants to check a clause to drag a scrollbar through it.
    const autoScroll = useAutoScroll()
    if (text === null) {
        return null
    }
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal licence" role="dialog" aria-label="Licence" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Licence</h2>
                <pre className="licence-text" ref={autoScroll}>{text}</pre>
                <div className="modal-actions">
                    <button className="btn primary" onClick={onClose}>Close</button>
                </div>
            </div>
        </div>
    )
}
