interface LicenceModalProps {
    text: string | null
    onClose: () => void
}

export function LicenceModal({text, onClose}: LicenceModalProps) {
    if (text === null) {
        return null
    }
    return (
        <div className="modal-backdrop" onClick={onClose}>
            <div className="modal licence" role="dialog" aria-label="Licence" onClick={(e) => e.stopPropagation()}>
                <h2 className="modal-title">Licence</h2>
                <pre className="licence-text">{text}</pre>
                <div className="modal-actions">
                    <button className="btn primary" onClick={onClose}>Close</button>
                </div>
            </div>
        </div>
    )
}
