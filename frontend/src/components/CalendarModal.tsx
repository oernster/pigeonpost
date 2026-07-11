import {useState} from 'react'
import {api, CalendarEvent, CalendarEventInstance, EventScope} from '../api'
import {browserZone, instantToZonedWall} from '../tz'
import {ModalClose} from './ModalClose'
import {ScopeChooser} from './ScopeChooser'
import {CalendarTimeGrid} from './CalendarTimeGrid'
import {useBackdropDismiss, useEscapeToClose} from './useBackdropDismiss'
import {
    DAYS_IN_WEEK,
    DEFAULT_EVENT_COLOUR,
    HOURS_PER_EVENT,
    MONTHS,
    MONTHS_SHORT,
    VIEW_MODES,
    WEEKDAYS,
    WEEKDAYS_FULL,
    dateInput,
    dateTimeInput,
    monthCells,
    pad,
    sameDay,
    weekDays,
    type ViewMode,
} from '../calendarModel'
import {useEventInstances} from '../hooks/useEventInstances'
import {useCalendars} from '../hooks/useCalendars'
import {useOpenFromReminder} from '../hooks/useOpenFromReminder'
import {CalendarsManager} from './CalendarsManager'
import {EventFormModal, type EventForm} from './EventFormModal'
import {useBanners} from '../hooks/useBanners'


interface CalendarModalProps {
    events: CalendarEvent[]
    // accountId, accountEmail and accountName identify the active account that organizes meetings: the
    // organizer written onto a meeting and the sender of its invitations. accountId is empty when no
    // account is selected, which disables sending.
    accountId: string
    accountEmail: string
    accountName: string
    // initialEventId, when set, opens the calendar with that event's dialog already showing. It is how a
    // clicked reminder lands on the event it is about.
    initialEventId?: string
    onChanged: () => void
    onClose: () => void
}

// CalendarModal shows a month view of events and edits them. It imports and exports iCalendar (.ics) so
// events round-trip with Outlook and Thunderbird. An event with attendees is a meeting: its invitations
// and cancellations are emailed through the active account. Deletion is always confirmed.
export function CalendarModal({events, accountId, accountEmail, accountName, initialEventId, onChanged, onClose}: CalendarModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const [viewDate, setViewDate] = useState(() => new Date())
    const [viewMode, setViewMode] = useState<ViewMode>('month')
    const [form, setForm] = useState<EventForm | null>(null)
    // attendeeDraft holds the email being typed into the add-attendee field.
    const [attendeeDraft, setAttendeeDraft] = useState('')
    // cancelledSent is true once a cancellation has been emailed for the open meeting, so the cancel and
    // resend actions are disabled: a withdrawn meeting must not be cancelled again or re-invited.
    const [cancelledSent, setCancelledSent] = useState(false)
    const [editScope, setEditScope] = useState<CalendarEventInstance | null>(null)
    // error, status and busy are the shared user-feedback banners, owned in one hook and read and driven by
    // the calendar shell, the event form and the calendars manager alike.
    const banners = useBanners()
    const {error, status, busy, setError, setStatus, setBusy} = banners
    // The calendars sub-feature (the list, the manager's open state, the calendar being edited and the one
    // pending deletion, plus save and delete) is its own hook; the list colours events and seeds a new event.
    const {
        calendars, managingCals, setManagingCals, calForm, setCalForm, saveCal,
        pendingCalDelete, setPendingCalDelete, confirmCalDelete,
    } = useCalendars({setError, setBusy, onChanged})
    // instances are the concrete occurrences shown for the visible range, expanded from the recurring events
    // by the backend and refetched by the application hook; bumpReload forces a refetch after a local change.
    const {instances, bumpReload} = useEventInstances({viewDate, viewMode, events, setError})

    // The event form and the calendars manager are nested modals that are deliberately not dismissed by a
    // backdrop click (so edits are not dropped), so give each its own Escape close. The active flags mean
    // Escape closes whichever is open before falling through to close the calendar itself.
    useEscapeToClose(() => setForm(null), form !== null)
    useEscapeToClose(() => setManagingCals(false), managingCals)

    // colourOf resolves an event's colour from its calendar, falling back to the default for events with
    // no calendar. The map is rebuilt each render, which is cheap for the handful of calendars a user has.
    const colourById = new Map(calendars.map((c) => [c.id, c.colour || DEFAULT_EVENT_COLOUR]))
    const colourOf = (e: CalendarEvent) => colourById.get(e.calendarId) ?? DEFAULT_EVENT_COLOUR
    const defaultCalendarId = () => calendars[0]?.id ?? ''

    const cells = monthCells(viewDate)
    const datedInstances = instances.map((i) => ({i, start: new Date(i.start)}))
    // isSeries reports whether an occurrence belongs to a recurring series, so an edit or delete asks how
    // far it should reach. A one-off event carries neither a rule nor a recurrence id.
    const isSeries = (i: CalendarEventInstance) => i.recurrenceId !== '' || i.event.recurrence !== ''

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

    // A clicked reminder lands on the event it is about: the hook jumps the view to the event and reveals its
    // dialog once the occurrence has loaded. A recurring event opens at series scope so a save reaches the
    // master; a one-off opens directly.
    useOpenFromReminder({
        initialEventId, events, instances, setViewDate,
        onReveal: (inst) => openForm(inst, isSeries(inst) ? EventScope.All : null),
    })

    const chooseEditScope = (scope: EventScope) => {
        if (editScope) openForm(editScope, scope)
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
                <EventFormModal
                    form={form}
                    setForm={setForm}
                    calendars={calendars}
                    accountId={accountId}
                    accountEmail={accountEmail}
                    accountName={accountName}
                    attendeeDraft={attendeeDraft}
                    setAttendeeDraft={setAttendeeDraft}
                    cancelledSent={cancelledSent}
                    setCancelledSent={setCancelledSent}
                    banners={banners}
                    onChanged={onChanged}
                    bumpReload={bumpReload}
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

            {managingCals && (
                <CalendarsManager
                    calendars={calendars}
                    calForm={calForm}
                    setCalForm={setCalForm}
                    pendingCalDelete={pendingCalDelete}
                    setPendingCalDelete={setPendingCalDelete}
                    saveCal={saveCal}
                    confirmCalDelete={confirmCalDelete}
                    onClose={() => {
                        setManagingCals(false)
                        setCalForm(null)
                    }}
                    busy={busy}
                />
            )}
        </div>
    )
}
