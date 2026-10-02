import {useState} from 'react'
import type {Dispatch, SetStateAction} from 'react'
import {api, CalendarEventInput, EventScope} from '../api'
import {zonedWallToISO} from '../tz'
import type {EventForm} from '../components/EventFormModal'
import type {Banners} from './useBanners'

export interface EventFormActionsDeps {
    form: EventForm
    setForm: Dispatch<SetStateAction<EventForm | null>>
    accountId: string
    accountEmail: string
    accountName: string
    cancelledSent: boolean
    setCancelledSent: Dispatch<SetStateAction<boolean>>
    banners: Banners
    onChanged: () => void
    bumpReload: () => void
    // meetingChangedSinceOpen says whether the attendees can see any difference since the form opened, which
    // decides whether a save emails them an update.
    meetingChangedSinceOpen: boolean
    // organises says whether the account organises the meeting. An attendee's copy of someone else's meeting
    // is never emailed out from here: that would invite the other attendees from the wrong person.
    organises: boolean
}

// useEventFormActions is everything the event form does to the calendar and the attendees: save (sending
// the invitation or update when a meeting's visible details changed), delete (asking the scope for a
// recurring event), resend the invitation and cancel the meeting. It owns the confirmations those raise.
export function useEventFormActions(deps: EventFormActionsDeps) {
    const {
        form, setForm, accountId, accountEmail, accountName, cancelledSent, setCancelledSent, banners,
        onChanged, bumpReload, meetingChangedSinceOpen, organises,
    } = deps
    const {setError, setStatus, setBusy} = banners
    const [cancelMeeting, setCancelMeeting] = useState(false)
    const [pendingDelete, setPendingDelete] = useState<{id: string; summary: string} | null>(null)
    const [deleteScope, setDeleteScope] = useState<{seriesId: string; occurrence: string; summary: string} | null>(null)

    const toISO = (value: string): string => (value ? new Date(value).toISOString() : '')

    const save = async () => {
        setBusy(true)
        setError('')
        try {
            // A timed event's wall time is interpreted in its chosen zone; an all-day date is floating and
            // carries no zone.
            const startISO = form.allDay ? toISO(form.start) : zonedWallToISO(form.start, form.timeZone)
            const endISO = form.allDay ? toISO(form.end) : (form.end ? zonedWallToISO(form.end, form.timeZone) : '')
            // A meeting (any attendees) needs an organiser to be replied to: the loaded one, else the active
            // account when newly organising. With no attendees it stays a plain entry with no organiser.
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
            // same way a calendar app's meeting Send both saves and notifies. Re-saving sends an update,
            // but only when something the attendees can see changed: a reminder or calendar tweak is a
            // local detail and saving it must not email anyone.
            if (hasAttendees && !cancelledSent) {
                if (!organises) {
                    console.info('meeting invite: not sending, the account does not organise this meeting', {savedId})
                    setStatus('Saved to your calendar. Only the organiser emails the attendees, so nobody was emailed.')
                    setForm(null)
                } else if (accountId === '') {
                    console.warn('meeting invite: not sending, no account selected', {savedId})
                    setStatus('Meeting saved. Select an account to send the invitation to the attendees.')
                } else if (!meetingChangedSinceOpen) {
                    console.info('meeting invite: not sending, no attendee-visible change', {savedId})
                    setStatus('Saved. The attendees were not emailed: nothing they can see changed.')
                    setForm(null)
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
            // organiser's own calendar too, the way a calendar app deletes a meeting you cancel.
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

    return {
        save, requestDelete, confirmDelete, confirmDeleteScope, sendInvitations, confirmCancelMeeting,
        cancelMeeting, setCancelMeeting, pendingDelete, setPendingDelete, deleteScope, setDeleteScope,
    }
}
