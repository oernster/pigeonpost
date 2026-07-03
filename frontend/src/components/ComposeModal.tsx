import {useState} from 'react'
import {api, ComposeInput} from '../api'

interface ComposeModalProps {
    accountId: string
    onClose: () => void
}

function splitAddresses(value: string): string[] {
    return value.split(',').map((part) => part.trim()).filter(Boolean)
}

export function ComposeModal({accountId, onClose}: ComposeModalProps) {
    const [to, setTo] = useState('')
    const [cc, setCc] = useState('')
    const [subject, setSubject] = useState('')
    const [body, setBody] = useState('')
    const [sending, setSending] = useState(false)
    const [error, setError] = useState('')

    const send = async () => {
        setSending(true)
        setError('')
        const req: ComposeInput = {
            accountId,
            to: splitAddresses(to),
            cc: splitAddresses(cc),
            subject,
            body,
        }
        try {
            await api.send(req)
            onClose()
        } catch (e) {
            setError(String(e))
            setSending(false)
        }
    }

    return (
        <div className="modal-backdrop" onClick={onClose}>
            <div className="modal compose" role="dialog" aria-label="New message" onClick={(e) => e.stopPropagation()}>
                <h2 className="modal-title">New message</h2>
                {error && <div className="compose-error">{error}</div>}
                <label className="field">
                    <span>To</span>
                    <input value={to} onChange={(e) => setTo(e.target.value)} placeholder="name@example.com, other@example.com"/>
                </label>
                <label className="field">
                    <span>Cc</span>
                    <input value={cc} onChange={(e) => setCc(e.target.value)}/>
                </label>
                <label className="field">
                    <span>Subject</span>
                    <input value={subject} onChange={(e) => setSubject(e.target.value)}/>
                </label>
                <textarea
                    className="compose-body"
                    value={body}
                    rows={12}
                    onChange={(e) => setBody(e.target.value)}
                    placeholder="Write your message..."
                />
                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose} disabled={sending}>Cancel</button>
                    <button className="btn primary" onClick={() => void send()} disabled={sending || to.trim() === ''}>
                        {sending ? 'Sending...' : 'Send'}
                    </button>
                </div>
            </div>
        </div>
    )
}
