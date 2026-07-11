import {useEffect, useState} from 'react'
import {api, Calendar} from '../api'

// CalForm is the inline colour-and-name editor's state for the calendar being added or edited. An empty id
// marks a new calendar.
export interface CalForm {
    id: string
    name: string
    colour: string
}

interface CalendarsInput {
    setError: (message: string) => void
    setBusy: (busy: boolean) => void
    onChanged: () => void
}

// useCalendars owns the calendars sub-feature: the list loaded from the backend (which colours events and
// seeds a new event's calendar), the manager's open state, the calendar being edited and the one pending
// deletion, plus save and delete. Deleting a calendar deletes its events too, so a delete also calls
// onChanged for the parent to reload the events. Busy and error are the shared banners, injected here.
export function useCalendars({setError, setBusy, onChanged}: CalendarsInput) {
    const [calendars, setCalendars] = useState<Calendar[]>([])
    const [managingCals, setManagingCals] = useState(false)
    const [calForm, setCalForm] = useState<CalForm | null>(null)
    const [pendingCalDelete, setPendingCalDelete] = useState<Calendar | null>(null)

    const reloadCalendars = () =>
        void api.listCalendars().then(setCalendars).catch((e) => setError(String(e)))
    useEffect(() => {
        reloadCalendars()
    }, [])

    const saveCal = async () => {
        if (!calForm || calForm.name.trim() === '') return
        setBusy(true)
        setError('')
        try {
            await api.saveCalendar({id: calForm.id, name: calForm.name.trim(), colour: calForm.colour})
            setCalForm(null)
            reloadCalendars()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const confirmCalDelete = async () => {
        if (!pendingCalDelete) return
        setBusy(true)
        setError('')
        try {
            // Deleting a calendar deletes its events too, so refresh the event list as well.
            await api.deleteCalendar(pendingCalDelete.id)
            if (calForm && calForm.id === pendingCalDelete.id) setCalForm(null)
            setPendingCalDelete(null)
            reloadCalendars()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    return {
        calendars, managingCals, setManagingCals, calForm, setCalForm, saveCal,
        pendingCalDelete, setPendingCalDelete, confirmCalDelete,
    }
}
