// calendarModel holds the pure calendar helpers and constants for the calendar dialog: month and week
// grid math, meeting-link extraction and provider naming, date-input formatting and the reminder,
// attendee, weekday and month data. No React, no api runtime, so the logic is unit-tested in isolation.

export const DAYS_IN_WEEK = 7
export const HOURS_PER_EVENT = 1

// Meeting-link detection for the event dialog. A received invite carries its join link in the event's
// location or description as a plain URL, so these turn it into a click that opens the default browser.
const MEETING_URL_RE = /https?:\/\/[^\s<>"')\]]+/g

const MEETING_HOSTS: {matches: (host: string) => boolean; label: string}[] = [
    {matches: (h) => h === 'teams.microsoft.com' || h === 'teams.live.com', label: 'Microsoft Teams'},
    {matches: (h) => h === 'meet.google.com', label: 'Google Meet'},
    {matches: (h) => h === 'zoom.us' || h.endsWith('.zoom.us'), label: 'Zoom'},
    {matches: (h) => h.endsWith('webex.com'), label: 'Webex'},
]

// extractUrls pulls the http(s) URLs out of a text field, trimming the trailing punctuation a sentence
// often leaves on the end of a pasted link.
export function extractUrls(text: string): string[] {
    const found = text.match(MEETING_URL_RE) ?? []
    return found.map((u) => u.replace(/[.,;:)\]]+$/, ''))
}

// meetingProvider names the video provider for a known host, so a join button can read "Join Microsoft
// Teams" rather than a bare URL; it returns null for an unrecognised or unparseable URL.
export function meetingProvider(url: string): string | null {
    let host = ''
    try {
        host = new URL(url).hostname.toLowerCase()
    } catch {
        return null
    }
    for (const provider of MEETING_HOSTS) {
        if (provider.matches(host)) {
            return provider.label
        }
    }
    return null
}
// DEFAULT_EVENT_COLOUR marks events not assigned to a calendar (or whose calendar has no colour).
export const DEFAULT_EVENT_COLOUR = '#7fb0ff'
export const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
export const WEEKDAYS_FULL = [
    'Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday',
]
export const MONTHS = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
]
export const MONTHS_SHORT = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
]

export type ViewMode = 'month' | 'week' | 'day'
export const VIEW_MODES: ViewMode[] = ['month', 'week', 'day']

// REMINDER_PRESETS are the reminder lead times offered in the form, in whole minutes before the start.
export const REMINDER_PRESETS: {minutes: number; label: string}[] = [
    {minutes: 0, label: 'At time of event'},
    {minutes: 5, label: '5 minutes before'},
    {minutes: 10, label: '10 minutes before'},
    {minutes: 15, label: '15 minutes before'},
    {minutes: 30, label: '30 minutes before'},
    {minutes: 60, label: '1 hour before'},
    {minutes: 120, label: '2 hours before'},
    {minutes: 1440, label: '1 day before'},
    {minutes: 10080, label: '1 week before'},
]
export const DEFAULT_REMINDER_MINUTES = 15

// DEFAULT_ATTENDEE_ROLE and DEFAULT_ATTENDEE_STATUS are the ICS ROLE and PARTSTAT values a freshly added
// attendee carries: a required participant who has not yet responded.
export const DEFAULT_ATTENDEE_ROLE = 'REQ-PARTICIPANT'
export const DEFAULT_ATTENDEE_STATUS = 'NEEDS-ACTION'

// ATTENDEE_STATUS_LABELS maps an ICS PARTSTAT value to a human label shown against each attendee.
const ATTENDEE_STATUS_LABELS: Record<string, string> = {
    'NEEDS-ACTION': 'No response yet',
    'ACCEPTED': 'Accepted',
    'DECLINED': 'Declined',
    'TENTATIVE': 'Tentative',
    'DELEGATED': 'Delegated',
}

export function attendeeStatusLabel(status: string): string {
    return ATTENDEE_STATUS_LABELS[status] || status
}

export function pad(n: number): string {
    return n < 10 ? '0' + n : String(n)
}

export function dateInput(d: Date): string {
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function dateTimeInput(d: Date): string {
    return `${dateInput(d)}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function sameDay(a: Date, b: Date): boolean {
    return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
}

// monthCells returns 42 day cells (six weeks) covering the month of viewDate, starting on the Sunday on
// or before the first of the month.
export function monthCells(viewDate: Date): Date[] {
    const first = new Date(viewDate.getFullYear(), viewDate.getMonth(), 1)
    const start = new Date(first)
    start.setDate(first.getDate() - first.getDay())
    const cells: Date[] = []
    for (let i = 0; i < 42; i++) {
        const d = new Date(start)
        d.setDate(start.getDate() + i)
        cells.push(d)
    }
    return cells
}

// weekDays returns the seven days of the week containing viewDate, starting on the Sunday on or before it.
export function weekDays(viewDate: Date): Date[] {
    const start = new Date(viewDate)
    start.setDate(viewDate.getDate() - viewDate.getDay())
    const days: Date[] = []
    for (let i = 0; i < DAYS_IN_WEEK; i++) {
        const d = new Date(start)
        d.setDate(start.getDate() + i)
        days.push(d)
    }
    return days
}
