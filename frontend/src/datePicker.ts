// datePicker holds the pure calendar model behind the app's date-picker dialog: the month grid a
// dialog renders, month arithmetic and the ISO date/datetime splicing that lets one picker serve
// both plain date fields and datetime-local fields (picking a date keeps the time already typed).
// No React, no api runtime and no wall-clock reads: the dialog passes today in.

// WEEKDAY_LABELS heads the calendar columns, Monday first.
export const WEEKDAY_LABELS = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su'] as const

const MONTH_LABELS = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
] as const

// DEFAULT_PICK_TIME is the time a datetime-local field gets when a date is picked while its time
// part is still empty; a time already typed is always kept.
export const DEFAULT_PICK_TIME = '09:00'

const DAYS_PER_WEEK = 7

// DayCell is one calendar square: its ISO date, its day-of-month number and whether it belongs to
// the shown month (leading and trailing squares come from the neighbours).
export interface DayCell {
    iso: string
    day: number
    inMonth: boolean
}

// YearMonth names a calendar page; month is 1 to 12.
export interface YearMonth {
    year: number
    month: number
}

// datePart returns the ISO date inside a date or datetime-local value ('' stays '').
export function datePart(value: string): string {
    return value.split('T')[0]
}

// timePart returns the HH:mm inside a datetime-local value, or '' when there is none.
export function timePart(value: string): string {
    const [, time] = value.split('T')
    return time ?? ''
}

// isoDate formats a year, month and day as the ISO date a date input holds.
export function isoDate(year: number, month: number, day: number): string {
    const mm = String(month).padStart(2, '0')
    const dd = String(day).padStart(2, '0')
    return `${year}-${mm}-${dd}`
}

// monthOf returns the calendar page an ISO date sits on, or the fallback for anything unparsable,
// so the dialog opens on the field's month when it has one and on today's otherwise.
export function monthOf(iso: string, fallback: YearMonth): YearMonth {
    const match = /^(\d{4})-(\d{2})/.exec(iso)
    if (!match) {
        return fallback
    }
    const month = Number(match[2])
    if (month < 1 || month > 12) {
        return fallback
    }
    return {year: Number(match[1]), month}
}

// addMonths steps a calendar page forward or back, carrying across year boundaries.
export function addMonths(page: YearMonth, delta: number): YearMonth {
    const index = page.year * 12 + (page.month - 1) + delta
    return {year: Math.floor(index / 12), month: (index % 12 + 12) % 12 + 1}
}

// monthLabel names a calendar page ("July 2026").
export function monthLabel(page: YearMonth): string {
    return `${MONTH_LABELS[page.month - 1]} ${page.year}`
}

// monthGrid lays a month out as Monday-first weeks, the leading and trailing squares filled from
// the neighbouring months so every week is complete. Date arithmetic runs in UTC so the grid never
// depends on the machine's zone.
export function monthGrid(page: YearMonth): DayCell[][] {
    const first = Date.UTC(page.year, page.month - 1, 1)
    const mondayOffset = (new Date(first).getUTCDay() + DAYS_PER_WEEK - 1) % DAYS_PER_WEEK
    const daysInMonth = new Date(Date.UTC(page.year, page.month, 0)).getUTCDate()
    const weekCount = Math.ceil((mondayOffset + daysInMonth) / DAYS_PER_WEEK)
    const weeks: DayCell[][] = []
    for (let week = 0; week < weekCount; week++) {
        const cells: DayCell[] = []
        for (let column = 0; column < DAYS_PER_WEEK; column++) {
            const offset = week * DAYS_PER_WEEK + column - mondayOffset
            const date = new Date(Date.UTC(page.year, page.month - 1, 1 + offset))
            cells.push({
                iso: isoDate(date.getUTCFullYear(), date.getUTCMonth() + 1, date.getUTCDate()),
                day: date.getUTCDate(),
                inMonth: date.getUTCMonth() === page.month - 1,
            })
        }
        weeks.push(cells)
    }
    return weeks
}

// mergeDate splices a picked ISO date into a field value: a date field simply becomes the date,
// while a datetime-local field keeps the time already typed (or gains the default). ISO dates
// compare correctly as strings, which beforeMin relies on.
export function mergeDate(value: string, iso: string, kind: 'date' | 'datetime-local'): string {
    if (kind === 'date') {
        return iso
    }
    return `${iso}T${timePart(value) || DEFAULT_PICK_TIME}`
}

// beforeMin reports whether a day falls before a field's minimum date ('' means no minimum), so
// the dialog can disable days the field would reject.
export function beforeMin(iso: string, min: string): boolean {
    const floor = datePart(min)
    return floor !== '' && iso < floor
}
