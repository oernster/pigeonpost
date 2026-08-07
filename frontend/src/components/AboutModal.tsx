import icon from '../assets/pigeonpost.png'
import {AboutInfo} from '../api'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'
import {useAutoScroll} from '../hooks/useAutoScroll'

interface AboutModalProps {
    about: AboutInfo | null
    onClose: () => void
}

export function AboutModal({about, onClose}: AboutModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    // The credits run past the dialog's height on most screens, so the body reads itself down gently and
    // steps aside as soon as the reader scrolls it themselves. The scroller is the body, not the dialog, so
    // Close stays pinned at the foot while it reads.
    const autoScroll = useAutoScroll()
    if (!about) {
        return null
    }
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div
                className="modal about pinned-actions"
                role="dialog"
                aria-label="About PigeonPost"
                onClick={(e) => e.stopPropagation()}
            >
                <ModalClose onClose={onClose}/>
                <div className="modal-body" ref={autoScroll}>
                    <img className="about-icon" src={icon} alt="PigeonPost"/>
                    <h2 className="about-name">{about.name}</h2>
                    <p className="about-tagline">{about.tagline}</p>
                    <div className="about-lines">
                        <div><span className="about-label">Version</span>{about.version}</div>
                        <div><span className="about-label">Author</span>{about.author}</div>
                        <div><span className="about-label">Licence</span>{about.licence}</div>
                        <div className="about-copyright">{about.copyright}</div>
                    </div>
                    <p className="about-attribution">{about.attribution}</p>
                    <hr className="about-rule"/>
                    <h3 className="about-credits-title">Open source credits</h3>
                    <ul className="about-credits">
                        {about.credits.map((credit) => (
                            <li key={credit.name}>
                                <span className="credit-name">{credit.name}</span>
                                <span className="credit-licence">{credit.licence}</span>
                            </li>
                        ))}
                    </ul>
                    <p className="about-thanks">Built on the Go, Qt-free Wails and React ecosystems, with thanks to their communities.</p>
                </div>
                <div className="modal-actions">
                    <button className="btn primary" onClick={onClose}>Close</button>
                </div>
            </div>
        </div>
    )
}
