import type {Dispatch, SetStateAction} from 'react'
import {CalDAVAccount} from '../api'
import {CalDAVAccountForm, validateCalDAVAccountForm} from '../caldavAccount'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'

interface CalDAVAccountsManagerProps {
    accounts: CalDAVAccount[]
    adding: boolean
    startAdd: () => void
    cancelAdd: () => void
    form: CalDAVAccountForm
    setForm: Dispatch<SetStateAction<CalDAVAccountForm>>
    submitAdd: () => void
    pull: (account: CalDAVAccount) => void
    // pullingId is the account whose pull is in flight, so only that row's button shows progress.
    pullingId: string
    pendingDelete: CalDAVAccount | null
    setPendingDelete: Dispatch<SetStateAction<CalDAVAccount | null>>
    confirmRemove: () => void
    // onClose closes the manager and clears the add form.
    onClose: () => void
    busy: boolean
    error: string
    status: string
}

// CalDAVAccountsManager is the remote-calendars (CalDAV) sub-feature's modal: the list of configured DAV
// accounts each with a read-only Pull and a Remove, plus the add-account form and the remove confirmation. It
// is the presentational surface over useCalDAVAccounts; all its state and actions are injected. A pull is
// one-way for now (remote to local), which the hint makes explicit.
export function CalDAVAccountsManager({
    accounts, adding, startAdd, cancelAdd, form, setForm, submitAdd, pull, pullingId,
    pendingDelete, setPendingDelete, confirmRemove, onClose, busy, error, status,
}: CalDAVAccountsManagerProps) {
    const problem = validateCalDAVAccountForm(form)
    return (
        <>
            <div className="modal-backdrop">
                <div className="modal event-form" role="dialog" aria-label="Remote calendars"
                     onClick={(e) => e.stopPropagation()}>
                    <ModalClose onClose={onClose}/>
                    <h2 className="modal-title">Remote calendars</h2>
                    <p className="setup-hint">
                        Add a CalDAV server (Fastmail, iCloud, Nextcloud and similar) to pull its calendars into
                        PigeonPost. For now this is a one-way, read-only pull: events come in; changes you make
                        here are not sent back to the server yet.
                    </p>
                    {error && <div className="compose-error">{error}</div>}
                    {status && <div className="setup-hint">{status}</div>}

                    <div className="caldav-accounts">
                        {accounts.length === 0 && !adding && (
                            <p className="field-hint">No remote calendars yet.</p>
                        )}
                        {accounts.map((account) => (
                            <div key={account.id} className="caldav-account-row">
                                <span className="caldav-account-info">
                                    <span className="caldav-account-name">{account.displayName}</span>
                                    <span className="caldav-account-meta">{account.username} · {account.baseUrl}</span>
                                </span>
                                <button className="btn" onClick={() => pull(account)} disabled={busy}>
                                    {pullingId === account.id ? 'Pulling…' : 'Pull'}
                                </button>
                                <button className="btn danger" onClick={() => setPendingDelete(account)} disabled={busy}>
                                    Remove
                                </button>
                            </div>
                        ))}
                    </div>

                    {adding ? (
                        <div className="rule-form">
                            <label className="field">
                                <span>Name</span>
                                <input value={form.displayName} autoFocus placeholder="Fastmail calendar"
                                       onChange={(e) => setForm((f) => ({...f, displayName: e.target.value}))}/>
                            </label>
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
                            <div className="modal-actions spread">
                                <button className="btn" onClick={cancelAdd} disabled={busy}>Cancel</button>
                                <button className="btn primary" onClick={() => void submitAdd()}
                                        disabled={busy || problem !== ''}>
                                    {busy ? 'Adding…' : 'Add account'}
                                </button>
                            </div>
                        </div>
                    ) : (
                        <div className="modal-actions spread">
                            <button className="btn" onClick={onClose}>Done</button>
                            <button className="btn primary" onClick={startAdd} disabled={busy}>Add account</button>
                        </div>
                    )}
                </div>
            </div>

            {pendingDelete && (
                <ConfirmDialog
                    title="Remove remote calendar"
                    message={`Remove the account "${pendingDelete.displayName}"? Calendars already pulled from it stay in PigeonPost. This cannot be undone.`}
                    confirmLabel="Remove"
                    busy={busy}
                    onConfirm={() => void confirmRemove()}
                    onCancel={() => setPendingDelete(null)}
                />
            )}
        </>
    )
}
