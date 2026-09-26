import type {Dispatch, SetStateAction} from 'react'
import {RecipientField} from './RecipientField'
import type {useContactPool} from '../hooks/useContactPool'
import type {useSeparatorCorrection} from '../hooks/useSeparatorCorrection'

interface ComposeHeaderProps {
    error: string
    correction: ReturnType<typeof useSeparatorCorrection>
    senders: {name: string; address: string}[]
    from: string
    setFrom: Dispatch<SetStateAction<string>>
    to: string
    setTo: Dispatch<SetStateAction<string>>
    cc: string
    setCc: Dispatch<SetStateAction<string>>
    bcc: string
    setBcc: Dispatch<SetStateAction<string>>
    subject: string
    setSubject: Dispatch<SetStateAction<string>>
    contacts: ReturnType<typeof useContactPool>
    // markDirty tells the draft autosave an edit was made.
    markDirty: () => void
}

// ComposeHeader is the compose window's address block, pinned above the scrolling body: the error, the
// separator correction, From (with more than one sender), To, Cc, Bcc and Subject.
export function ComposeHeader({
    error, correction, senders, from, setFrom, to, setTo, cc, setCc, bcc, setBcc, subject, setSubject, contacts, markDirty,
}: ComposeHeaderProps) {
    return (
        <div className="compose-header">
        {error && <div className="compose-error">{error}</div>}
        {correction.pending && (
            <div className="compose-correction">
                <div>Addresses should be separated by a comma or semicolon. Did you mean:</div>
                <div className="compose-correction-value">{correction.pending.preview}</div>
                <div className="compose-correction-actions">
                    <button type="button" className="btn" onClick={correction.apply}>Use this</button>
                    <button type="button" className="btn" onClick={correction.dismiss}>Dismiss</button>
                </div>
            </div>
        )}
        {senders.length > 1 && (
            <label className="field">
                <span>From</span>
                <select value={from} onChange={(e) => setFrom(e.target.value)}>
                    {senders.map((s) => (
                        <option key={s.address} value={s.address}>
                            {s.name ? `${s.name} <${s.address}>` : s.address}
                        </option>
                    ))}
                </select>
            </label>
        )}
        <RecipientField label="To" value={to} placeholder="name@example.com, other@example.com"
                        pool={contacts.pool} ensurePool={contacts.ensurePool}
                        onChange={(value) => {
                            markDirty()
                            setTo(value)
                        }}/>
        <RecipientField label="Cc" value={cc}
                        pool={contacts.pool} ensurePool={contacts.ensurePool}
                        onChange={(value) => {
                            markDirty()
                            setCc(value)
                        }}/>
        <RecipientField label="Bcc" value={bcc}
                        pool={contacts.pool} ensurePool={contacts.ensurePool}
                        onChange={(value) => {
                            markDirty()
                            setBcc(value)
                        }}/>
        <label className="field">
            <span>Subject</span>
            <input value={subject} onChange={(e) => {
                markDirty()
                setSubject(e.target.value)
            }}/>
        </label>
        </div>
    )
}
