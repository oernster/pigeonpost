import {useEffect, useState} from 'react'
import {api, Contact, ContactInput, ContactEmailInput, ContactPhoneInput, ContactAddressInput, ContactGroup, ContactGroupInput, ContactImportResult} from '../api'
import {useBackdropDismiss} from './useBackdropDismiss'
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
    birthday: string
    emails: ContactEmailInput[]
    phones: ContactPhoneInput[]
    addresses: ContactAddressInput[]
}

const emptyForm: ContactForm = {
    id: '', uid: '', formattedName: '', givenName: '', familyName: '',
    organization: '', title: '', note: '', birthday: '', emails: [], phones: [], addresses: [],
}

// GroupForm backs the group editor: a name and the ids of the contacts in the group (a mailing list).
interface GroupForm {
    id: string
    name: string
    members: string[]
}

// displayNameOf derives the vCard formatted name (FN), which is required for export and used as the list
// title, from the parts the user fills. There is no separate full-name field: given and family name make
// it, falling back to the organization, then the first email address.
function displayNameOf(f: ContactForm): string {
    const person = [f.givenName, f.familyName].map((s) => s.trim()).filter(Boolean).join(' ')
    if (person) return person
    if (f.organization.trim() !== '') return f.organization.trim()
    const firstEmail = f.emails.find((e) => e.address.trim() !== '')
    return firstEmail ? firstEmail.address.trim() : ''
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
        birthday: c.birthday,
        emails: (c.emails ?? []).map((e) => ({label: e.label, address: e.address})),
        phones: (c.phones ?? []).map((p) => ({label: p.label, number: p.number})),
        addresses: (c.addresses ?? []).map((a) => ({
            label: a.label, street: a.street, locality: a.locality,
            region: a.region, postalCode: a.postalCode, country: a.country,
        })),
    }
}

// addressIsEmpty is true when every component of an address row is blank after trimming, so such a row
// is dropped on save rather than sent to the backend (which would reject it).
function addressIsEmpty(a: ContactAddressInput): boolean {
    return [a.street, a.locality, a.region, a.postalCode, a.country].every((s) => s.trim() === '')
}

// plural appends an s to a noun unless there is exactly one of them.
function plural(n: number, noun: string): string {
    return `${n} ${noun}${n === 1 ? '' : 's'}`
}

// importSummary describes what an import changed and names the file it came from. A file that yields
// nothing is reported explicitly rather than passing in silence, since a silent import cannot be told
// apart from one that did not run. Naming the source matters just as much: an address book usually
// holds several exports with similar names, so a count on its own cannot distinguish a file that
// really did hold five contacts from the wrong file having been picked.
function importSummary(result: ContactImportResult): string {
    const from = result.file ? ` from ${result.file}` : ''
    if (result.added > 0 && result.updated > 0) {
        return `Imported ${plural(result.added, 'contact')} and updated ${plural(result.updated, 'existing contact')}${from}.`
    }
    if (result.added > 0) {
        return `Imported ${plural(result.added, 'contact')}${from}.`
    }
    if (result.updated > 0) {
        return `Updated ${plural(result.updated, 'existing contact')}${from}. None were new.`
    }
    return `No contacts found${from}.`
}

