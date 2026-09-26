import {Dispatch, SetStateAction, useState} from 'react'
import {Calendar, CalendarEventInstance, EventScope} from '../api'
import {browserZone, instantToZonedWall} from '../tz'
import {HOURS_PER_EVENT, dateInput, dateTimeInput} from '../calendarModel'
import type {EventForm} from '../components/EventFormModal'

// isSeries reports whether an occurrence belongs to a recurring series, so an edit or delete asks how far it
// should reach. A one-off event carries neither a rule nor a recurrence id.
export const isSeries = (i: CalendarEventInstance) => i.recurrenceId !== '' || i.event.recurrence !== ''

// blankEventForm is a new event's form: timed, from start to end in this browser's zone, in the given
// calendar, with nothing else filled in.
function blankEventForm(calendarId: string, start: Date, end: Date): EventForm {
    return {
        id: '', uid: '', calendarId, summary: '', description: '', location: '',
        category: '',
        allDay: false, start: dateTimeInput(start), end: dateTimeInput(end), timeZone: browserZone(),
        reminders: [], recurrence: '', extra: '', organizerAddress: '', organizerName: '', attendees: [],
        scope: null, occurrence: '', series: false,
    }
}

export interface EventFormOpeningDeps {
    // calendars seeds a new event with the first one.
    calendars: Calendar[]
    setError: (message: string) => void
    setStatus: (message: string) => void
}

export interface EventFormOpening {
    form: EventForm | null
    setForm: Dispatch<SetStateAction<EventForm | null>>
    attendeeDraft: string
    setAttendeeDraft: Dispatch<SetStateAction<string>>
    cancelledSent: boolean
    setCancelledSent: Dispatch<SetStateAction<boolean>>
    editScope: CalendarEventInstance | null
    setEditScope: Dispatch<SetStateAction<CalendarEventInstance | null>>
    openNew: (day: Date) => void
    openAt: (start: Date) => void
    openInstance: (inst: CalendarEventInstance) => void
    openForm: (inst: CalendarEventInstance, scope: EventScope | null) => void
    chooseEditScope: (scope: EventScope) => void
}

// useEventFormOpening owns the calendar's event dialog: whether it is open and with what, the attendee
// being typed, whether the open meeting's cancellation has gone and the recurring occurrence waiting on a
// scope. Its openers start a new event (on a day or at a time) or edit an occurrence, asking the scope first
// when the occurrence belongs to a series.
export function useEventFormOpening(deps: EventFormOpeningDeps): EventFormOpening {
    const {calendars, setError, setStatus} = deps
    const [form, setForm] = useState<EventForm | null>(null)
    // attendeeDraft holds the email being typed into the add-attendee field.
    const [attendeeDraft, setAttendeeDraft] = useState('')
    // cancelledSent is true once a cancellation has been emailed for the open meeting, so the cancel and
    // resend actions are disabled: a withdrawn meeting must not be cancelled again or re-invited.
    const [cancelledSent, setCancelledSent] = useState(false)
    const [editScope, setEditScope] = useState<CalendarEventInstance | null>(null)

    const defaultCalendarId = () => calendars[0]?.id ?? ''

    const openNew = (day: Date) => {
        setError('')
        setStatus('')
        setCancelledSent(false)
        const start = new Date(day)
        start.setHours(9, 0, 0, 0)
        const end = new Date(start)
        end.setHours(10, 0, 0, 0)
        setForm(blankEventForm(defaultCalendarId(), start, end))
    }

    // openAt starts a new one-hour event at the clicked time in the week or day time-grid.
    const openAt = (start: Date) => {
        setError('')
        setStatus('')
        setCancelledSent(false)
        const end = new Date(start)
        end.setHours(start.getHours() + HOURS_PER_EVENT)
        setForm(blankEventForm(defaultCalendarId(), start, end))
    }

    // openForm populates the edit form for an occurrence at the given scope. For an All-scope edit the form
    // shows the series master's own start and end (so editing the time changes the series); otherwise it
    // shows this occurrence's times.
    const openForm = (inst: CalendarEventInstance, scope: EventScope | null) => {
        const ev = inst.event
        const useMaster = scope === EventScope.All
        const startISO = useMaster ? ev.start : inst.start
        const endISO = useMaster ? ev.end : inst.end
        const zone = ev.timeZone || browserZone()
        // Timed events show their wall time in the event's own zone; all-day events are floating dates.
        const startWall = ev.allDay ? dateInput(new Date(startISO)) : instantToZonedWall(startISO, zone)
        const endWall = endISO ? (ev.allDay ? dateInput(new Date(endISO)) : instantToZonedWall(endISO, zone)) : ''
        setForm({
            id: ev.id, uid: ev.uid, calendarId: ev.calendarId, summary: ev.summary,
            description: ev.description, location: ev.location, category: ev.category, allDay: ev.allDay,
            start: startWall, end: endWall, timeZone: zone,
            reminders: [...ev.reminders], recurrence: ev.recurrence, extra: ev.extra,
            organizerAddress: ev.organizer.address, organizerName: ev.organizer.commonName,
            attendees: ev.attendees.map((a) => ({
                address: a.address, commonName: a.commonName, role: a.role, status: a.status, rsvp: a.rsvp,
            })),
            scope, occurrence: inst.recurrenceId, series: isSeries(inst),
        })
        setAttendeeDraft('')
        setError('')
        setStatus('')
        setCancelledSent(false)
        setEditScope(null)
    }

    // openInstance edits an occurrence. A recurring occurrence first asks the scope; a one-off opens the
    // form directly.
    const openInstance = (inst: CalendarEventInstance) => {
        if (isSeries(inst)) setEditScope(inst)
        else openForm(inst, null)
    }

    const chooseEditScope = (scope: EventScope) => {
        if (editScope) openForm(editScope, scope)
    }

    return {
        form, setForm, attendeeDraft, setAttendeeDraft, cancelledSent, setCancelledSent, editScope, setEditScope,
        openNew, openAt, openInstance, openForm, chooseEditScope,
    }
}
