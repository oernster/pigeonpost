import {useEffect, useState} from 'react'
import {api, CalDAVAccount} from '../api'
import {CalDAVAccountForm, emptyCalDAVAccountForm, validateCalDAVAccountForm} from '../caldavAccount'

interface CalDAVAccountsInput {
    setError: (message: string) => void
    setStatus: (message: string) => void
    setBusy: (busy: boolean) => void
    // onSynced refreshes the calendar after a sync reconciles remote events into the local store.
    onSynced: () => void
}

// useCalDAVAccounts owns the remote-calendars (CalDAV) sub-feature: the accounts loaded from the backend,
// the manager's open state, the add-account form and the account pending removal, plus add, remove and the
// two-way sync. It is the calendar counterpart to useCalendars for DAV accounts; busy, error and status are
// the shared calendar banners, injected here. A sync pushes local changes then reconciles the server, so a
// successful sync calls onSynced for the calendar to reload its events.
export function useCalDAVAccounts({setError, setStatus, setBusy, onSynced}: CalDAVAccountsInput) {
    const [accounts, setAccounts] = useState<CalDAVAccount[]>([])
    const [managing, setManaging] = useState(false)
    const [adding, setAdding] = useState(false)
    const [form, setForm] = useState<CalDAVAccountForm>(emptyCalDAVAccountForm())
    const [pendingDelete, setPendingDelete] = useState<CalDAVAccount | null>(null)
    // syncingId is the id of the account whose sync is in flight, so only that row shows progress. The shared
    // busy flag disables every action; the label must not imply the other accounts are syncing.
    const [syncingId, setSyncingId] = useState('')

    const reload = () =>
        void api.listCalDAVAccounts().then(setAccounts).catch((e) => setError(String(e)))
    useEffect(() => {
        reload()
    }, [])

    const startAdd = () => {
        setForm(emptyCalDAVAccountForm())
        setAdding(true)
        setError('')
        setStatus('')
    }

    const cancelAdd = () => {
        setAdding(false)
        setForm(emptyCalDAVAccountForm())
    }

    const submitAdd = async () => {
        const problem = validateCalDAVAccountForm(form)
        if (problem !== '') {
            setError(problem)
            return
        }
        setBusy(true)
        setError('')
        setStatus('')
        try {
            await api.addCalDAVAccount(
                form.displayName.trim(), form.baseUrl.trim(), form.username.trim(), form.password,
            )
            setAdding(false)
            setForm(emptyCalDAVAccountForm())
            reload()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const confirmRemove = async () => {
        if (!pendingDelete) return
        setBusy(true)
        setError('')
        setStatus('')
        try {
            await api.removeCalDAVAccount(pendingDelete.id)
            setPendingDelete(null)
            reload()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const sync = async (account: CalDAVAccount) => {
        setBusy(true)
        setSyncingId(account.id)
        setError('')
        setStatus('')
        try {
            await api.syncCalDAV(account.id)
            setStatus(`Synced ${account.displayName}.`)
            onSynced()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
            setSyncingId('')
        }
    }

    return {
        accounts, managing, setManaging, adding, startAdd, cancelAdd,
        form, setForm, submitAdd, pendingDelete, setPendingDelete, confirmRemove, sync, syncingId,
    }
}
