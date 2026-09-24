import type {Dispatch, SetStateAction} from 'react'
import {Contact, ContactInput, ContactEmailInput, ContactPhoneInput, ContactAddressInput} from '../api'
import {useNestedDialogClose} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'
import {DateField} from './DateField'

export interface ContactForm {
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

export const emptyForm: ContactForm = {
    id: '', uid: '', formattedName: '', givenName: '', familyName: '',
    organization: '', title: '', note: '', birthday: '', emails: [], phones: [], addresses: [],
}

// displayNameOf derives the vCard formatted name (FN), which is required for export and used as the list
// title, from the parts the user fills. There is no separate full-name field: given and family name make
// it, falling back to the organisation, then the first email address.
export function displayNameOf(f: ContactForm): string {
    const person = [f.givenName, f.familyName].map((s) => s.trim()).filter(Boolean).join(' ')
    if (person) return person
    if (f.organization.trim() !== '') return f.organization.trim()
    const firstEmail = f.emails.find((e) => e.address.trim() !== '')
    return firstEmail ? firstEmail.address.trim() : ''
}

export function formFor(c: Contact): ContactForm {
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

// contactInputOf is the save request for a form: the derived display name, with blank email, phone and
// address rows dropped.
export function contactInputOf(form: ContactForm): ContactInput {
    return {
        ...form,
        formattedName: displayNameOf(form),
        emails: form.emails.filter((e) => e.address.trim() !== ''),
        phones: form.phones.filter((p) => p.number.trim() !== ''),
        addresses: form.addresses.filter((a) => !addressIsEmpty(a)),
    }
}

interface ContactFormModalProps {
    form: ContactForm
    setForm: Dispatch<SetStateAction<ContactForm | null>>
    busy: boolean
    error: string
    onSave: () => void
    // onDelete asks for the open contact to be deleted; the confirmation belongs to the address book.
    onDelete: () => void
    onCancel: () => void
}

// ContactFormModal is the contact editor, a dialog stacked on the address book the way the event editor
// stacks on the calendar. The name rows are pinned above the scrolling details and the actions below them,
// so who is being edited and the Save button stay on screen however many emails, phones and addresses the
// contact carries. It edits the form it is given; saving and deleting belong to the address book. It
// closes on Escape but not on a click beside it (see useNestedDialogClose).
export function ContactFormModal({form, setForm, busy, error, onSave, onDelete, onCancel}: ContactFormModalProps) {
    useNestedDialogClose(onCancel)
    const set = <K extends keyof ContactForm>(key: K, value: ContactForm[K]) =>
        setForm((f) => (f ? {...f, [key]: value} : f))
    const title = form.id ? 'Edit contact' : 'New contact'

    return (
        <div className="modal-backdrop">
            <div className="modal contact-form pinned-actions" role="dialog" aria-label={title}
                 onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">{title}</h2>
                <div className="rule-form pinned-form-header">
                    <div className="rule-form-row">
                        <input className="tag-name-input" placeholder="First name" value={form.givenName} autoFocus
                               onChange={(e) => set('givenName', e.target.value)}/>
                        <input className="tag-name-input" placeholder="Last name" value={form.familyName}
                               onChange={(e) => set('familyName', e.target.value)}/>
                    </div>
                </div>
                <div className="modal-body">
                    <div className="rule-form">
                        <div className="rule-form-row">
                            <input className="tag-name-input" placeholder="Organisation" value={form.organization}
                                   onChange={(e) => set('organization', e.target.value)}/>
                            <input className="tag-name-input" placeholder="Job title" value={form.title}
                                   onChange={(e) => set('title', e.target.value)}/>
                        </div>
                        <div className="rule-form-row">
                            <DateField kind="date" ariaLabel="Birthday" pickerTitle="Birthday" compact
                                       value={form.birthday} onChange={(v) => set('birthday', v)}/>
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
                            <div className="contact-address" key={`address-${i}`}>
                                <div className="contact-address-grid">
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
                                </div>
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
                    </div>
                </div>
                {error && <div className="compose-error">{error}</div>}
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel}>Cancel</button>
                    <div className="action-group">
                        {form.id !== '' && (
                            <button className="btn danger" onClick={onDelete}>Delete contact</button>
                        )}
                        <button className="btn primary" onClick={onSave} disabled={busy || displayNameOf(form) === ''}>
                            {busy ? 'Saving…' : (form.id ? 'Save changes' : 'Add contact')}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}