// ContactsModal lists the address book and edits contacts. It imports and exports vCard and CSV so
// contacts round-trip with Outlook and Thunderbird. Deletion is always confirmed.
export function ContactsModal({contacts, onChanged, onClose}: ContactsModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const [form, setForm] = useState<ContactForm | null>(null)
    const [pendingDelete, setPendingDelete] = useState<Contact | null>(null)
    const [error, setError] = useState('')
    const [status, setStatus] = useState('')
    const [busy, setBusy] = useState(false)

    const set = <K extends keyof ContactForm>(key: K, value: ContactForm[K]) =>
        setForm((f) => (f ? {...f, [key]: value} : f))

    const [groups, setGroups] = useState<ContactGroup[]>([])
    const [groupFilter, setGroupFilter] = useState('')
    const [groupForm, setGroupForm] = useState<GroupForm | null>(null)
    const [pendingGroupDelete, setPendingGroupDelete] = useState<ContactGroup | null>(null)

    const reloadGroups = () =>
        void api.listContactGroups().then(setGroups).catch((e) => setError(String(e)))
    useEffect(() => {
        reloadGroups()
    }, [])

    const activeGroup = groups.find((g) => g.id === groupFilter) ?? null
    const shownContacts = activeGroup ? contacts.filter((c) => activeGroup.members.includes(c.id)) : contacts
    const memberCount = (g: ContactGroup) => contacts.filter((c) => g.members.includes(c.id)).length

    // The contact editor and the group editor are mutually exclusive: opening one closes the other so the
    // modal never shows two forms at once.
    const openContact = (c: Contact | null) => {
        setGroupForm(null)
        setError('')
        setForm(c ? formFor(c) : {...emptyForm})
    }

    const openGroupEditor = (g: ContactGroup | null) => {
        setForm(null)
        setError('')
        setGroupForm(g ? {id: g.id, name: g.name, members: [...g.members]} : {id: '', name: '', members: []})
    }

    const toggleMember = (id: string) =>
        setGroupForm((gf) => gf ? {
            ...gf,
            members: gf.members.includes(id) ? gf.members.filter((m) => m !== id) : [...gf.members, id],
        } : gf)

    const saveGroup = async () => {
        if (!groupForm || groupForm.name.trim() === '') return
        setBusy(true)
        setError('')
        try {
            const req: ContactGroupInput = {
                id: groupForm.id,
                name: groupForm.name.trim(),
                // Drop any member id whose contact no longer exists, so a deleted contact does not linger.
                members: groupForm.members.filter((id) => contacts.some((c) => c.id === id)),
            }
            await api.saveContactGroup(req)
            setGroupForm(null)
            reloadGroups()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const confirmGroupDelete = async () => {
        if (!pendingGroupDelete) return
        setBusy(true)
        setError('')
        try {
            await api.deleteContactGroup(pendingGroupDelete.id)
            if (groupFilter === pendingGroupDelete.id) setGroupFilter('')
            if (groupForm && groupForm.id === pendingGroupDelete.id) setGroupForm(null)
            setPendingGroupDelete(null)
            reloadGroups()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const save = async () => {
        if (!form) return
        setBusy(true)
        setError('')
        try {
            const req: ContactInput = {
                ...form,
                formattedName: displayNameOf(form),
                emails: form.emails.filter((e) => e.address.trim() !== ''),
                phones: form.phones.filter((p) => p.number.trim() !== ''),
                addresses: form.addresses.filter((a) => !addressIsEmpty(a)),
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
            const result = await api.importContactsFromFile()
            if (result.cancelled) return
            setStatus(importSummary(result))
            if (result.added > 0 || result.updated > 0) onChanged()
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
        <div className="modal-backdrop" {...dismiss}>
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
                    <button className="btn primary" onClick={() => openContact(null)}>New contact</button>
                </div>

                <div className="cg-bar">
                    <button className={'cg-chip' + (groupFilter === '' ? ' active' : '')}
                            onClick={() => setGroupFilter('')}>
                        All ({contacts.length})
                    </button>
                    {groups.map((g) => (
                        <button key={g.id} className={'cg-chip' + (groupFilter === g.id ? ' active' : '')}
                                onClick={() => setGroupFilter(g.id)}>
                            {g.name} ({memberCount(g)})
                        </button>
                    ))}
                    <span className="cg-bar-spacer"/>
                    {activeGroup && (
                        <>
                            <button className="cg-chip" onClick={() => openGroupEditor(activeGroup)}>Edit group</button>
                            <button className="cg-chip" onClick={() => setPendingGroupDelete(activeGroup)}>Delete group</button>
                        </>
                    )}
                    <button className="cg-chip" onClick={() => openGroupEditor(null)}>+ New group</button>
                </div>

                {shownContacts.length === 0 ? (
                    <p className="empty-body">{activeGroup ? 'No contacts in this group yet.' : 'No contacts yet.'}</p>
                ) : (
                    <ul className="list">
                        {shownContacts.map((c) => (
                            <li key={c.id} className="list-item">
                                <span className="item-text" onClick={() => openContact(c)}>
                                    <span className="item-title" title={c.formattedName}>{c.formattedName}</span>
                                    <span
                                        className="item-sub"
                                        title={c.emails && c.emails.length > 0 ? c.emails[0].address : c.organization}
                                    >
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
                        <div className="rule-form-row">
                            <input className="tag-name-input" placeholder="First name" value={form.givenName} autoFocus
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
                        <div className="rule-form-row">
                            <input className="tag-name-input" type="date" aria-label="Birthday" value={form.birthday}
                                   onChange={(e) => set('birthday', e.target.value)}/>
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
                        {form.addresses.map((ad, i) => (
                            <div className="rule-form-row" key={`address-${i}`}>
                                <input className="tag-name-input" placeholder="label (e.g. home)" value={ad.label}
                                       onChange={(e) => set('addresses', form.addresses.map((x, j) => j === i ? {...x, label: e.target.value} : x))}/>
                                <input className="tag-name-input" placeholder="street" value={ad.street}
                                       onChange={(e) => set('addresses', form.addresses.map((x, j) => j === i ? {...x, street: e.target.value} : x))}/>
                                <input className="tag-name-input" placeholder="city" value={ad.locality}
                                       onChange={(e) => set('addresses', form.addresses.map((x, j) => j === i ? {...x, locality: e.target.value} : x))}/>
                                <input className="tag-name-input" placeholder="region" value={ad.region}
                                       onChange={(e) => set('addresses', form.addresses.map((x, j) => j === i ? {...x, region: e.target.value} : x))}/>
                                <input className="tag-name-input" placeholder="postal code" value={ad.postalCode}
                                       onChange={(e) => set('addresses', form.addresses.map((x, j) => j === i ? {...x, postalCode: e.target.value} : x))}/>
                                <input className="tag-name-input" placeholder="country" value={ad.country}
                                       onChange={(e) => set('addresses', form.addresses.map((x, j) => j === i ? {...x, country: e.target.value} : x))}/>
                                <button className="account-action delete" aria-label="Remove address" title="Remove address"
                                        onClick={() => set('addresses', form.addresses.filter((_, j) => j !== i))}>&times;</button>
                            </div>
                        ))}
                        <div className="rule-form-row">
                            <button className="btn" onClick={() => set('emails', [...form.emails, {label: '', address: ''}])}>
                                Add email
                            </button>
                            <button className="btn" onClick={() => set('phones', [...form.phones, {label: '', number: ''}])}>
                                Add phone
                            </button>
                            <button className="btn" onClick={() => set('addresses', [...form.addresses, {label: '', street: '', locality: '', region: '', postalCode: '', country: ''}])}>
                                Add address
                            </button>
                        </div>
                        <textarea className="tag-name-input" placeholder="Notes" value={form.note} rows={3}
                                  onChange={(e) => set('note', e.target.value)}/>
                        <div className="modal-actions spread">
                            <button className="btn" onClick={() => setForm(null)}>Cancel</button>
                            <button className="btn primary" onClick={() => void save()}
                                    disabled={busy || displayNameOf(form) === ''}>
                                {busy ? 'Saving…' : (form.id ? 'Save changes' : 'Add contact')}
                            </button>
                        </div>
                    </div>
                )}

                {groupForm && (
                    <div className="rule-form">
                        <input className="tag-name-input" placeholder="Group name" value={groupForm.name} autoFocus
                               onChange={(e) => setGroupForm((gf) => gf ? {...gf, name: e.target.value} : gf)}/>
                        <p className="setup-hint">Choose the contacts in this group.</p>
                        {contacts.length === 0 ? (
                            <p className="empty-body">Add contacts first, then group them.</p>
                        ) : (
                            <div className="cg-members">
                                {contacts.map((c) => (
                                    <label key={c.id} className="cg-member">
                                        <input type="checkbox" checked={groupForm.members.includes(c.id)}
                                               onChange={() => toggleMember(c.id)}/>
                                        <span>{c.formattedName}</span>
                                    </label>
                                ))}
                            </div>
                        )}
                        <div className="modal-actions spread">
                            <button className="btn" onClick={() => setGroupForm(null)}>Cancel</button>
                            <button className="btn primary" onClick={() => void saveGroup()}
                                    disabled={busy || groupForm.name.trim() === ''}>
                                {busy ? 'Saving…' : (groupForm.id ? 'Save group' : 'Create group')}
                            </button>
                        </div>
                    </div>
                )}

                {!form && !groupForm && (
                    <div className="modal-actions spread">
                        <button className="btn" onClick={onClose}>Close</button>
                    </div>
                )}
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

            {pendingGroupDelete && (
                <ConfirmDialog
                    title="Delete group"
                    message={`Delete the group "${pendingGroupDelete.name}"? The contacts themselves are not deleted.`}
                    confirmLabel="Delete"
                    busy={busy}
                    onConfirm={() => void confirmGroupDelete()}
                    onCancel={() => setPendingGroupDelete(null)}
                />
            )}
        </div>
    )
}
