import {useEffect, useState} from 'react'
import {CalendarEvent, CalendarEventInstance} from '../api'

interface OpenFromReminderInput {
    // initialEventId, when set, is the event a clicked reminder wants opened. It can change while the calendar
    // is already open, so a later click opens the newly requested event too.
    initialEventId: string | undefined
    events: CalendarEvent[]
    instances: CalendarEventInstance[]
    setViewDate: (date: Date) => void
    // onReveal opens the found occurrence's dialog. It is called once, when the occurrence is present in the
    // loaded range.
    onReveal: (inst: CalendarEventInstance) => void
}

// useOpenFromReminder lands a clicked reminder on the event it is about. It tracks the pending event id, jumps
// the month view to that event so its occurrence is expanded into the loaded range, then reveals the dialog
// once the occurrence has loaded and clears the id so it opens only once.
export function useOpenFromReminder({initialEventId, events, instances, setViewDate, onReveal}: OpenFromReminderInput) {
    // pendingOpenId is an event whose dialog should open once its occurrence has loaded, set when the calendar
    // is opened from a reminder. It is cleared once the dialog opens.
    const [pendingOpenId, setPendingOpenId] = useState<string | null>(initialEventId ?? null)

    // A reminder can be clicked while the calendar is already open, so sync the pending id from the prop
    // rather than only from the mount initialiser, so it opens the newly requested event too.
    useEffect(() => {
        if (initialEventId) {
            setPendingOpenId(initialEventId)
        }
    }, [initialEventId])

    // When opened from a reminder, jump the month view to the target event so its occurrence is expanded
    // into range. Re-runs when the events prop arrives, so a jump still happens if the list loaded after
    // the pending id was set. The early return once the id is cleared stops it after the dialog opens.
    useEffect(() => {
        if (!pendingOpenId) {
            return
        }
        const ev = events.find((e) => e.id === pendingOpenId)
        if (ev && ev.start) {
            setViewDate(new Date(ev.start))
        }
    }, [pendingOpenId, events])

    // Once the target event's occurrence is present in the loaded range, reveal its dialog and clear the
    // pending id so it opens only once.
    useEffect(() => {
        if (!pendingOpenId) {
            return
        }
        const inst = instances.find((i) => i.event.id === pendingOpenId)
        if (inst) {
            onReveal(inst)
            setPendingOpenId(null)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [pendingOpenId, instances])
}
