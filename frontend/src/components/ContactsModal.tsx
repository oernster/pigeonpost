import {useState} from 'react'
import {api, Contact, ContactInput, ContactEmailInput, ContactPhoneInput} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'

interface ContactsModalProps {
    contacts: Contact[]
    onChanged: () => void
    onClose: () => void
}

interface ContactForm {
    id: string
    uid: string
    formattedName: string
    givenName: string
    familyName: string
    organization: string
    title: string
    note: string
    emails: ContactEmailInput[]
    phones: ContactPhoneInput[]
}

const emptyForm: ContactForm = {
    id: '', uid: '', formattedName: '', givenName: '', familyName: '',
    organization: '', title: '', note: '', emails: [], phones: [],
}

function formFor(c: Contact): ContactForm {
    return {
        id: c.id,
        uid: c.uid,
        formattedName: c.formattedName,
        givenName: c.givenName,
        familyName: c.familyName,
        organization: c.organization,
        title: c.title,
        note: c.note,
        emails: (c.emails ?? []).map((e) => ({label: e.label, address: e.address})),
        phones: (c.phones ?? []).map((p) => ({label: p.label, number: p.number})),
    }
}

// ContactsModal lists the address book and edits contacts. It imports and exports vCard and CSV so
// contacts round-trip with Outlook and Thunderbird. Deletion is always confirmed.
export function ContactsModal({contacts, onChanged, onClose}: ContactsModalProps) {
    const [form, setForm] = useState<ContactForm | null>(null)
    const [pendingDelete, setPendingDelete] = useState<Contact | null>(null)
    const [error, setError] = useState('')
    const [status, setStatus] = useState('')
    const [busy, setBusy] = useState(false)

    const set = <K extends keyof ContactForm>(key: K, value: ContactForm[K]) =>
        setForm((f) => (f ? {...f, [key]: value} : f))

    const save = async () => {
        if (!form) return
        setBusy(true)
        setError('')
        try {
            const req: ContactInput = {
                ...form,
                emails: form.emails.filter((e) => e.address.trim() !== ''),
                phones: form.phones.filter((p) => p.number.trim() !== ''),
            }
            await api.saveContact(req)
            setForm(null)
            setStatus('')
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
            await api.deleteContact(pendingDelete.id)
            setPendingDelete(null)
            if (form && form.id === pendingDelete.id) setForm(null)
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
            const n = await api.importContactsFromFile()
            if (n > 0) {
                setStatus(`Imported ${n} contact${n === 1 ? '' : 's'}.`)
                onChanged()
            }
        } catch (e) {
            setError(String(e))
        }
    }

    const doExport = async (format: string) => {
        setError('')
        setStatus('')
        try {
            const written = await api.exportContactsToFile(format)
            if (written) setStatus(`Exported ${contacts.length} contact${contacts.length === 1 ? '' : 's'}.`)
        } catch (e) {
            setError(String(e))
        }
    }

    return (
        <div className="modal-backdrop" onClick={onClose}>
            <div className="modal" role="dialog" aria-label="Contacts" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Contacts</h2>
                <p className="setup-hint">Your address book. Import and export use vCard or CSV, so contacts
                    round-trip with Outlook and Thunderbird.</p>
                {error && <div className="compose-error">{error}</div>}
                {status && <div className="setup-hint">{status}</div>}

                <div className="modal-actions">
                    <button className="btn" onClick={() => void doImport()}>Import…</button>
                    <button className="btn" onClick={() => void doExport('vcard')} disabled={contacts.length === 0}>
                        Export vCard
                    </button>
                    <button className="btn" onClick={() => void doExport('csv')} disabled={contacts.length === 0}>
                        Export CSV
                    </button>
                    <button className="btn primary" onClick={() => setForm({...emptyForm})}>New contact</button>
                </div>

                {contacts.length === 0 ? (
                    <p className="empty-body">No contacts yet.</p>
                ) : (
                    <ul className="list">
                        {contacts.map((c) => (
                            <li key={c.id} className="list-item">
                                <span className="item-text" onClick={() => setForm(formFor(c))}>
                                    <span className="item-title">{c.formattedName}</span>
                                    <span className="item-sub">
                                        {c.emails && c.emails.length > 0 ? c.emails[0].address : c.organization}
                                    </span>
                                </span>
                                <button
                                    className="account-action delete"
                                    aria-label={`Delete ${c.formattedName}`}
                                    title="Delete contact"
                                    onClick={() => setPendingDelete(c)}
                                >
                                    &times;
                                </button>
                            </li>
                        ))}
                    </ul>
                )}

                {form && (
                    <div className="rule-form">
                        <input className="tag-name-input" placeholder="Full name" value={form.formattedName} autoFocus
                               onChange={(e) => set('formattedName', e.target.value)}/>
                        <div className="rule-form-row">
                            <input className="tag-name-input" placeholder="First name" value={form.givenName}
                                   onChange={(e) => set('givenName', e.target.value)}/>
                            <input className="tag-name-input" placeholder="Last name" value={form.familyName}
                                   onChange={(e) => set('familyName', e.target.value)}/>
                        </div>
                        <div className="rule-form-row">
                            <input className="tag-name-input" placeholder="Organization" value={form.organization}
                                   onChange={(e) => set('organization', e.target.value)}/>
                            <input className="tag-name-input" placeholder="Job title" value={form.title}
                                   onChange={(e) => set('title', e.target.value)}/>
                        </div>

                        {form.emails.map((em, i) => (
                            <div className="rule-form-row" key={`email-${i}`}>
                                <input className="tag-name-input" placeholder="label (e.g. work)" value={em.label}
                                       onChange={(e) => set('emails', form.emails.map((x, j) => j === i ? {...x, label: e.target.value} : x))}/>
                                <input className="tag-name-input" placeholder="email address" value={em.address}
                                       onChange={(e) => set('emails', form.emails.map((x, j) => j === i ? {...x, address: e.target.value} : x))}/>
                                <button className="account-action delete" aria-label="Remove email" title="Remove email"
                                        onClick={() => set('emails', form.emails.filter((_, j) => j !== i))}>&times;</button>
                            </div>
                        ))}
                        {form.phones.map((ph, i) => (
                            <div className="rule-form-row" key={`phone-${i}`}>
                                <input className="tag-name-input" placeholder="label (e.g. mobile)" value={ph.label}
                                       onChange={(e) => set('phones', form.phones.map((x, j) => j === i ? {...x, label: e.target.value} : x))}/>
                                <input className="tag-name-input" placeholder="phone number" value={ph.number}
                                       onChange={(e) => set('phones', form.phones.map((x, j) => j === i ? {...x, number: e.target.value} : x))}/>
                                <button className="account-action delete" aria-label="Remove phone" title="Remove phone"
                                        onClick={() => set('phones', form.phones.filter((_, j) => j !== i))}>&times;</button>
                            </div>
                        ))}
                        <div className="rule-form-row">
                            <button className="btn" onClick={() => set('emails', [...form.emails, {label: '', address: ''}])}>
                                Add email
                            </button>
                            <button className="btn" onClick={() => set('phones', [...form.phones, {label: '', number: ''}])}>
                                Add phone
                            </button>
                        </div>
                        <textarea className="tag-name-input" placeholder="Notes" value={form.note} rows={3}
                                  onChange={(e) => set('note', e.target.value)}/>
                        <div className="modal-actions spread">
                            <button className="btn" onClick={() => setForm(null)}>Cancel</button>
                            <button className="btn primary" onClick={() => void save()}
                                    disabled={busy || form.formattedName.trim() === ''}>
                                {busy ? 'Saving…' : (form.id ? 'Save changes' : 'Add contact')}
                            </button>
                        </div>
                    </div>
                )}

                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose}>Close</button>
                </div>
            </div>

            {pendingDelete && (
                <ConfirmDialog
                    title="Delete contact"
                    message={`Delete "${pendingDelete.formattedName}"? This cannot be undone.`}
                    confirmLabel="Delete"
                    busy={busy}
                    onConfirm={() => void confirmDelete()}
                    onCancel={() => setPendingDelete(null)}
                />
            )}
        </div>
    )
}
