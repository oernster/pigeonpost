import {useState} from 'react'
import {api, CalendarEvent, CalendarEventInput} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const MONTHS = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
]

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

interface CalendarModalProps {
    events: CalendarEvent[]
    onChanged: () => void
    onClose: () => void
}

// CalendarModal shows a month view of events and edits them. It imports and exports iCalendar (.ics) so
// events round-trip with Outlook and Thunderbird. Deletion is always confirmed.
export function CalendarModal({events, onChanged, onClose}: CalendarModalProps) {
    const [viewDate, setViewDate] = useState(() => new Date())
    const [form, setForm] = useState<EventForm | null>(null)
    const [pendingDelete, setPendingDelete] = useState<CalendarEvent | null>(null)
    const [error, setError] = useState('')
    const [status, setStatus] = useState('')
    const [busy, setBusy] = useState(false)

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
            id: '', uid: '', calendarId: '', summary: '', description: '', location: '',
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

    const shiftMonth = (delta: number) =>
        setViewDate((d) => new Date(d.getFullYear(), d.getMonth() + delta, 1))

    return (
        <div className="modal-backdrop" onClick={onClose}>
            <div className="modal calendar-modal" role="dialog" aria-label="Calendar" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Calendar</h2>
                {error && <div className="compose-error">{error}</div>}
                {status && <div className="setup-hint">{status}</div>}

                <div className="modal-actions cal-toolbar">
                    <button className="btn" aria-label="Previous month" onClick={() => shiftMonth(-1)}>‹</button>
                    <span className="cal-month">{MONTHS[viewDate.getMonth()]} {viewDate.getFullYear()}</span>
                    <button className="btn" aria-label="Next month" onClick={() => shiftMonth(1)}>›</button>
                    <button className="btn" onClick={() => setViewDate(new Date())}>Today</button>
                    <span className="cal-spacer"/>
                    <button className="btn" onClick={() => void doImport()}>Import…</button>
                    <button className="btn" onClick={() => void doExport()} disabled={events.length === 0}>Export ICS</button>
                </div>

                <div className="cal-grid">
                    {WEEKDAYS.map((w) => (<div key={w} className="cal-weekday">{w}</div>))}
                    {cells.map((day, i) => {
                        const dayEvents = dated.filter((p) => sameDay(p.start, day))
                        const inMonth = day.getMonth() === viewDate.getMonth()
                        return (
                            <div key={i} className={'cal-cell' + (inMonth ? '' : ' cal-cell-dim')} onClick={() => openNew(day)}>
                                <div className="cal-daynum">{day.getDate()}</div>
                                {dayEvents.map((p) => (
                                    <button key={p.e.id} className="cal-event" title={p.e.summary}
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

                {form && (
                    <div className="rule-form">
                        <input className="tag-name-input" placeholder="Event title" value={form.summary}
                               onChange={(e) => set('summary', e.target.value)}/>
                        <label className="cal-allday">
                            <input type="checkbox" checked={form.allDay}
                                   onChange={(e) => set('allDay', e.target.checked)}/> All day
                        </label>
                        <div className="rule-form-row">
                            <input className="tag-name-input" type={form.allDay ? 'date' : 'datetime-local'}
                                   value={form.start} onChange={(e) => set('start', e.target.value)}/>
                            <input className="tag-name-input" type={form.allDay ? 'date' : 'datetime-local'}
                                   value={form.end} onChange={(e) => set('end', e.target.value)}/>
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
                )}

                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose}>Close</button>
                    <button className="btn primary" onClick={() => openNew(new Date())}>New event</button>
                </div>
            </div>

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
        </div>
    )
}
