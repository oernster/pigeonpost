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

export const MS_PER_DAY = 86400000
// MONTH_MAX_LANES caps how many stacked event rows a month cell shows before the rest collapse into a
// "+N more" affordance, so a busy day cannot push the six week rows past the dialog height.
export const MONTH_MAX_LANES = 3

// dayIndex maps a date to an integer day number (whole local days since the epoch), so day spans and week
// columns become integer arithmetic. It reads the date's own midnight, and Math.round absorbs the sub-day
// shift a daylight-saving change introduces so consecutive local days always differ by exactly one.
export function dayIndex(d: Date): number {
    const midnight = new Date(d.getFullYear(), d.getMonth(), d.getDate())
    return Math.round(midnight.getTime() / MS_PER_DAY)
}

// eventDaySpan returns the inclusive first and last day indices an event occupies. An event that ends
// exactly at midnight does not occupy the day it ends on, so the last day is taken from the final instant
// (one millisecond before the end); a zero-length or reversed range still occupies its start day.
export function eventDaySpan(startMs: number, endMs: number): {firstDay: number; lastDay: number} {
    const firstDay = dayIndex(new Date(startMs))
    const lastInstant = endMs > startMs ? endMs - 1 : startMs
    const lastDay = Math.max(firstDay, dayIndex(new Date(lastInstant)))
    return {firstDay, lastDay}
}

// MonthBarInput is one event reduced to the inclusive day range it covers, keyed so the caller can map the
// placed bar back to its instance.
export interface MonthBarInput {
    key: string
    firstDay: number
    lastDay: number
}

// MonthBar is a placed event within one week row: the column range it spans (0 to 6 inclusive), the lane it
// sits in and whether the week boundary clips it, so the caller can square that end and drop the title on a
// continuation.
export interface MonthBar {
    key: string
    startCol: number
    endCol: number
    lane: number
    continuesLeft: boolean
    continuesRight: boolean
}

// WeekLayout is the placement for one week row: the bars that fit within maxLanes, the number of lanes used
// and the per-column count of events that did not fit (each rendered as "+N more").
export interface WeekLayout {
    bars: MonthBar[]
    lanes: number
    overflow: number[]
}

// layoutWeek places every event that intersects the seven-day week starting at weekStartDay into a column
// span and a lane. Events are ordered by start then longest-first, so a multi-day bar takes a low lane above
// the shorter events it spans, and each is given the lowest lane free across its whole span. Events past
// maxLanes are dropped from the bars and counted into overflow for each day they cover, so a busy day shows
// a "+N more" rather than growing without bound.
export function layoutWeek(weekStartDay: number, events: MonthBarInput[], maxLanes: number): WeekLayout {
    const weekEndDay = weekStartDay + DAYS_IN_WEEK - 1
    const inWeek = events
        .filter((e) => e.lastDay >= weekStartDay && e.firstDay <= weekEndDay)
        .sort((a, b) => (a.firstDay - b.firstDay) || ((b.lastDay - b.firstDay) - (a.lastDay - a.firstDay)))
    const laneEnds: number[] = []
    const bars: MonthBar[] = []
    const overflow = new Array<number>(DAYS_IN_WEEK).fill(0)
    for (const e of inWeek) {
        const startCol = Math.max(0, e.firstDay - weekStartDay)
        const endCol = Math.min(DAYS_IN_WEEK - 1, e.lastDay - weekStartDay)
        let lane = 0
        while (lane < laneEnds.length && laneEnds[lane] >= startCol) {
            lane++
        }
        laneEnds[lane] = endCol
        if (lane < maxLanes) {
            bars.push({
                key: e.key, startCol, endCol, lane,
                continuesLeft: e.firstDay < weekStartDay,
                continuesRight: e.lastDay > weekEndDay,
            })
        } else {
            for (let c = startCol; c <= endCol; c++) {
                overflow[c]++
            }
        }
    }
    return {bars, lanes: Math.min(laneEnds.length, maxLanes), overflow}
}

const DARK_INK = '#0b1220'
const LIGHT_INK = '#ffffff'
// LUMA_THRESHOLD is the mid perceived-luminance (0 to 255) at which a fill flips from wanting dark ink to
// light ink over it.
const LUMA_THRESHOLD = 140

// contrastInk returns a dark or light ink for a label laid over the given hex fill, so a multi-day event bar
// keeps its title legible whatever colour its calendar uses. It compares the fill's perceived luminance (the
// Rec. 601 weighting of red, green and blue) against a mid threshold and falls back to the dark ink for an
// unparseable colour.
export function contrastInk(hex: string): string {
    const match = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
    if (!match) {
        return DARK_INK
    }
    const value = parseInt(match[1], 16)
    const red = (value >> 16) & 0xff
    const green = (value >> 8) & 0xff
    const blue = value & 0xff
    const luma = (red * 299 + green * 587 + blue * 114) / 1000
    return luma > LUMA_THRESHOLD ? DARK_INK : LIGHT_INK
}
