import {useEffect, useRef, useState} from 'react'
import {api, Calendar, CalendarEvent, CalendarEventInput, CalendarEventInstance, EventScope} from '../api'
import {browserZone, instantToZonedWall, zonedWallToISO, zoneOptions} from '../tz'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {ScopeChooser} from './ScopeChooser'
import {RecurrenceEditor} from './RecurrenceEditor'
import {PickerButton} from './PickerButton'
import {CalendarTimeGrid} from './CalendarTimeGrid'
import {useBackdropDismiss} from './useBackdropDismiss'

const DAYS_IN_WEEK = 7
const HOURS_PER_EVENT = 1
// DEFAULT_EVENT_COLOUR marks events not assigned to a calendar (or whose calendar has no colour).
const DEFAULT_EVENT_COLOUR = '#7fb0ff'
const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const WEEKDAYS_FULL = [
    'Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday',
]
const MONTHS = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
]
const MONTHS_SHORT = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
]

type ViewMode = 'month' | 'week' | 'day'
const VIEW_MODES: ViewMode[] = ['month', 'week', 'day']

// REMINDER_PRESETS are the reminder lead times offered in the form, in whole minutes before the start.
const REMINDER_PRESETS: {minutes: number; label: string}[] = [
    {minutes: 0, label: 'At time of event'},
    {minutes: 5, label: '5 minutes before'},
    {minutes: 10, label: '10 minutes before'},
    {minutes: 15, label: '15 minutes before'},
    {minutes: 30, label: '30 minutes before'},
    {minutes: 60, label: '1 hour before'},
    {minutes: 120, label: '2 hours before'},
    {minutes: 1440, label: '1 day before'},
    {minutes: 10080, label: '1 week before'},
]
const DEFAULT_REMINDER_MINUTES = 15

// DEFAULT_ATTENDEE_ROLE and DEFAULT_ATTENDEE_STATUS are the ICS ROLE and PARTSTAT values a freshly added
// attendee carries: a required participant who has not yet responded.
const DEFAULT_ATTENDEE_ROLE = 'REQ-PARTICIPANT'
const DEFAULT_ATTENDEE_STATUS = 'NEEDS-ACTION'

// ATTENDEE_STATUS_LABELS maps an ICS PARTSTAT value to a human label shown against each attendee.
const ATTENDEE_STATUS_LABELS: Record<string, string> = {
    'NEEDS-ACTION': 'No response yet',
    'ACCEPTED': 'Accepted',
    'DECLINED': 'Declined',
    'TENTATIVE': 'Tentative',
    'DELEGATED': 'Delegated',
}

function attendeeStatusLabel(status: string): string {
    return ATTENDEE_STATUS_LABELS[status] || status
}

// AttendeeRow is one invited party held in the edit form. It mirrors the fields the backend persists so a
// loaded meeting round-trips its attendees' roles and reply statuses unchanged.
interface AttendeeRow {
    address: string
    commonName: string
    role: string
    status: string
    rsvp: boolean
}

interface EventForm {
    id: string
    uid: string
    calendarId: string
    summary: string
    description: string
    location: string
    allDay: boolean
    start: string
    end: string
    // timeZone is the IANA zone the start and end wall-clock times are entered in; empty is treated as the
    // browser zone. It is ignored for all-day events.
    timeZone: string
    // reminders are lead times in whole minutes before the start.
    reminders: number[]
    recurrence: string
    // extra is the opaque preserved ICS, carried unchanged so an edit does not strip unmodelled data.
    extra: string
    // organizerAddress and organizerName carry a loaded meeting's organizer. They are empty for a new
    // event; on save with attendees the organizer defaults to the active account. attendees is the invited
    // list: a non-empty list makes the event a meeting.
    organizerAddress: string
    organizerName: string
    attendees: AttendeeRow[]
    // scope is set when editing a recurring occurrence: it says how far the save reaches. It is null for a
    // new event or a one-off edit, which save directly. occurrence is the RFC 3339 recurrence id of the
    // occurrence being edited; series marks the event as part of a recurring series.
    scope: EventScope | null
    occurrence: string
    series: boolean
}

function pad(n: number): string {
    return n < 10 ? '0' + n : String(n)
}

