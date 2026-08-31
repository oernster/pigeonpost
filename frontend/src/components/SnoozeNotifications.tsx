import {useEffect, useState} from 'react'
import {EventsOn} from '../../wailsjs/runtime'
import {Message} from '../api'

interface SnoozeNotificationsProps {
    // onOpen is called with the message whose toast was clicked, so the parent can show it: its account,
    // then its folder, then the message itself.
    onOpen: (message: Message) => void
}

// noSubjectLabel stands in for a message that carries no subject, so a toast never shows a blank line
// where the subject should be. It matches what the desktop notification says for the same message.
const noSubjectLabel = '(no subject)'

// SnoozeNotifications shows a banner for each snoozed message that has come back, mirroring the
// reminder toasts. It exists because the desktop notification alone is not a reliable announcement: a
// Windows balloon reaches the user only if the shell's notification pipeline lets it through, which the
// app can neither see nor control, so an expiring snooze could pass with nothing shown at all. A toast
// is drawn by the app itself and cannot be suppressed by anything outside it.
//
// Clicking a toast opens the message it names; the × dismisses it without opening.
export function SnoozeNotifications({onOpen}: SnoozeNotificationsProps) {
    const [returned, setReturned] = useState<Message[]>([])
    useEffect(() => {
        return EventsOn('snooze:resurfaced', (messages: Message[]) => {
            // The backend sends the whole batch that came due at once; each message gets its own toast, so
            // a batch reads as what it is rather than as one message standing for several.
            setReturned((current) => [...current, ...(messages ?? [])])
        })
    }, [])
    if (returned.length === 0) {
        return null
    }
    const dismiss = (index: number) => setReturned((rs) => rs.filter((_, i) => i !== index))
    const open = (index: number, message: Message) => {
        onOpen(message)
        dismiss(index)
    }
    const sender = (message: Message) => message.fromName || message.fromAddress
    return (
        <div className="snooze-toasts">
            {returned.map((message, i) => (
                <div key={`${message.id}-${i}`} className="snooze-toast" role="alert">
                    <button
                        type="button"
                        className="snooze-toast-body"
                        title="Open the message"
                        onClick={() => open(i, message)}
                    >
                        <div className="snooze-toast-title">Snoozed message is back</div>
                        <div className="snooze-toast-summary">{message.subject || noSubjectLabel}</div>
                        <div className="snooze-toast-when">{sender(message)}</div>
                    </button>
                    <button type="button" className="btn" aria-label="Dismiss returned message"
                            onClick={() => dismiss(i)}>×</button>
                </div>
            ))}
        </div>
    )
}
