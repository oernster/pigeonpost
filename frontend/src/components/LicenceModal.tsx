import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'
import {useAutoScroll} from '../hooks/useAutoScroll'
import type {LicenceView} from '../hooks/useHelpPanels'

interface LicenceModalProps {
    licence: LicenceView | null
    onClose: () => void
}

export function LicenceModal({licence, onClose}: LicenceModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    // A full licence is far longer than the pane, so it reads itself down rather than asking anyone who
    // wants to check a clause to drag a scrollbar through it. The text is the scroller and Close is pinned
    // below it, so the way out never scrolls away mid-licence.
    const autoScroll = useAutoScroll()
    if (licence === null) {
        return null
    }
    // The title names the licence, so which one this is stays on screen once its own opening lines have
    // scrolled away; it falls back to the plain word when no name came with the text.
    const title = licence.name ? `Licence: ${licence.name}` : 'Licence'
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal licence pinned-actions" role="dialog" aria-label="Licence" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">{title}</h2>
                <pre className="licence-text" ref={autoScroll}>{licence.text}</pre>
                <div className="modal-actions">
                    <button className="btn primary" onClick={onClose}>Close</button>
                </div>
            </div>
        </div>
    )
}
