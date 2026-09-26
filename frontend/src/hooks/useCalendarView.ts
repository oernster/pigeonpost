import {Dispatch, SetStateAction, useState} from 'react'
import {DAYS_IN_WEEK, MONTHS, MONTHS_SHORT, WEEKDAYS_FULL, weekDays, type ViewMode} from '../calendarModel'

export interface CalendarView {
    viewDate: Date
    setViewDate: Dispatch<SetStateAction<Date>>
    viewMode: ViewMode
    setViewMode: Dispatch<SetStateAction<ViewMode>>
    shift: (delta: number) => void
    headerLabel: () => string
    openDay: (day: Date) => void
}

// useCalendarView owns where the calendar is looking: the date in view and whether it shows a month, a week
// or a day, how the header names that, how Previous and Next move it and opening one day from the month.
export function useCalendarView(): CalendarView {
    const [viewDate, setViewDate] = useState(() => new Date())
    const [viewMode, setViewMode] = useState<ViewMode>('month')

    // shift moves the view by one unit of the active mode: a month, a week or a day.
    const shift = (delta: number) =>
        setViewDate((d) => {
            if (viewMode === 'month') return new Date(d.getFullYear(), d.getMonth() + delta, 1)
            const n = new Date(d)
            n.setDate(d.getDate() + delta * (viewMode === 'week' ? DAYS_IN_WEEK : 1))
            return n
        })

    const headerLabel = (): string => {
        if (viewMode === 'month') return `${MONTHS[viewDate.getMonth()]} ${viewDate.getFullYear()}`
        if (viewMode === 'day') {
            return `${WEEKDAYS_FULL[viewDate.getDay()]}, ${viewDate.getDate()} ` +
                `${MONTHS[viewDate.getMonth()]} ${viewDate.getFullYear()}`
        }
        const wd = weekDays(viewDate)
        const a = wd[0]
        const b = wd[DAYS_IN_WEEK - 1]
        return `${a.getDate()} ${MONTHS_SHORT[a.getMonth()]} to ` +
            `${b.getDate()} ${MONTHS_SHORT[b.getMonth()]} ${b.getFullYear()}`
    }

    const openDay = (day: Date) => {
        setViewDate(day)
        setViewMode('day')
    }

    return {viewDate, setViewDate, viewMode, setViewMode, shift, headerLabel, openDay}
}
