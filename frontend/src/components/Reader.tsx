import {Message} from '../api'

interface ReaderProps {
    message: Message | null
    onToggleRead: (message: Message) => void
}

export function Reader({message, onToggleRead}: ReaderProps) {
    if (!message) {
        return (
            <section className="pane reader">
                <div className="empty-state"><p className="empty-body">Select a message to read.</p></div>
            </section>
        )
    }

    const sender = message.fromName
        ? `${message.fromName} <${message.fromAddress}>`
        : message.fromAddress || '(unknown sender)'

    return (
        <section className="pane reader">
            <div className="reader-header">
                <div className="reader-toolbar">
                    <button className="btn" onClick={() => onToggleRead(message)}>
                        {message.read ? 'Mark as unread' : 'Mark as read'}
                    </button>
                </div>
                <h2 className="reader-subject">{message.subject || '(no subject)'}</h2>
                <div className="reader-meta">
                    <span className="reader-label">From</span>
                    <span>{sender}</span>
                </div>
                {message.date && (
                    <div className="reader-meta">
                        <span className="reader-label">Date</span>
                        <span>{new Date(message.date).toLocaleString()}</span>
                    </div>
                )}
            </div>
            <div className="reader-body">
                {message.snippet ? <p>{message.snippet}</p> : <p className="empty-body">No preview available.</p>}
                <p className="phase-note">
                    Full message body rendering arrives in a later phase. This view shows the cached
                    header summary only.
                </p>
            </div>
        </section>
    )
}
