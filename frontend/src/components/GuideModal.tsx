import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'
import {useAutoScroll} from '../hooks/useAutoScroll'
import {guideSections} from './guideContent'

interface GuideModalProps {
    open: boolean
    onClose: () => void
}

// GuideModal is the first entry on the Help menu: what the pictures in the bar and the folder list are,
// then the rules the windows cannot state for themselves. Its words live in guideContent.ts; this file
// only draws them, so a change to what the app says is a change to one document rather than to markup.
//
// It is built like About and the licence: the body is the scroller, Close is pinned below it and the body
// reads itself down gently until the reader takes over. The guide is longer than either, which is exactly
// why it must not scroll the dialog itself and take the way out off screen with it.
export function GuideModal({open, onClose}: GuideModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const autoScroll = useAutoScroll()
    if (!open) {
        return null
    }
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div
                className="modal guide pinned-actions"
                role="dialog"
                aria-label="Guide"
                onClick={(e) => e.stopPropagation()}
            >
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">How PigeonPost works</h2>
                <div className="modal-body guide-body" ref={autoScroll}>
                    {guideSections.map((section) => (
                        <section className="guide-section" key={section.heading}>
                            <h3 className="guide-heading">{section.heading}</h3>
                            {section.intro && <p className="guide-intro">{section.intro}</p>}
                            {section.entries?.map((entry) => (
                                <p className="guide-entry" key={entry.name}>
                                    <img className="guide-icon" src={entry.icon} alt="" draggable={false}/>
                                    <span><b>{entry.name}</b>: {entry.text}</span>
                                </p>
                            ))}
                            {section.rules?.map((rule) => (
                                <p className="guide-rule" key={rule.title}>
                                    <b>{rule.title}</b> {rule.text}
                                </p>
                            ))}
                            {section.paragraphs?.map((text) => (
                                <p className="guide-para" key={text}>{text}</p>
                            ))}
                        </section>
                    ))}
                </div>
                <div className="modal-actions">
                    <button className="btn primary" onClick={onClose}>Close</button>
                </div>
            </div>
        </div>
    )
}
