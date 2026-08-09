import {UpdateStatus} from '../api'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'

interface UpdateModalProps {
    status: UpdateStatus | null
    onClose: () => void
    // Opens the platform download (or the release page fallback) in the system browser.
    onDownload: (url: string) => void
    // Persists the skipped release so it never prompts again.
    onSkip: (version: string) => void
}

// The outcome of an update check. A newer release offers Download, Skip This Version and Later; a
// manual check that found nothing reports up to date or unreachable in the same dialog, so every
// outcome of Help > Check for Updates is answered somewhere visible.
export function UpdateModal({status, onClose, onDownload, onSkip}: UpdateModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    if (!status) {
        return null
    }
    const downloadURL = status.downloadUrl || status.pageUrl
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div
                className="modal update pinned-actions"
                role="dialog"
                aria-label="Software update"
                onClick={(e) => e.stopPropagation()}
            >
                <ModalClose onClose={onClose}/>
                {status.updateAvailable ? (
                    <>
                        <div className="modal-body">
                            <h2>Update available</h2>
                            <p>PigeonPost {status.latest} is available. You are running {status.current}.</p>
                        </div>
                        <div className="modal-actions">
                            <button
                                className="btn primary"
                                onClick={() => {
                                    if (downloadURL) {
                                        onDownload(downloadURL)
                                    }
                                    onClose()
                                }}
                            >Download</button>
                            <button
                                className="btn"
                                onClick={() => {
                                    onSkip(status.latest)
                                    onClose()
                                }}
                            >Skip This Version</button>
                            <button className="btn" onClick={onClose}>Later</button>
                        </div>
                    </>
                ) : (
                    <>
                        <div className="modal-body">
                            <h2>Check for updates</h2>
                            <p>
                                {status.latest
                                    ? 'You are running the latest version.'
                                    : 'The update check could not reach GitHub. Please try again later.'}
                            </p>
                        </div>
                        <div className="modal-actions">
                            <button className="btn primary" onClick={onClose}>Close</button>
                        </div>
                    </>
                )}
            </div>
        </div>
    )
}
