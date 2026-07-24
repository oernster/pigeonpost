import {useState} from 'react'
import type {Dispatch, SetStateAction} from 'react'
import {api, Calendar, CalendarEventInput, EventScope} from '../api'
import {EVENT_CATEGORIES} from '../categories'
import {zonedWallToISO, zoneOptions} from '../tz'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {ScopeChooser} from './ScopeChooser'
import {RecurrenceEditor} from './RecurrenceEditor'
import {DateField} from './DateField'
import {
    DEFAULT_ATTENDEE_ROLE,
    DEFAULT_ATTENDEE_STATUS,
    DEFAULT_REMINDER_MINUTES,
    REMINDER_PRESETS,
    attendeeStatusLabel,
    extractUrls,
    meetingProvider,
} from '../calendarModel'
import type {Banners} from '../hooks/useBanners'

// AttendeeRow is one invited party held in the edit form. It mirrors the fields the backend persists so a
// loaded meeting round-trips its attendees' roles and reply statuses unchanged.
export interface AttendeeRow {
    address: string
    commonName: string
    role: string
    status: string
    rsvp: boolean
}

export interface EventForm {
    id: string
    uid: string
    calendarId: string
    summary: string
    description: string
    location: string
    // category is the optional event category value (empty means none); it is picked from EVENT_CATEGORIES.
    category: string
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

interface EventFormModalProps {
    form: EventForm
    setForm: Dispatch<SetStateAction<EventForm | null>>
    calendars: Calendar[]
    accountId: string
    accountEmail: string
    accountName: string
    attendeeDraft: string
    setAttendeeDraft: Dispatch<SetStateAction<string>>
    cancelledSent: boolean
    setCancelledSent: Dispatch<SetStateAction<boolean>>
    banners: Banners
    onChanged: () => void
    bumpReload: () => void
}

// EventFormModal is the calendar's event editor: the title, calendar, times, timezone, location, description,
// meeting links, recurrence, reminders and attendees of one event, plus its save, its delete and the meeting
// invite and cancellation. It owns the delete and cancel confirmations and the recurring-series delete scope.
// The form working state, the shared banners (error, status, busy) and the reload callbacks are injected; the
// scope decision for a recurring edit is made before the form opens, in the calendar.
export function EventFormModal({
    form, setForm, calendars, accountId, accountEmail, accountName, attendeeDraft, setAttendeeDraft,
    cancelledSent, setCancelledSent, banners, onChanged, bumpReload,
}: EventFormModalProps) {
    const {error, status, busy, setError, setStatus, setBusy} = banners
    const [cancelMeeting, setCancelMeeting] = useState(false)
    const [pendingDelete, setPendingDelete] = useState<{id: string; summary: string} | null>(null)
    const [deleteScope, setDeleteScope] = useState<{seriesId: string; occurrence: string; summary: string} | null>(null)

    const set = <K extends keyof EventForm>(key: K, value: EventForm[K]) =>
        setForm((f) => (f ? {...f, [key]: value} : f))

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
        if (form.organizerAddress) return form.organizerName || form.organizerAddress
        return accountName ? `${accountName} (${accountEmail})` : accountEmail
    }

    // primaryActionLabel names the save button. For a meeting with a usable account, saving also emails the
    // invitation to the attendees, so the label says so rather than a plain Save.
    const primaryActionLabel = (): string => {
        if (form.attendees.length > 0 && accountId !== '' && !cancelledSent) {
            return form.id ? 'Save and send update' : 'Send invitation'
        }
        return form.id ? 'Save changes' : 'Add event'
    }

    const toISO = (value: string): string => (value ? new Date(value).toISOString() : '')

    const save = async () => {
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
                description: form.description, location: form.location, category: form.category,
                allDay: form.allDay,
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
            if (form.id === pendingDelete.id) setForm(null)
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
        if (form.id === '' || accountId === '') return
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
        if (form.id === '' || accountId === '') return
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

    // Meeting links for the open event: the join URL is the first known-provider link across the location
    // and description, falling back to the first location URL (often the venue or meeting link). The
    // remaining description links are offered separately so every link in the event is clickable.
    const locationUrls = extractUrls(form.location)
    const descriptionUrls = extractUrls(form.description)
    const joinUrl = [...locationUrls, ...descriptionUrls].find((u) => meetingProvider(u)) ?? locationUrls[0] ?? null
    const joinLabel = joinUrl ? (meetingProvider(joinUrl) ?? 'meeting') : ''
    const otherLinks = [...new Set(descriptionUrls)].filter((u) => u !== joinUrl)

    return (
        <>
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
                            <DateField kind={form.allDay ? 'date' : 'datetime-local'} ariaLabel="Start"
                                       pickerTitle="Start date" value={form.start}
                                       onChange={(v) => set('start', v)}/>
                            <DateField kind={form.allDay ? 'date' : 'datetime-local'} ariaLabel="End"
                                       pickerTitle="End date" value={form.end}
                                       onChange={(v) => set('end', v)}/>
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
                        <label className="cal-category">
                            Category
                            <select className="tag-name-input" aria-label="Category" value={form.category}
                                    onChange={(e) => set('category', e.target.value)}>
                                <option value="">None</option>
                                {EVENT_CATEGORIES.map((c) => (
                                    <option key={c.value} value={c.value}>{`${c.emoji} ${c.label}`}</option>
                                ))}
                            </select>
                        </label>
                        <textarea className="tag-name-input" placeholder="Description" rows={2} value={form.description}
                                  onChange={(e) => set('description', e.target.value)}/>
                        {(joinUrl || otherLinks.length > 0) && (
                            <div className="cal-links">
                                {joinUrl && (
                                    <button type="button" className="btn primary cal-join"
                                            onClick={() => void api.openExternal(joinUrl)}>
                                        {`Join ${joinLabel}`}
                                    </button>
                                )}
                                {otherLinks.map((u) => (
                                    <a key={u} className="cal-link" href={u} title={u}
                                       onClick={(e) => {
                                           e.preventDefault()
                                           void api.openExternal(u)
                                       }}>
                                        {u}
                                    </a>
                                ))}
                            </div>
                        )}
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

            {cancelMeeting && (
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
        </>
    )
}