function dateInput(d: Date): string {
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function dateTimeInput(d: Date): string {
    return `${dateInput(d)}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function sameDay(a: Date, b: Date): boolean {
    return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
}

// monthCells returns 42 day cells (six weeks) covering the month of viewDate, starting on the Sunday on
// or before the first of the month.
function monthCells(viewDate: Date): Date[] {
    const first = new Date(viewDate.getFullYear(), viewDate.getMonth(), 1)
    const start = new Date(first)
    start.setDate(first.getDate() - first.getDay())
    const cells: Date[] = []
    for (let i = 0; i < 42; i++) {
        const d = new Date(start)
        d.setDate(start.getDate() + i)
        cells.push(d)
    }
    return cells
}

// weekDays returns the seven days of the week containing viewDate, starting on the Sunday on or before it.
function weekDays(viewDate: Date): Date[] {
    const start = new Date(viewDate)
    start.setDate(viewDate.getDate() - viewDate.getDay())
    const days: Date[] = []
    for (let i = 0; i < DAYS_IN_WEEK; i++) {
        const d = new Date(start)
        d.setDate(start.getDate() + i)
        days.push(d)
    }
    return days
}

interface CalendarModalProps {
    events: CalendarEvent[]
    // accountId, accountEmail and accountName identify the active account that organizes meetings: the
    // organizer written onto a meeting and the sender of its invitations. accountId is empty when no
    // account is selected, which disables sending.
    accountId: string
    accountEmail: string
    accountName: string
    onChanged: () => void
    onClose: () => void
}

// CalendarModal shows a month view of events and edits them. It imports and exports iCalendar (.ics) so
// events round-trip with Outlook and Thunderbird. An event with attendees is a meeting: its invitations
// and cancellations are emailed through the active account. Deletion is always confirmed.
export function CalendarModal({events, accountId, accountEmail, accountName, onChanged, onClose}: CalendarModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const startRef = useRef<HTMLInputElement>(null)
    const endRef = useRef<HTMLInputElement>(null)
    const [viewDate, setViewDate] = useState(() => new Date())
    const [viewMode, setViewMode] = useState<ViewMode>('month')
    const [form, setForm] = useState<EventForm | null>(null)
    // attendeeDraft holds the email being typed into the add-attendee field. cancelMeeting gates the
    // confirm dialog for emailing a meeting cancellation, a destructive outward action.
    const [attendeeDraft, setAttendeeDraft] = useState('')
    const [cancelMeeting, setCancelMeeting] = useState(false)
    // cancelledSent is true once a cancellation has been emailed for the open meeting, so the cancel and
    // resend actions are disabled: a withdrawn meeting must not be cancelled again or re-invited.
    const [cancelledSent, setCancelledSent] = useState(false)
    const [pendingDelete, setPendingDelete] = useState<{id: string; summary: string} | null>(null)
    const [editScope, setEditScope] = useState<CalendarEventInstance | null>(null)
    const [deleteScope, setDeleteScope] = useState<{seriesId: string; occurrence: string; summary: string} | null>(null)
    const [error, setError] = useState('')
    const [status, setStatus] = useState('')
    const [busy, setBusy] = useState(false)
    const [calendars, setCalendars] = useState<Calendar[]>([])
    const [managingCals, setManagingCals] = useState(false)
    const [calForm, setCalForm] = useState<{id: string; name: string; colour: string} | null>(null)
    const [pendingCalDelete, setPendingCalDelete] = useState<Calendar | null>(null)
    // instances are the concrete occurrences shown for the visible range, expanded from recurring events by
    // the backend. reloadKey forces a refetch after a local change even if the parent's events prop is stable.
    const [instances, setInstances] = useState<CalendarEventInstance[]>([])
    const [reloadKey, setReloadKey] = useState(0)
    const bumpReload = () => setReloadKey((k) => k + 1)

    const reloadCalendars = () =>
        void api.listCalendars().then(setCalendars).catch((e) => setError(String(e)))
    useEffect(() => {
        reloadCalendars()
    }, [])

    // visibleRange is the inclusive [from, to] window of the active view, used to expand recurring events
    // into just the occurrences on screen.
    const visibleRange = (): {from: string; to: string} => {
        const days = viewMode === 'month' ? monthCells(viewDate)
            : viewMode === 'week' ? weekDays(viewDate) : [viewDate]
        const first = new Date(days[0])
        first.setHours(0, 0, 0, 0)
        const last = new Date(days[days.length - 1])
        last.setHours(23, 59, 59, 0)
        return {from: first.toISOString(), to: last.toISOString()}
    }

    useEffect(() => {
        const {from, to} = visibleRange()
        let active = true
        api.listEventInstances(from, to)
            .then((xs) => {
                if (active) setInstances(xs)
            })
            .catch((e) => {
                if (active) setError(String(e))
            })
        return () => {
            active = false
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [viewDate, viewMode, reloadKey, events])

    // colourOf resolves an event's colour from its calendar, falling back to the default for events with
    // no calendar. The map is rebuilt each render, which is cheap for the handful of calendars a user has.
    const colourById = new Map(calendars.map((c) => [c.id, c.colour || DEFAULT_EVENT_COLOUR]))
    const colourOf = (e: CalendarEvent) => colourById.get(e.calendarId) ?? DEFAULT_EVENT_COLOUR
    const defaultCalendarId = () => calendars[0]?.id ?? ''

    const saveCal = async () => {
        if (!calForm || calForm.name.trim() === '') return
        setBusy(true)
        setError('')
        try {
            await api.saveCalendar({id: calForm.id, name: calForm.name.trim(), colour: calForm.colour})
            setCalForm(null)
            reloadCalendars()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const confirmCalDelete = async () => {
        if (!pendingCalDelete) return
        setBusy(true)
        setError('')
        try {
            // Deleting a calendar deletes its events too, so refresh the event list as well.
            await api.deleteCalendar(pendingCalDelete.id)
            if (calForm && calForm.id === pendingCalDelete.id) setCalForm(null)
            setPendingCalDelete(null)
            reloadCalendars()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const cells = monthCells(viewDate)
    const datedInstances = instances.map((i) => ({i, start: new Date(i.start)}))
    // isSeries reports whether an occurrence belongs to a recurring series, so an edit or delete asks how
    // far it should reach. A one-off event carries neither a rule nor a recurrence id.
    const isSeries = (i: CalendarEventInstance) => i.recurrenceId !== '' || i.event.recurrence !== ''

    const set = <K extends keyof EventForm>(key: K, value: EventForm[K]) =>
        setForm((f) => (f ? {...f, [key]: value} : f))

    const openNew = (day: Date) => {
        setError('')
        setStatus('')
        setCancelledSent(false)
        const start = new Date(day)
        start.setHours(9, 0, 0, 0)
        const end = new Date(start)
        end.setHours(10, 0, 0, 0)
        setForm({
            id: '', uid: '', calendarId: defaultCalendarId(), summary: '', description: '', location: '',
            allDay: false, start: dateTimeInput(start), end: dateTimeInput(end), timeZone: browserZone(),
            reminders: [], recurrence: '', extra: '', organizerAddress: '', organizerName: '', attendees: [],
            scope: null, occurrence: '', series: false,
        })
    }

    // openAt starts a new one-hour event at the clicked time in the week or day time-grid.
    const openAt = (start: Date) => {
        setError('')
        setStatus('')
        setCancelledSent(false)
        const end = new Date(start)
        end.setHours(start.getHours() + HOURS_PER_EVENT)
        setForm({
            id: '', uid: '', calendarId: defaultCalendarId(), summary: '', description: '', location: '',
            allDay: false, start: dateTimeInput(start), end: dateTimeInput(end), timeZone: browserZone(),
            reminders: [], recurrence: '', extra: '', organizerAddress: '', organizerName: '', attendees: [],
            scope: null, occurrence: '', series: false,
        })
    }

    // openInstance edits an occurrence. A recurring occurrence first asks the scope; a one-off opens the
    // form directly.
    const openInstance = (inst: CalendarEventInstance) => {
        if (isSeries(inst)) setEditScope(inst)
        else openForm(inst, null)
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
            description: ev.description, location: ev.location, allDay: ev.allDay,
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

    const chooseEditScope = (scope: EventScope) => {
        if (editScope) openForm(editScope, scope)
    }

    const setReminder = (index: number, minutes: number) =>
        setForm((f) => (f ? {...f, reminders: f.reminders.map((r, i) => (i === index ? minutes : r))} : f))
    const addReminder = () =>
        setForm((f) => (f ? {...f, reminders: [...f.reminders, DEFAULT_REMINDER_MINUTES]} : f))
    const removeReminder = (index: number) =>
        setForm((f) => (f ? {...f, reminders: f.reminders.filter((_, i) => i !== index)} : f))

    // isAttendeeEmail is a light client-side check; the backend validates the address authoritatively.
    const isAttendeeEmail = (value: string): boolean => {
        const at = value.indexOf('@')
        return at > 0 && at < value.length - 1
    }

    // addAttendee appends the drafted email as a required, not-yet-responded attendee, ignoring a blank or
    // duplicate address.
    const addAttendee = () => {
        const address = attendeeDraft.trim()
        if (!isAttendeeEmail(address)) return
        setForm((f) => {
            if (!f) return f
            if (f.attendees.some((a) => a.address.toLowerCase() === address.toLowerCase())) return f
            return {
                ...f,
                attendees: [...f.attendees, {
                    address, commonName: '', role: DEFAULT_ATTENDEE_ROLE, status: DEFAULT_ATTENDEE_STATUS, rsvp: true,
                }],
            }
        })
        setAttendeeDraft('')
    }

    const removeAttendee = (index: number) =>
        setForm((f) => (f ? {...f, attendees: f.attendees.filter((_, i) => i !== index)} : f))

    // organizerLabel is the organizer shown in the meeting section: the loaded meeting's organizer, or the
    // active account that will own a newly organized meeting.
    const organizerLabel = (): string => {
        if (!form) return ''
        if (form.organizerAddress) return form.organizerName || form.organizerAddress
        return accountName ? `${accountName} (${accountEmail})` : accountEmail
    }

    // primaryActionLabel names the save button. For a meeting with a usable account, saving also emails the
    // invitation to the attendees, so the label says so rather than a plain Save.
    const primaryActionLabel = (): string => {
        if (!form) return ''
        if (form.attendees.length > 0 && accountId !== '' && !cancelledSent) {
            return form.id ? 'Save and send update' : 'Send invitation'
        }
        return form.id ? 'Save changes' : 'Add event'
    }

    const toISO = (value: string): string => (value ? new Date(value).toISOString() : '')

    const save = async () => {
        if (!form) return
        setBusy(true)
        setError('')
        try {
            // A timed event's wall time is interpreted in its chosen zone; an all-day date is floating and
            // carries no zone.
            const startISO = form.allDay ? toISO(form.start) : zonedWallToISO(form.start, form.timeZone)
            const endISO = form.allDay ? toISO(form.end) : (form.end ? zonedWallToISO(form.end, form.timeZone) : '')
            // A meeting (any attendees) needs an organizer to be replied to: keep the loaded one, or adopt
            // the active account when newly organizing. An event with no attendees stays a plain entry with
            // an empty organizer.
            const hasAttendees = form.attendees.length > 0
            const organizerAddress = form.organizerAddress || (hasAttendees ? accountEmail : '')
            const organizerName = form.organizerAddress ? form.organizerName : (hasAttendees ? accountName : '')
            const req: CalendarEventInput = {
                id: form.id, uid: form.uid, calendarId: form.calendarId, summary: form.summary,
                description: form.description, location: form.location, allDay: form.allDay,
                start: startISO, end: endISO, timeZone: form.allDay ? '' : form.timeZone,
                reminders: form.reminders, recurrence: form.recurrence, extra: form.extra,
                organizer: {address: organizerAddress, commonName: organizerName},
                attendees: form.attendees.map((a) => ({
                    address: a.address, commonName: a.commonName, role: a.role, status: a.status, rsvp: a.rsvp,
                })),
            }
            let savedId = form.id
            if (form.scope !== null) {
                await api.saveEventScoped(req, form.scope, form.occurrence)
            } else {
                savedId = await api.saveEvent(req)
                // Reflect the persisted id (freshly generated for a new event) back onto the form at once,
                // so if the send below fails a retry reuses this id rather than creating a duplicate event.
                setForm((f) => (f ? {...f, id: savedId, organizerAddress, organizerName} : f))
            }
            bumpReload()
            onChanged()
            // Saving a meeting sends its invitation: adding attendees and saving is what invites them, the
            // same way a calendar app's meeting Send both saves and notifies. Re-saving sends an update.
            if (hasAttendees && !cancelledSent) {
                if (accountId === '') {
                    console.warn('meeting invite: not sending, no account selected', {savedId})
                    setStatus('Meeting saved. Select an account to send the invitation to the attendees.')
                } else {
                    console.info('meeting invite: sending request', {accountId, savedId, attendees: form.attendees.length})
                    await api.sendMeetingRequest(accountId, savedId)
                    console.info('meeting invite: request sent', {savedId})
                    const n = form.attendees.length
                    setStatus(`Invitation sent to ${n} attendee${n === 1 ? '' : 's'}.`)
                    setForm(null)
                }
            } else {
                setForm(null)
            }
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    // requestDelete starts a delete from the edit form: a recurring occurrence asks the scope, a one-off is
    // confirmed directly.
    const requestDelete = () => {
        if (!form) return
        if (form.series) {
            setDeleteScope({seriesId: form.id, occurrence: form.occurrence, summary: form.summary})
            return
        }
        // Confirm straight from the open form. A previous version looked the event up in the events prop
        // first and silently did nothing when the lookup missed (a just-saved event not yet in that stale
        // list), which made delete impossible; the form already holds the id and summary the confirm needs.
        setPendingDelete({id: form.id, summary: form.summary})
    }

    const confirmDelete = async () => {
        if (!pendingDelete) return
        setBusy(true)
        setError('')
        try {
            await api.deleteEvent(pendingDelete.id)
            if (form && form.id === pendingDelete.id) setForm(null)
            setPendingDelete(null)
            bumpReload()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const confirmDeleteScope = async (scope: EventScope) => {
        if (!deleteScope) return
        setBusy(true)
        setError('')
        try {
            await api.deleteEventScoped(scope, deleteScope.seriesId, deleteScope.occurrence)
            setForm(null)
            setDeleteScope(null)
            bumpReload()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    // sendInvitations emails a meeting REQUEST to the saved event's attendees from the active account. It
    // is available only once the event exists (so it has an id to send) and an account is selected.
    const sendInvitations = async () => {
        if (!form || form.id === '' || accountId === '') return
        setBusy(true)
        setError('')
        setStatus('')
        try {
            await api.sendMeetingRequest(accountId, form.id)
            const n = form.attendees.length
            setStatus(`Invitation sent to ${n} attendee${n === 1 ? '' : 's'}.`)
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    // confirmCancelMeeting emails a meeting CANCEL to the attendees, withdrawing the meeting. It is
    // confirmed first because it is an outward action that cannot be recalled.
    const confirmCancelMeeting = async () => {
        if (!form || form.id === '' || accountId === '') return
        setBusy(true)
        setError('')
        setStatus('')
        try {
            await api.sendMeetingCancel(accountId, form.id)
            // Mark it cancelled before the delete, so if the delete fails the attendees are known to have
            // been notified and the meeting cannot be cancelled again.
            setCancelledSent(true)
            // A cancelled meeting is withdrawn: after notifying the attendees, remove it from the
            // organizer's own calendar too, the way a calendar app deletes a meeting you cancel.
            await api.deleteEvent(form.id)
            setCancelMeeting(false)
            setForm(null)
            bumpReload()
            onChanged()
            setStatus('Meeting cancelled: the attendees were notified and it was removed from your calendar.')
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const doImport = async () => {
        setError('')
        setStatus('')
        try {
            const n = await api.importEventsFromFile()
            if (n > 0) {
                setStatus(`Imported ${n} event${n === 1 ? '' : 's'}.`)
                bumpReload()
                onChanged()
            }
        } catch (e) {
            setError(String(e))
        }
    }

    const doExport = async () => {
        setError('')
        setStatus('')
        try {
            const written = await api.exportEventsToFile()
            if (written) setStatus(`Exported ${events.length} event${events.length === 1 ? '' : 's'}.`)
        } catch (e) {
            setError(String(e))
        }
    }

    // shift moves the view by one unit of the active mode: a month, a week or a day.
    const shift = (delta: number) =>
        setViewDate((d) => {
            if (viewMode === 'month') return new Date(d.getFullYear(), d.getMonth() + delta, 1)
            const n = new Date(d)
            n.setDate(d.getDate() + delta * (viewMode === 'week' ? DAYS_IN_WEEK : 1))
            return n
        })

    const headerLabel = (): string => {
        if (viewMode === 'month') return `${MONTHS[viewDate.getMonth()]} ${viewDate.getFullYear()}`
        if (viewMode === 'day') {
            return `${WEEKDAYS_FULL[viewDate.getDay()]}, ${viewDate.getDate()} ` +
                `${MONTHS[viewDate.getMonth()]} ${viewDate.getFullYear()}`
        }
        const wd = weekDays(viewDate)
        const a = wd[0]
        const b = wd[DAYS_IN_WEEK - 1]
        return `${a.getDate()} ${MONTHS_SHORT[a.getMonth()]} to ` +
            `${b.getDate()} ${MONTHS_SHORT[b.getMonth()]} ${b.getFullYear()}`
    }

    const openDay = (day: Date) => {
        setViewDate(day)
        setViewMode('day')
    }

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal calendar-modal" role="dialog" aria-label="Calendar" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Calendar</h2>
                {error && <div className="compose-error">{error}</div>}
                {status && <div className="setup-hint">{status}</div>}

                <div className="modal-actions cal-toolbar">
                    <button className="btn" aria-label="Previous" onClick={() => shift(-1)}>‹</button>
                    <span className="cal-month">{headerLabel()}</span>
                    <button className="btn" aria-label="Next" onClick={() => shift(1)}>›</button>
                    <button className="btn" onClick={() => setViewDate(new Date())}>Today</button>
                    <span className="cal-viewswitch">
                        {VIEW_MODES.map((m) => (
                            <button key={m} className={'btn cal-view-btn' + (viewMode === m ? ' active' : '')}
                                    aria-pressed={viewMode === m} onClick={() => setViewMode(m)}>
                                {m.charAt(0).toUpperCase() + m.slice(1)}
                            </button>
                        ))}
                    </span>
                    <span className="cal-spacer"/>
                    <button className="btn" onClick={() => setManagingCals(true)}>Calendars</button>
                    <button className="btn" onClick={() => void doImport()}>Import…</button>
                    <button className="btn" onClick={() => void doExport()} disabled={events.length === 0}>Export ICS</button>
                </div>

                {viewMode === 'month' ? (
                    <div className="cal-grid">
                        {WEEKDAYS.map((w) => (<div key={w} className="cal-weekday">{w}</div>))}
                        {cells.map((day, i) => {
                            const dayEvents = datedInstances.filter((p) => sameDay(p.start, day))
                            const inMonth = day.getMonth() === viewDate.getMonth()
                            return (
                                <div key={i} className={'cal-cell' + (inMonth ? '' : ' cal-cell-dim')} onClick={() => openNew(day)}>
                                    <button className="cal-daynum" title="Open day view"
                                            onClick={(ev) => {
                                                ev.stopPropagation()
                                                openDay(day)
                                            }}>{day.getDate()}</button>
                                    {dayEvents.map((p) => (
                                        <button key={`${p.i.event.id}@${p.i.start}`} className="cal-event" title={p.i.event.summary}
                                                style={{borderLeft: `3px solid ${colourOf(p.i.event)}`}}
                                                onClick={(ev) => {
                                                    ev.stopPropagation()
                                                    openInstance(p.i)
                                                }}>
                                            {p.i.event.allDay ? '' : `${pad(p.start.getHours())}:${pad(p.start.getMinutes())} `}{p.i.event.summary}
                                        </button>
                                    ))}
                                </div>
                            )
                        })}
                    </div>
                ) : (
                    <CalendarTimeGrid
                        days={viewMode === 'week' ? weekDays(viewDate) : [viewDate]}
                        instances={instances}
                        colourOf={colourOf}
                        onNewAt={openAt}
                        onEdit={openInstance}
                    />
                )}

                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose}>Close</button>
                    <button className="btn primary" onClick={() => openNew(new Date())}>New event</button>
                </div>
            </div>

            {form && (
                <div className="modal-backdrop">
                    <div className="modal event-form" role="dialog"
                         aria-label={form.id ? 'Edit event' : 'New event'} onClick={(e) => e.stopPropagation()}>
                        <ModalClose onClose={() => setForm(null)}/>
                        <h2 className="modal-title">{form.id ? 'Edit event' : 'New event'}</h2>
                        <div className="rule-form">
                            <input className="tag-name-input" placeholder="Event title" value={form.summary} autoFocus
                                   onChange={(e) => set('summary', e.target.value)}/>
                            {calendars.length > 0 && (
                                <select className="tag-name-input" aria-label="Calendar" value={form.calendarId}
                                        onChange={(e) => set('calendarId', e.target.value)}>
                                    <option value="">No calendar</option>
                                    {calendars.map((c) => (
                                        <option key={c.id} value={c.id}>{c.name}</option>
                                    ))}
                                </select>
                            )}
                            <label className="cal-allday">
                                <input type="checkbox" checked={form.allDay}
                                       onChange={(e) => set('allDay', e.target.checked)}/> All day
                            </label>
                            <div className="rule-form-row">
                                <div className="date-field">
                                    <input ref={startRef} className="tag-name-input"
                                           type={form.allDay ? 'date' : 'datetime-local'}
                                           value={form.start} onChange={(e) => set('start', e.target.value)}/>
                                    <PickerButton target={startRef}/>
                                </div>
                                <div className="date-field">
                                    <input ref={endRef} className="tag-name-input"
                                           type={form.allDay ? 'date' : 'datetime-local'}
                                           value={form.end} onChange={(e) => set('end', e.target.value)}/>
                                    <PickerButton target={endRef}/>
                                </div>
                            </div>
                            {!form.allDay && (
                                <label className="cal-tz">
                                    Time zone
                                    <select className="tag-name-input" aria-label="Time zone" value={form.timeZone}
                                            onChange={(e) => set('timeZone', e.target.value)}>
                                        {zoneOptions(form.timeZone).map((z) => (
                                            <option key={z} value={z}>{z}</option>
                                        ))}
                                    </select>
                                </label>
                            )}
                            <input className="tag-name-input" placeholder="Location" value={form.location}
                                   onChange={(e) => set('location', e.target.value)}/>
                            <textarea className="tag-name-input" placeholder="Description" rows={2} value={form.description}
                                      onChange={(e) => set('description', e.target.value)}/>
                            {form.scope === EventScope.This ? (
                                <p className="setup-hint">This change applies to this event only.</p>
                            ) : (
                                <RecurrenceEditor value={form.recurrence} onChange={(r) => set('recurrence', r)}
                                                  startDate={form.start}/>
                            )}
                            <div className="reminders">
                                <div className="reminders-head">
                                    <span>Reminders</span>
                                    <button type="button" className="btn" onClick={addReminder}>+ Add reminder</button>
                                </div>
                                {form.reminders.map((r, i) => (
                                    <div key={i} className="reminder-row">
                                        <select className="tag-name-input" aria-label="Reminder" value={r}
                                                onChange={(e) => setReminder(i, Number(e.target.value))}>
                                            {!REMINDER_PRESETS.some((p) => p.minutes === r) && (
                                                <option value={r}>{r} minutes before</option>
                                            )}
                                            {REMINDER_PRESETS.map((p) => (
                                                <option key={p.minutes} value={p.minutes}>{p.label}</option>
                                            ))}
                                        </select>
                                        <button type="button" className="btn danger" aria-label="Remove reminder"
                                                onClick={() => removeReminder(i)}>×</button>
                                    </div>
                                ))}
                            </div>
                            <div className="meeting-section">
                                <div className="reminders-head">
                                    <span>Attendees</span>
                                </div>
                                {form.attendees.length > 0 && (
                                    <p className="setup-hint">Organizer: {organizerLabel()}</p>
                                )}
                                {form.attendees.map((a, i) => (
                                    <div key={a.address} className="attendee-row">
                                        <span className="attendee-email" title={a.address}>
                                            {a.commonName || a.address}
                                        </span>
                                        <span className="attendee-status">{attendeeStatusLabel(a.status)}</span>
                                        <button type="button" className="btn danger" aria-label="Remove attendee"
                                                onClick={() => removeAttendee(i)}>×</button>
                                    </div>
                                ))}
                                <div className="attendee-add">
                                    <input className="tag-name-input" type="email" placeholder="Attendee email"
                                           value={attendeeDraft}
                                           onChange={(e) => setAttendeeDraft(e.target.value)}
                                           onKeyDown={(e) => {
                                               if (e.key === 'Enter') {
                                                   e.preventDefault()
                                                   addAttendee()
                                               }
                                           }}/>
                                    <button type="button" className="btn" onClick={addAttendee}
                                            disabled={!isAttendeeEmail(attendeeDraft.trim())}>+ Add attendee</button>
                                </div>
                                {form.attendees.length > 0 && (
                                    accountId === '' ? (
                                        <p className="setup-hint">Select an account to send the invitation to the attendees.</p>
                                    ) : (
                                        <>
                                            <p className="setup-hint">
                                                {cancelledSent
                                                    ? 'This meeting has been cancelled. The attendees have been notified.'
                                                    : `Saving ${form.id === '' ? 'this meeting' : ''} sends an invitation to the attendees by email.`}
                                            </p>
                                            {form.id !== '' && (
                                                <div className="invite-card-actions">
                                                    <button type="button" className="btn" disabled={busy || cancelledSent}
                                                            onClick={() => void sendInvitations()}>Resend invitation</button>
                                                    <button type="button" className="btn danger-outline" disabled={busy || cancelledSent}
                                                            onClick={() => setCancelMeeting(true)}>
                                                        {cancelledSent ? 'Meeting cancelled' : 'Cancel meeting'}
                                                    </button>
                                                </div>
                                            )}
                                        </>
                                    )
                                )}
                            </div>
                            {(error || status) && (
                                <div className={error ? 'compose-error' : 'setup-hint'}>{error || status}</div>
                            )}
                            <div className="modal-actions spread">
                                <span>
                                    {form.id && (
                                        <button className="btn danger" onClick={requestDelete}>Delete</button>
                                    )}
                                </span>
                                <span className="cal-form-actions">
                                    <button className="btn" onClick={() => setForm(null)}>Cancel</button>
                                    <button className="btn primary" onClick={() => void save()}
                                            disabled={busy || form.summary.trim() === '' || form.start === ''}>
                                        {busy ? 'Saving…' : primaryActionLabel()}
                                    </button>
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {pendingDelete && (
                <ConfirmDialog
                    title="Delete event"
                    message={`Delete "${pendingDelete.summary}"? This cannot be undone.`}
                    confirmLabel="Delete"
                    busy={busy}
                    onConfirm={() => void confirmDelete()}
                    onCancel={() => setPendingDelete(null)}
                />
            )}

            {cancelMeeting && form && (
                <ConfirmDialog
                    title="Cancel meeting"
                    message={`Email a cancellation for "${form.summary}" to its ${form.attendees.length} ` +
                        `attendee${form.attendees.length === 1 ? '' : 's'}? This cannot be undone.`}
                    confirmLabel="Send cancellation"
                    busy={busy}
                    onConfirm={() => void confirmCancelMeeting()}
                    onCancel={() => setCancelMeeting(false)}
                />
            )}

            {editScope && (
                <ScopeChooser
                    title="Edit recurring event"
                    message={`"${editScope.event.summary}" repeats. Which events should this change apply to?`}
                    busy={busy}
                    onChoose={chooseEditScope}
                    onCancel={() => setEditScope(null)}
                />
            )}

            {deleteScope && (
                <ScopeChooser
                    title="Delete recurring event"
                    message={`"${deleteScope.summary}" repeats. Which events should be deleted? This cannot be undone.`}
                    danger
                    busy={busy}
                    onChoose={(scope) => void confirmDeleteScope(scope)}
                    onCancel={() => setDeleteScope(null)}
                />
            )}

            {managingCals && (
                <div className="modal-backdrop">
                    <div className="modal event-form" role="dialog" aria-label="Calendars"
                         onClick={(e) => e.stopPropagation()}>
                        <ModalClose onClose={() => {
                            setManagingCals(false)
                            setCalForm(null)
                        }}/>
                        <h2 className="modal-title">Calendars</h2>
                        <p className="setup-hint">Calendars group your events and colour them across every view.</p>
                        <div className="cg-bar">
                            {calendars.map((c) => (
                                <button key={c.id} className="cg-chip"
                                        onClick={() => setCalForm({id: c.id, name: c.name, colour: c.colour || DEFAULT_EVENT_COLOUR})}>
                                    <span className="cal-swatch" style={{background: c.colour || DEFAULT_EVENT_COLOUR}}/>
                                    {c.name}
                                </button>
                            ))}
                            <button className="cg-chip"
                                    onClick={() => setCalForm({id: '', name: '', colour: DEFAULT_EVENT_COLOUR})}>
                                + New calendar
                            </button>
                        </div>
                        {calForm && (
                            <div className="rule-form">
                                <div className="rule-form-row">
                                    <input type="color" className="cal-colour" aria-label="Calendar colour"
                                           value={calForm.colour}
                                           onChange={(e) => setCalForm((cf) => cf ? {...cf, colour: e.target.value} : cf)}/>
                                    <input className="tag-name-input" placeholder="Calendar name" value={calForm.name}
                                           autoFocus
                                           onChange={(e) => setCalForm((cf) => cf ? {...cf, name: e.target.value} : cf)}/>
                                </div>
                                <div className="modal-actions spread">
                                    <span>
                                        {calForm.id && (
                                            <button className="btn danger" onClick={() => {
                                                const c = calendars.find((x) => x.id === calForm.id)
                                                if (c) setPendingCalDelete(c)
                                            }}>Delete</button>
                                        )}
                                    </span>
                                    <span className="cal-form-actions">
                                        <button className="btn" onClick={() => setCalForm(null)}>Cancel</button>
                                        <button className="btn primary" onClick={() => void saveCal()}
                                                disabled={busy || calForm.name.trim() === ''}>
                                            {busy ? 'Saving…' : (calForm.id ? 'Save calendar' : 'Add calendar')}
                                        </button>
                                    </span>
                                </div>
                            </div>
                        )}
                        {!calForm && (
                            <div className="modal-actions spread">
                                <button className="btn" onClick={() => setManagingCals(false)}>Done</button>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {pendingCalDelete && (
                <ConfirmDialog
                    title="Delete calendar"
                    message={`Delete the calendar "${pendingCalDelete.name}"? Its events are deleted too. This cannot be undone.`}
                    confirmLabel="Delete"
                    busy={busy}
                    onConfirm={() => void confirmCalDelete()}
                    onCancel={() => setPendingCalDelete(null)}
                />
            )}
        </div>
    )
}
