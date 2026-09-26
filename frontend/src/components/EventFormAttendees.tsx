import type {Dispatch, SetStateAction} from 'react'
import {DEFAULT_ATTENDEE_ROLE, DEFAULT_ATTENDEE_STATUS, attendeeStatusLabel} from '../calendarModel'
import type {EventForm} from './EventFormModal'

interface EventFormAttendeesProps {
    form: EventForm
    setForm: Dispatch<SetStateAction<EventForm | null>>
    attendeeDraft: string
    setAttendeeDraft: Dispatch<SetStateAction<string>>
    accountId: string
    accountEmail: string
    accountName: string
    busy: boolean
    cancelledSent: boolean
    // sendsOnSave is whether saving will email the attendees, which the hint beneath the list says.
    sendsOnSave: boolean
    sendInvitations: () => Promise<void>
    setCancelMeeting: Dispatch<SetStateAction<boolean>>
}

// EventFormAttendees is the event form's meeting section: the invited list with each reply, the field that
// adds an address, the organiser and what saving will send; a saved meeting also offers resend and cancel.
export function EventFormAttendees({
    form, setForm, attendeeDraft, setAttendeeDraft, accountId, accountEmail, accountName, busy, cancelledSent,
    sendsOnSave, sendInvitations, setCancelMeeting,
}: EventFormAttendeesProps) {
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

    // organizerLabel is the meeting section's organiser: the loaded one; else the account a new meeting gets.
    const organizerLabel = (): string => {
        if (form.organizerAddress) return form.organizerName || form.organizerAddress
        return accountName ? `${accountName} (${accountEmail})` : accountEmail
    }

    return (
        <div className="meeting-section">
            <div className="reminders-head">
                <span>Attendees</span>
            </div>
            {form.attendees.length > 0 && (
                <p className="setup-hint">Organiser: {organizerLabel()}</p>
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
                                : sendsOnSave
                                    ? (form.id === ''
                                        ? 'Saving this meeting sends an invitation to the attendees by email.'
                                        : 'Saving sends an update to the attendees by email.')
                                    : 'Saving keeps the change local. An update is emailed only when something the attendees can see changes.'}
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
    )
}
