import {useEffect, useState} from 'react'
import {api, Invitation, PartStat} from '../api'

interface InviteCardProps {
    messageId: string
    // onActed runs after an action changes the calendar (accept, decline, cancel or apply-reply), so the
    // surrounding view can refresh. It is optional: the reader does not need to react.
    onActed?: () => void
}

// STATUS_LABELS maps an ICS PARTSTAT value to a human label for display.
const STATUS_LABELS: Record<string, string> = {
    'ACCEPTED': 'Accepted',
    'DECLINED': 'Declined',
    'TENTATIVE': 'Tentative',
    'NEEDS-ACTION': 'No response yet',
    'DELEGATED': 'Delegated',
}

// statusLabel returns the human label for a PARTSTAT value, falling back to the raw value.
function statusLabel(status: string): string {
    return STATUS_LABELS[status] || status
}

// formatWhen renders an event's start, and its end when present, in the user's locale.
function formatWhen(start: string, end: string): string {
    if (!start) {
        return ''
    }
    const from = new Date(start).toLocaleString()
    if (!end) {
        return from
    }
    return `${from} to ${new Date(end).toLocaleString()}`
}

// InviteCard shows the meeting invitation a message carries and the actions for it: accept, tentative or
// decline a request, remove a cancelled meeting, or apply an incoming reply. It fetches the invitation
// itself from the message id, so the reader only needs to know a message has one.
export function InviteCard({messageId, onActed}: InviteCardProps) {
    const [invite, setInvite] = useState<Invitation | null>(null)
    const [myStatus, setMyStatus] = useState('')
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)

    useEffect(() => {
        let active = true
        setInvite(null)
        setError('')
        api.getInvitation(messageId)
            .then((inv) => {
                if (active) {
                    setInvite(inv)
                    setMyStatus(inv.myStatus)
                }
            })
            .catch((e) => {
                if (active) {
                    setError(String(e))
                }
            })
        return () => {
            active = false
        }
    }, [messageId])

    if (error && !invite) {
        return (
            <div className="invite-card">
                <p className="invite-card-note">Could not read the invitation: {error}</p>
            </div>
        )
    }
    if (!invite) {
        return (
            <div className="invite-card">
                <p className="invite-card-note">Reading invitation…</p>
            </div>
        )
    }

    const respond = (status: PartStat) => {
        setBusy(true)
        setError('')
        api.respondToInvitation(messageId, status)
            .then(() => {
                setMyStatus(status)
                onActed?.()
            })
            .catch((e) => setError(String(e)))
            .finally(() => setBusy(false))
    }

    const runAction = (action: () => Promise<void>) => {
        setBusy(true)
        setError('')
        action()
            .then(() => onActed?.())
            .catch((e) => setError(String(e)))
            .finally(() => setBusy(false))
    }

    const event = invite.event
    const responder = invite.attendees.length > 0 ? invite.attendees[0] : null

    return (
        <div className="invite-card">
            <p className="invite-card-title">{event.summary || '(no title)'}</p>
            <div className="invite-card-row">
                <span className="invite-card-label">When</span>
                <span>{formatWhen(event.start, event.end)}</span>
            </div>
            {event.location && (
                <div className="invite-card-row">
                    <span className="invite-card-label">Where</span>
                    <span>{event.location}</span>
                </div>
            )}
            {invite.organizer.address && (
                <div className="invite-card-row">
                    <span className="invite-card-label">Organizer</span>
                    <span>{invite.organizer.commonName || invite.organizer.address}</span>
                </div>
            )}
            {invite.attendees.length > 0 && (
                <div className="invite-card-row">
                    <span className="invite-card-label">Attendees</span>
                    <span>
                        {invite.attendees.map((a, i) => (
                            <span key={a.address}>
                                {i > 0 ? ', ' : ''}
                                {a.commonName || a.address}
                                <span className="invite-card-attendee-status"> ({statusLabel(a.status)})</span>
                            </span>
                        ))}
                    </span>
                </div>
            )}

            {invite.method === 'REQUEST' && (
                <>
                    <div className="invite-card-actions">
                        <button className="btn primary" disabled={busy} onClick={() => respond('ACCEPTED')}>Accept</button>
                        <button className="btn" disabled={busy} onClick={() => respond('TENTATIVE')}>Tentative</button>
                        <button className="btn danger-outline" disabled={busy} onClick={() => respond('DECLINED')}>Decline</button>
                    </div>
                    {myStatus && myStatus !== 'NEEDS-ACTION' && (
                        <p className="invite-card-note">Your response: {statusLabel(myStatus)}</p>
                    )}
                </>
            )}

            {invite.method === 'CANCEL' && (
                <>
                    <p className="invite-card-note">This meeting has been cancelled.</p>
                    <div className="invite-card-actions">
                        <button
                            className="btn danger-outline"
                            disabled={busy}
                            onClick={() => runAction(() => api.removeCancelledMeeting(messageId))}
                        >
                            Remove from calendar
                        </button>
                    </div>
                </>
            )}

            {invite.method === 'REPLY' && (
                <>
                    <p className="invite-card-note">
                        {responder
                            ? `${responder.commonName || responder.address} responded: ${statusLabel(responder.status)}`
                            : 'A reply to your meeting.'}
                    </p>
                    <div className="invite-card-actions">
                        <button
                            className="btn primary"
                            disabled={busy}
                            onClick={() => runAction(() => api.applyMeetingReply(messageId))}
                        >
                            Update meeting
                        </button>
                    </div>
                </>
            )}

            {invite.method === 'PUBLISH' && (
                <p className="invite-card-note">Shared calendar event.</p>
            )}

            {error && <p className="invite-card-note">Something went wrong: {error}</p>}
        </div>
    )
}
