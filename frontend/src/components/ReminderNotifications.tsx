import {useEffect, useState} from 'react'
import {EventsOn} from '../../wailsjs/runtime'

interface Reminder {
    eventId: string
    summary: string
    start: string
}

interface ReminderNotificationsProps {
    // onOpen is called with the reminder's event id when its toast is clicked, so the parent can open the
    // calendar with that event's dialog on top.
    onOpen: (eventId: string) => void
}

// ReminderNotifications listens for calendar reminders pushed from the backend scheduler and shows a
// dismissible banner for each. This is the in-window surface; when the window is in the background the
// backend also flashes the taskbar and raises a Windows tray balloon. Clicking a toast opens the event it
// is about in the calendar; the × dismisses it without opening.
export function ReminderNotifications({onOpen}: ReminderNotificationsProps) {
    const [reminders, setReminders] = useState<Reminder[]>([])
    useEffect(() => {
        return EventsOn('calendar:reminder', (r: Reminder) => setReminders((rs) => [...rs, r]))
    }, [])
    if (reminders.length === 0) {
        return null
    }
    const dismiss = (index: number) => setReminders((rs) => rs.filter((_, i) => i !== index))
    const open = (index: number, eventId: string) => {
        onOpen(eventId)
        dismiss(index)
    }
    const when = (iso: string) =>
        new Date(iso).toLocaleString(undefined, {weekday: 'short', hour: '2-digit', minute: '2-digit'})
    return (
        <div className="reminder-toasts">
            {reminders.map((r, i) => (
                <div key={i} className="reminder-toast" role="alert">
                    <button
                        type="button"
                        className="reminder-toast-body"
                        title="Open in calendar"
                        onClick={() => open(i, r.eventId)}
                    >
                        <div className="reminder-toast-title">Reminder</div>
                        <div className="reminder-toast-summary">{r.summary}</div>
                        <div className="reminder-toast-when">{when(r.start)}</div>
                    </button>
                    <button type="button" className="btn" aria-label="Dismiss reminder"
                            onClick={() => dismiss(i)}>×</button>
                </div>
            ))}
        </div>
    )
}
