import {Contact} from '../api'
import {useNestedDialogClose} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'

// GroupForm backs the group editor: a name and the ids of the contacts in the group (a mailing list).
export interface GroupForm {
    id: string
    name: string
    members: string[]
}

interface ContactGroupFormModalProps {
    form: GroupForm
    contacts: Contact[]
    busy: boolean
    error: string
    onNameChange: (name: string) => void
    onToggleMember: (id: string) => void
    onSave: () => void
    onCancel: () => void
}

// ContactGroupFormModal is the group editor, a dialog stacked on the address book. The group name is
// pinned above the scrolling member list and the actions below it, so a long address book never scrolls
// the name or the Save button out of view. Saving belongs to the address book. It closes on Escape but
// not on a click beside it (see useNestedDialogClose).
export function ContactGroupFormModal({
    form, contacts, busy, error, onNameChange, onToggleMember, onSave, onCancel,
}: ContactGroupFormModalProps) {
    useNestedDialogClose(onCancel)
    const title = form.id ? 'Edit group' : 'New group'

    return (
        <div className="modal-backdrop">
            <div className="modal contact-group-form pinned-actions" role="dialog" aria-label={title}
                 onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">{title}</h2>
                <div className="rule-form pinned-form-header">
                    <input className="tag-name-input" placeholder="Group name" value={form.name} autoFocus
                           onChange={(e) => onNameChange(e.target.value)}/>
                </div>
                <div className="modal-body">
                    <p className="setup-hint">Choose the contacts in this group.</p>
                    {contacts.length === 0 ? (
                        <p className="empty-body">Add contacts first, then group them.</p>
                    ) : (
                        <div className="cg-members">
                            {contacts.map((c) => (
                                <label key={c.id} className="cg-member">
                                    <input type="checkbox" checked={form.members.includes(c.id)}
                                           onChange={() => onToggleMember(c.id)}/>
                                    <span>{c.formattedName}</span>
                                </label>
                            ))}
                        </div>
                    )}
                </div>
                {error && <div className="compose-error">{error}</div>}
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel}>Cancel</button>
                    <button className="btn primary" onClick={onSave} disabled={busy || form.name.trim() === ''}>
                        {busy ? 'Saving…' : (form.id ? 'Save group' : 'Create group')}
                    </button>
                </div>
            </div>
        </div>
    )
}
