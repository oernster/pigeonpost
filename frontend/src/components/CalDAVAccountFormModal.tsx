import type {Dispatch, SetStateAction} from 'react'
import {CalDAVAccountForm, validateCalDAVAccountForm} from '../caldavAccount'
import {useBackdropDismiss} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'

interface CalDAVAccountFormModalProps {
    form: CalDAVAccountForm
    setForm: Dispatch<SetStateAction<CalDAVAccountForm>>
    busy: boolean
    error: string
    onSubmit: () => void
    onCancel: () => void
}

// CalDAVAccountFormModal is the add-a-remote-calendar form, a dialog stacked on the remote calendars list
// the way the contact editor stacks on the address book. The account's name is pinned above the scrolling
// connection fields and the actions below them. Like the list, it is presentational: the form state and
// the add itself belong to useCalDAVAccounts.
export function CalDAVAccountFormModal({form, setForm, busy, error, onSubmit, onCancel}: CalDAVAccountFormModalProps) {
    const dismiss = useBackdropDismiss(onCancel)
    const problem = validateCalDAVAccountForm(form)
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal caldav-form pinned-actions" role="dialog" aria-label="Add remote calendar"
                 onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">Add remote calendar</h2>
                <div className="rule-form pinned-form-header">
                    <label className="field">
                        <span>Name</span>
                        <input value={form.displayName} autoFocus placeholder="Fastmail calendar"
                               onChange={(e) => setForm((f) => ({...f, displayName: e.target.value}))}/>
                    </label>
                </div>
                <div className="modal-body">
                    <div className="rule-form">
                        <label className="field">
                            <span>Server address</span>
                            <input value={form.baseUrl} placeholder="https://caldav.fastmail.com"
                                   onChange={(e) => setForm((f) => ({...f, baseUrl: e.target.value}))}/>
                        </label>
                        <label className="field">
                            <span>Username</span>
                            <input value={form.username} placeholder="you@example.com"
                                   onChange={(e) => setForm((f) => ({...f, username: e.target.value}))}/>
                        </label>
                        <label className="field">
                            <span>Password</span>
                            <input type="password" value={form.password}
                                   onChange={(e) => setForm((f) => ({...f, password: e.target.value}))}/>
                        </label>
                        <p className="field-hint">
                            Many providers need an app-specific password rather than your normal one. Your
                            password is stored in the operating system keychain, never in the app database.
                        </p>
                    </div>
                </div>
                {error && <div className="compose-error">{error}</div>}
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel} disabled={busy}>Cancel</button>
                    <button className="btn primary" onClick={() => void onSubmit()} disabled={busy || problem !== ''}>
                        {busy ? 'Adding…' : 'Add account'}
                    </button>
                </div>
            </div>
        </div>
    )
}
