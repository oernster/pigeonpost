import {useEffect, useState} from 'react'
import {api, CalendarEvent, CalendarEventInstance} from '../api'
import {monthCells, weekDays, type ViewMode} from '../calendarModel'

interface EventInstancesInput {
    viewDate: Date
    viewMode: ViewMode
    // events is a refetch trigger only: when the parent's event list changes the visible occurrences are
    // re-expanded so an edit made elsewhere shows here too.
    events: CalendarEvent[]
    setError: (message: string) => void
}

// useEventInstances loads the concrete occurrences shown for the visible range, expanded from the recurring
// events by the backend. It refetches whenever the view moves (a new date or mode), whenever bumpReload is
// called after a local change and whenever the parent's events prop changes. reloadKey is the manual refetch
// trigger bumpReload increments. A stale in-flight response is discarded by the active flag so a fast view
// change cannot show the previous range's occurrences.
export function useEventInstances({viewDate, viewMode, events, setError}: EventInstancesInput) {
    const [instances, setInstances] = useState<CalendarEventInstance[]>([])
    const [reloadKey, setReloadKey] = useState(0)
    const bumpReload = () => setReloadKey((k) => k + 1)

    // visibleRange is the inclusive [from, to] window of the active view, used to expand recurring events
    // into just the occurrences on screen.
    const visibleRange = (): {from: string; to: string} => {
        const days = viewMode === 'month' ? monthCells(viewDate)
            : viewMode === 'week' ? weekDays(viewDate) : [viewDate]
        const first = new Date(days[0])
        first.setHours(0, 0, 0, 0)
        const last = new Date(days[days.length - 1])
        last.setHours(23, 59, 59, 0)
        return {from: first.toISOString(), to: last.toISOString()}
    }

    useEffect(() => {
        const {from, to} = visibleRange()
        let active = true
        api.listEventInstances(from, to)
            .then((xs) => {
                if (active) setInstances(xs)
            })
            .catch((e) => {
                if (active) setError(String(e))
            })
        return () => {
            active = false
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [viewDate, viewMode, reloadKey, events])

    return {instances, bumpReload}
}
