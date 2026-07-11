import {useState} from 'react'

// Banners is the calendar's shared user-feedback surface: a one-off error, a transient status line and a
// busy flag that disables actions while a request is in flight. The event form, the calendars manager and
// the calendar shell all read and drive the same three, so they are owned in one place and injected as a
// unit rather than threaded as six separate props.
export interface Banners {
    error: string
    status: string
    busy: boolean
    setError: (message: string) => void
    setStatus: (message: string) => void
    setBusy: (busy: boolean) => void
}

// useBanners owns the shared error, status and busy state for the calendar and returns them as one unit.
export function useBanners(): Banners {
    const [error, setError] = useState('')
    const [status, setStatus] = useState('')
    const [busy, setBusy] = useState(false)
    return {error, status, busy, setError, setStatus, setBusy}
}
