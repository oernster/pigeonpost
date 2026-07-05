import {useEffect, useState} from 'react'
import {EventsOn} from '../../wailsjs/runtime'

interface Reminder {
    eventId: string
    summary: string
    start: string
}

// ReminderNotifications listens for calendar reminders pushed from the backend scheduler and shows a
// dismissible banner for each. On-screen only for now; OS-level toasts are a later addition.
export function ReminderNotifications() {
    const [reminders, setReminders] = useState<Reminder[]>([])
    useEffect(() => {
        return EventsOn('calendar:reminder', (r: Reminder) => setReminders((rs) => [...rs, r]))
    }, [])
    if (reminders.length === 0) {
        return null
    }
    const dismiss = (index: number) => setReminders((rs) => rs.filter((_, i) => i !== index))
    const when = (iso: string) =>
        new Date(iso).toLocaleString(undefined, {weekday: 'short', hour: '2-digit', minute: '2-digit'})
    return (
        <div className="reminder-toasts">
            {reminders.map((r, i) => (
                <div key={i} className="reminder-toast" role="alert">
                    <div className="reminder-toast-body">
                        <div className="reminder-toast-title">Reminder</div>
                        <div className="reminder-toast-summary">{r.summary}</div>
                        <div className="reminder-toast-when">{when(r.start)}</div>
                    </div>
                    <button type="button" className="btn" aria-label="Dismiss reminder"
                            onClick={() => dismiss(i)}>×</button>
                </div>
            ))}
        </div>
    )
}
