import type {Dispatch, SetStateAction} from 'react'
import {CalDAVAccount} from '../api'
import {CalDAVAccountForm} from '../caldavAccount'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {CalDAVAccountFormModal} from './CalDAVAccountFormModal'

interface CalDAVAccountsManagerProps {
    accounts: CalDAVAccount[]
    adding: boolean
    startAdd: () => void
    cancelAdd: () => void
    form: CalDAVAccountForm
    setForm: Dispatch<SetStateAction<CalDAVAccountForm>>
    submitAdd: () => void
    sync: (account: CalDAVAccount) => void
    // syncingId is the account whose sync is in flight, so only that row's button shows progress.
    syncingId: string
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
// accounts each with a two-way Sync and a Remove, plus the remove confirmation. The add-account form is its
// own dialog stacked on this one (see CalDAVAccountFormModal). It is the presentational surface over
// useCalDAVAccounts; all its state and actions are injected. A sync is two-way (local changes go up, server
// changes come down), which the hint makes explicit.
export function CalDAVAccountsManager({
    accounts, adding, startAdd, cancelAdd, form, setForm, submitAdd, sync, syncingId,
    pendingDelete, setPendingDelete, confirmRemove, onClose, busy, error, status,
}: CalDAVAccountsManagerProps) {
    return (
        <>
            <div className="modal-backdrop">
                <div className="modal event-form pinned-actions" role="dialog" aria-label="Remote calendars"
                     onClick={(e) => e.stopPropagation()}>
                    <ModalClose onClose={onClose}/>
                    <h2 className="modal-title">Remote calendars</h2>
                    <div className="modal-body">
                        <p className="setup-hint">
                            Add a CalDAV server (Fastmail, iCloud, Nextcloud and similar) to sync its calendars with
                            PigeonPost. A sync is two-way: your local changes are sent to the server, then the
                            server's changes are brought in. If the same event changed in both places, the server's
                            version wins and your local version is kept as a separate copy so nothing is lost.
                        </p>
                        <div className="caldav-accounts">
                            {accounts.length === 0 && (
                                <p className="field-hint">No remote calendars yet.</p>
                            )}
                            {accounts.map((account) => (
                                <div key={account.id} className="caldav-account-row">
                                    <span className="caldav-account-info">
                                        <span className="caldav-account-name">{account.displayName}</span>
                                        <span className="caldav-account-meta">{account.username} · {account.baseUrl}</span>
                                    </span>
                                    <button className="btn" onClick={() => sync(account)} disabled={busy}>
                                        {syncingId === account.id ? 'Syncing…' : 'Sync'}
                                    </button>
                                    <button className="btn danger" onClick={() => setPendingDelete(account)} disabled={busy}>
                                        Remove
                                    </button>
                                </div>
                            ))}
                        </div>
                    </div>
                    {/* Beside the actions rather than in the scrolling body, so a sync's outcome shows
                        however far down the list is. While the add form is open its error shows there. */}
                    {error && !adding && <div className="compose-error">{error}</div>}
                    {status && <div className="setup-hint">{status}</div>}
                    <div className="modal-actions spread">
                        <button className="btn" onClick={onClose}>Done</button>
                        <button className="btn primary" onClick={startAdd} disabled={busy}>Add account</button>
                    </div>
                </div>
            </div>

            {adding && (
                <CalDAVAccountFormModal
                    form={form}
                    setForm={setForm}
                    busy={busy}
                    error={error}
                    onSubmit={submitAdd}
                    onCancel={cancelAdd}
                />
            )}

            {pendingDelete && (
                <ConfirmDialog
                    title="Remove remote calendar"
                    message={`Remove the account "${pendingDelete.displayName}"? Calendars already synced from it stay in PigeonPost. This cannot be undone.`}
                    confirmLabel="Remove"
                    busy={busy}
                    onConfirm={() => void confirmRemove()}
                    onCancel={() => setPendingDelete(null)}
                />
            )}
        </>
    )
}
