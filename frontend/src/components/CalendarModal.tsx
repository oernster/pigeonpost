import {type RefObject, useEffect, useRef, useState} from 'react'
import {api, Calendar, CalendarEvent, CalendarEventInput} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
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
    recurrence: string
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

// PickerButton opens a date field's native calendar. The browser's own picker icon is hidden because its
// focus state cannot be styled reliably, so this is a normal focusable button instead: a white glyph at
// rest, and a teal square when hovered or focused, so it is obvious when it holds keyboard focus.
function PickerButton({target}: {target: RefObject<HTMLInputElement>}) {
    return (
        <button
            type="button"
            className="date-picker-btn"
            aria-label="Open the date picker"
            onClick={() => target.current?.showPicker()}
        >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor"
                 strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <rect x="3" y="4" width="18" height="18" rx="2"/>
                <line x1="16" y1="2" x2="16" y2="6"/>
                <line x1="8" y1="2" x2="8" y2="6"/>
                <line x1="3" y1="10" x2="21" y2="10"/>
            </svg>
        </button>
    )
}

interface CalendarModalProps {
    events: CalendarEvent[]
    onChanged: () => void
    onClose: () => void
}

// CalendarModal shows a month view of events and edits them. It imports and exports iCalendar (.ics) so
// events round-trip with Outlook and Thunderbird. Deletion is always confirmed.
export function CalendarModal({events, onChanged, onClose}: CalendarModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const startRef = useRef<HTMLInputElement>(null)
    const endRef = useRef<HTMLInputElement>(null)
    const [viewDate, setViewDate] = useState(() => new Date())
    const [viewMode, setViewMode] = useState<ViewMode>('month')
    const [form, setForm] = useState<EventForm | null>(null)
    const [pendingDelete, setPendingDelete] = useState<CalendarEvent | null>(null)
    const [error, setError] = useState('')
    const [status, setStatus] = useState('')
    const [busy, setBusy] = useState(false)
    const [calendars, setCalendars] = useState<Calendar[]>([])
    const [managingCals, setManagingCals] = useState(false)
    const [calForm, setCalForm] = useState<{id: string; name: string; colour: string} | null>(null)
    const [pendingCalDelete, setPendingCalDelete] = useState<Calendar | null>(null)

    const reloadCalendars = () =>
        void api.listCalendars().then(setCalendars).catch((e) => setError(String(e)))
    useEffect(() => {
        reloadCalendars()
    }, [])

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
    const dated = events.map((e) => ({e, start: new Date(e.start)}))

    const set = <K extends keyof EventForm>(key: K, value: EventForm[K]) =>
        setForm((f) => (f ? {...f, [key]: value} : f))

    const openNew = (day: Date) => {
        const start = new Date(day)
        start.setHours(9, 0, 0, 0)
        const end = new Date(start)
        end.setHours(10, 0, 0, 0)
        setForm({
            id: '', uid: '', calendarId: defaultCalendarId(), summary: '', description: '', location: '',
            allDay: false, start: dateTimeInput(start), end: dateTimeInput(end), recurrence: '',
        })
    }

    // openAt starts a new one-hour event at the clicked time in the week or day time-grid.
    const openAt = (start: Date) => {
        const end = new Date(start)
        end.setHours(start.getHours() + HOURS_PER_EVENT)
        setForm({
            id: '', uid: '', calendarId: defaultCalendarId(), summary: '', description: '', location: '',
            allDay: false, start: dateTimeInput(start), end: dateTimeInput(end), recurrence: '',
        })
    }

    const openEdit = (e: CalendarEvent) => {
        const start = new Date(e.start)
        const end = e.end ? new Date(e.end) : null
        setForm({
            id: e.id, uid: e.uid, calendarId: e.calendarId, summary: e.summary,
            description: e.description, location: e.location, allDay: e.allDay,
            start: e.allDay ? dateInput(start) : dateTimeInput(start),
            end: end ? (e.allDay ? dateInput(end) : dateTimeInput(end)) : '',
            recurrence: e.recurrence,
        })
    }

    const toISO = (value: string): string => (value ? new Date(value).toISOString() : '')

    const save = async () => {
        if (!form) return
        setBusy(true)
        setError('')
        try {
            const req: CalendarEventInput = {
                id: form.id, uid: form.uid, calendarId: form.calendarId, summary: form.summary,
                description: form.description, location: form.location, allDay: form.allDay,
                start: toISO(form.start), end: toISO(form.end), recurrence: form.recurrence,
            }
            await api.saveEvent(req)
            setForm(null)
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const confirmDelete = async () => {
        if (!pendingDelete) return
        setBusy(true)
        setError('')
        try {
            await api.deleteEvent(pendingDelete.id)
            if (form && form.id === pendingDelete.id) setForm(null)
            setPendingDelete(null)
            onChanged()
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
                            const dayEvents = dated.filter((p) => sameDay(p.start, day))
                            const inMonth = day.getMonth() === viewDate.getMonth()
                            return (
                                <div key={i} className={'cal-cell' + (inMonth ? '' : ' cal-cell-dim')} onClick={() => openNew(day)}>
                                    <button className="cal-daynum" title="Open day view"
                                            onClick={(ev) => {
                                                ev.stopPropagation()
                                                openDay(day)
                                            }}>{day.getDate()}</button>
                                    {dayEvents.map((p) => (
                                        <button key={p.e.id} className="cal-event" title={p.e.summary}
                                                style={{borderLeft: `3px solid ${colourOf(p.e)}`}}
                                                onClick={(ev) => {
                                                    ev.stopPropagation()
                                                    openEdit(p.e)
                                                }}>
                                            {p.e.allDay ? '' : `${pad(p.start.getHours())}:${pad(p.start.getMinutes())} `}{p.e.summary}
                                        </button>
                                    ))}
                                </div>
                            )
                        })}
                    </div>
                ) : (
                    <CalendarTimeGrid
                        days={viewMode === 'week' ? weekDays(viewDate) : [viewDate]}
                        events={events}
                        colourOf={colourOf}
                        onNewAt={openAt}
                        onEdit={openEdit}
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
                            <input className="tag-name-input" placeholder="Location" value={form.location}
                                   onChange={(e) => set('location', e.target.value)}/>
                            <textarea className="tag-name-input" placeholder="Description" rows={2} value={form.description}
                                      onChange={(e) => set('description', e.target.value)}/>
                            <div className="modal-actions spread">
                                <span>
                                    {form.id && (
                                        <button className="btn danger" onClick={() => {
                                            const ev = events.find((x) => x.id === form.id)
                                            if (ev) setPendingDelete(ev)
                                        }}>Delete</button>
                                    )}
                                </span>
                                <span className="cal-form-actions">
                                    <button className="btn" onClick={() => setForm(null)}>Cancel</button>
                                    <button className="btn primary" onClick={() => void save()}
                                            disabled={busy || form.summary.trim() === '' || form.start === ''}>
                                        {busy ? 'Saving…' : (form.id ? 'Save changes' : 'Add event')}
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
