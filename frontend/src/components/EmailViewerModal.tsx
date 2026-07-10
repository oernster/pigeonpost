import {useState} from 'react'
import {EmailView} from '../api'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'

interface EmailViewerModalProps {
    email: EmailView
    onClose: () => void
}

// EmailViewerModal shows an attached .eml inside PigeonPost rather than handing it to an external mail
// client. It renders the parsed headers and the sanitised body the same way the main reader does, with
// remote images parked behind a Load images bar until the reader asks for them.
export function EmailViewerModal({email, onClose}: EmailViewerModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const [imagesShown, setImagesShown] = useState(false)
    const rawHtml = email.html ?? ''
    const hasBlockedImages = rawHtml.includes('data-pp-src=')
    const renderedHtml = imagesShown ? rawHtml.replace(/data-pp-src=/g, 'src=') : rawHtml
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div
                className="modal email-viewer"
                role="dialog"
                aria-label="Attached email"
                onClick={(e) => e.stopPropagation()}
            >
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">{email.subject || '(no subject)'}</h2>
                <div className="email-viewer-headers">
                    {email.from && <div><span className="email-viewer-label">From</span>{email.from}</div>}
                    {email.to && <div><span className="email-viewer-label">To</span>{email.to}</div>}
                    {email.date && <div><span className="email-viewer-label">Date</span>{email.date}</div>}
                </div>
                {hasBlockedImages && !imagesShown && (
                    <div className="images-blocked-bar">
                        <span>Remote images were not loaded to protect your privacy.</span>
                        <button className="btn" onClick={() => setImagesShown(true)}>Load images</button>
                    </div>
                )}
                {rawHtml.trim() !== '' ? (
                    <div className="email-viewer-body reader-html" dangerouslySetInnerHTML={{__html: renderedHtml}}/>
                ) : (
                    <pre className="email-viewer-body reader-text">{email.plain || '(no content)'}</pre>
                )}
            </div>
        </div>
    )
}
