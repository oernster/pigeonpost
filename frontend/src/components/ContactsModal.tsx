import {useEffect, useState} from 'react'
import {api, Contact, ContactGroup, ContactGroupInput, ContactImportResult} from '../api'
import {useBackdropDismiss} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {AUTO_COLLECT_KEY, autoCollectStored, shouldAutoCollect} from '../autoCollect'
import {ContactFormModal, type ContactForm, contactInputOf, emptyForm, formFor} from './ContactFormModal'
import {ContactGroupFormModal, type GroupForm} from './ContactGroupFormModal'

interface ContactsModalProps {
    contacts: Contact[]
    onChanged: () => void
    onClose: () => void
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

    // autoCollect mirrors the persisted add-recipients-to-contacts setting (on by default); the
    // composer reads the same key at send time.
    const [autoCollect, setAutoCollect] = useState(() =>
        shouldAutoCollect(window.localStorage.getItem(AUTO_COLLECT_KEY)))

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
            await api.saveContact(contactInputOf(form))
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
            <div className="modal contacts pinned-actions" role="dialog" aria-label="Contacts" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Contacts</h2>
                <p className="setup-hint">Your address book. Import and export use vCard or CSV, so contacts
                    round-trip with Outlook and Thunderbird.</p>
                <label className="auto-collect-toggle">
                    <input type="checkbox" checked={autoCollect}
                           onChange={(e) => {
                               setAutoCollect(e.target.checked)
                               window.localStorage.setItem(AUTO_COLLECT_KEY, autoCollectStored(e.target.checked))
                           }}/>
                    <span>Add people you email to contacts automatically</span>
                </label>
                {/* An error while an editor is open is shown in that editor, which sits over this one. */}
                {error && !form && !groupForm && <div className="compose-error">{error}</div>}
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

                <div className="modal-body">
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
                    <ul className="list contacts-grid">
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
                                    className="contact-edit"
                                    aria-label={`Edit ${c.formattedName}`}
                                    title="Edit contact"
                                    onClick={() => openContact(c)}
                                >
                                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor"
                                         strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                                        <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/>
                                    </svg>
                                </button>
                            </li>
                        ))}
                    </ul>
                )}
                </div>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose}>Close</button>
                </div>
            </div>

            {form && (
                <ContactFormModal
                    form={form}
                    setForm={setForm}
                    busy={busy}
                    error={error}
                    onSave={() => void save()}
                    onDelete={() => {
                        const open = contacts.find((c) => c.id === form.id)
                        if (open) setPendingDelete(open)
                    }}
                    onCancel={() => setForm(null)}
                />
            )}

            {groupForm && (
                <ContactGroupFormModal
                    form={groupForm}
                    contacts={contacts}
                    busy={busy}
                    error={error}
                    onNameChange={(name) => setGroupForm((gf) => gf ? {...gf, name} : gf)}
                    onToggleMember={toggleMember}
                    onSave={() => void saveGroup()}
                    onCancel={() => setGroupForm(null)}
                />
            )}

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
