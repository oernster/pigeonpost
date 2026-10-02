import {useState} from 'react'
import type {Dispatch, SetStateAction} from 'react'
import {api, Calendar, EventScope} from '../api'
import {EVENT_CATEGORIES} from '../categories'
import {zoneOptions} from '../tz'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {ScopeChooser} from './ScopeChooser'
import {RecurrenceEditor} from './RecurrenceEditor'
import {DateField} from './DateField'
import {
    DEFAULT_REMINDER_MINUTES,
    REMINDER_PRESETS,
    extractUrls,
    meetingProvider,
    organisesMeeting,
} from '../calendarModel'
import type {Banners} from '../hooks/useBanners'
import {useEventFormActions} from '../hooks/useEventFormActions'
import {EventFormAttendees} from './EventFormAttendees'

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
    // organizerAddress and organizerName carry a loaded meeting's organiser. They are empty for a new
    // event; on save with attendees the organiser defaults to the active account. attendees is the invited
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

// meetingView is the slice of the form the attendees can see: the fields the outgoing iTIP REQUEST
// carries (title, times, zone, location, description, category, recurrence and the invited list).
// Reminders and the local calendar assignment are deliberately absent: they are private to this machine,
// so editing them must not email the attendees an update.
export const meetingView = (f: EventForm): string => JSON.stringify({
    summary: f.summary, description: f.description, location: f.location, category: f.category,
    allDay: f.allDay, start: f.start, end: f.end, timeZone: f.allDay ? '' : f.timeZone,
    recurrence: f.recurrence,
    attendees: f.attendees.map((a) => [a.address, a.commonName, a.role, a.rsvp]),
})

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
    // openedView is the meeting view of the form as it was opened; comparing against it on save decides
    // whether the attendees need an update at all. The modal mounts fresh for each opened event, so the
    // initializer runs exactly once per edit session.
    const [openedView] = useState(() => meetingView(form))

    const set = <K extends keyof EventForm>(key: K, value: EventForm[K]) =>
        setForm((f) => (f ? {...f, [key]: value} : f))

    const setReminder = (index: number, minutes: number) =>
        setForm((f) => (f ? {...f, reminders: f.reminders.map((r, i) => (i === index ? minutes : r))} : f))
    const addReminder = () =>
        setForm((f) => (f ? {...f, reminders: [...f.reminders, DEFAULT_REMINDER_MINUTES]} : f))
    const removeReminder = (index: number) =>
        setForm((f) => (f ? {...f, reminders: f.reminders.filter((_, i) => i !== index)} : f))

    // meetingChangedSinceOpen says whether the attendees can see any difference: a new event is always a
    // change, an existing one only when its meeting view moved since the form opened. A reminder or
    // calendar tweak leaves the view identical, so the save stays local.
    const meetingChangedSinceOpen = form.id === '' || meetingView(form) !== openedView
    // organises says whether the account organises this meeting; only the organiser emails the attendees.
    const organises = organisesMeeting(form.organizerAddress, accountEmail)
    // sendsOnSave says whether this save will email the attendees: the event is a meeting the account
    // organises, an account can send, it is not already cancelled and something the attendees can see changed.
    const sendsOnSave = form.attendees.length > 0 && accountId !== '' && organises && !cancelledSent
        && meetingChangedSinceOpen

    // primaryActionLabel names the save button. When saving will also email the attendees, the label says
    // so rather than a plain Save.
    const primaryActionLabel = (): string => {
        if (sendsOnSave) {
            return form.id ? 'Save and send update' : 'Send invitation'
        }
        return form.id ? 'Save changes' : 'Add event'
    }

    // Save, delete, resend and cancel, with the confirmations they raise, are their own hook.
    const actions = useEventFormActions({
        form, setForm, accountId, accountEmail, accountName, cancelledSent, setCancelledSent, banners,
        onChanged, bumpReload, meetingChangedSinceOpen, organises,
    })
    const {
        save, requestDelete, confirmDelete, confirmDeleteScope, confirmCancelMeeting,
        cancelMeeting, setCancelMeeting, pendingDelete, setPendingDelete, deleteScope, setDeleteScope,
    } = actions

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
                <div className="modal event-form pinned-actions" role="dialog"
                     aria-label={form.id ? 'Edit event' : 'New event'} onClick={(e) => e.stopPropagation()}>
                    <ModalClose onClose={() => setForm(null)}/>
                    <h2 className="modal-title">{form.id ? 'Edit event' : 'New event'}</h2>
                    <div className="rule-form pinned-form-header">
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
                    </div>
                    <div className="modal-body">
                    <div className="rule-form">
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
                        <EventFormAttendees
                            form={form} setForm={setForm} attendeeDraft={attendeeDraft} setAttendeeDraft={setAttendeeDraft}
                            accountId={accountId} accountEmail={accountEmail} accountName={accountName} busy={busy}
                            cancelledSent={cancelledSent} sendsOnSave={sendsOnSave} organises={organises}
                            sendInvitations={actions.sendInvitations} setCancelMeeting={actions.setCancelMeeting}/>
                        {(error || status) && (
                            <div className={error ? 'compose-error' : 'setup-hint'}>{error || status}</div>
                        )}
                    </div>
                    </div>
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
