import {useState} from 'react'
import {api, Rule, RuleInput} from '../api'
import {ModalClose} from './ModalClose'

interface RuleManagerModalProps {
    rules: Rule[]
    onChanged: () => void
    onClose: () => void
}

const FIELD_LABELS: Record<string, string> = {from: 'From', to: 'To', cc: 'Cc', subject: 'Subject'}
const OPERATOR_LABELS: Record<string, string> = {
    contains: 'contains',
    notContains: "doesn't contain",
    equals: 'is',
    startsWith: 'starts with',
    endsWith: 'ends with',
}
const ACTION_LABELS: Record<string, string> = {markRead: 'mark as read', flag: 'flag it'}

// RuleManagerModal lists filter rules and adds new ones. Rules run on each sync; a matching message
// (its chosen field compared with the operator against the text) has the chosen action applied.
export function RuleManagerModal({rules, onChanged, onClose}: RuleManagerModalProps) {
    const [name, setName] = useState('')
    const [field, setField] = useState('from')
    const [operator, setOperator] = useState('contains')
    const [contains, setContains] = useState('')
    const [action, setAction] = useState('markRead')
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)

    const add = async () => {
        setBusy(true)
        setError('')
        try {
            const req: RuleInput = {id: '', name, field, operator, contains, action}
            await api.saveRule(req)
            setName('')
            setContains('')
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const remove = async (id: string) => {
        setError('')
        try {
            await api.deleteRule(id)
            onChanged()
        } catch (e) {
            setError(String(e))
        }
    }

    return (
        <div className="modal-backdrop" onClick={onClose}>
            <div className="modal" role="dialog" aria-label="Filter rules" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Filter rules</h2>
                <p className="setup-hint">Rules run on each sync. When a message matches, its action is applied.</p>
                {error && <div className="compose-error">{error}</div>}
                {rules.length === 0 ? (
                    <p className="empty-body">No rules yet.</p>
                ) : (
                    <ul className="list">
                        {rules.map((r) => (
                            <li key={r.id} className="list-item">
                                <span className="item-text">
                                    <span className="item-title">{r.name}</span>
                                    <span className="item-sub">
                                        If {FIELD_LABELS[r.field] ?? r.field} {OPERATOR_LABELS[r.operator] ?? r.operator} "{r.contains}", {ACTION_LABELS[r.action] ?? r.action}
                                    </span>
                                </span>
                                <button
                                    className="account-action delete"
                                    aria-label={`Delete ${r.name}`}
                                    title="Delete rule"
                                    onClick={() => void remove(r.id)}
                                >
                                    &times;
                                </button>
                            </li>
                        ))}
                    </ul>
                )}
                <div className="rule-form">
                    <input
                        className="tag-name-input"
                        placeholder="Rule name"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                    />
                    <div className="rule-form-row">
                        <select value={field} onChange={(e) => setField(e.target.value)}>
                            <option value="from">From</option>
                            <option value="to">To</option>
                            <option value="cc">Cc</option>
                            <option value="subject">Subject</option>
                        </select>
                        <select value={operator} onChange={(e) => setOperator(e.target.value)}>
                            <option value="contains">contains</option>
                            <option value="notContains">doesn't contain</option>
                            <option value="equals">is</option>
                            <option value="startsWith">starts with</option>
                            <option value="endsWith">ends with</option>
                        </select>
                        <input
                            className="tag-name-input"
                            placeholder="text to match"
                            value={contains}
                            onChange={(e) => setContains(e.target.value)}
                        />
                    </div>
                    <div className="rule-form-row">
                        <span>then</span>
                        <select value={action} onChange={(e) => setAction(e.target.value)}>
                            <option value="markRead">Mark as read</option>
                            <option value="flag">Flag</option>
                        </select>
                    </div>
                </div>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose}>Close</button>
                    <button
                        className="btn primary"
                        onClick={() => void add()}
                        disabled={busy || name.trim() === '' || contains.trim() === ''}
                    >
                        {busy ? 'Adding...' : 'Add rule'}
                    </button>
                </div>
            </div>
        </div>
    )
}
