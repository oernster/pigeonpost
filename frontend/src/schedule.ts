// Send-later and snooze share this module: the preset moments their menus offer and the helpers that
// bridge a native datetime-local input. Every choice is computed from a passed-in now, so the module
// stays pure and each rollover rule (an evening that has passed, the next Monday) is testable.

export interface ScheduleChoice {
    label: string
    at: Date
}

// The preset anchors: evening is 18:00 and morning is 09:00 local time. A preset whose natural moment
// has already passed rolls forward (this evening becomes tomorrow evening) so a choice is always in
// the future.
const EVENING_HOUR = 18
const MORNING_HOUR = 9
const SEND_SOON_HOURS = 1
const SNOOZE_SOON_HOURS = 3
const DAYS_PER_WEEK = 7
// JS Date.getDay() counts from Sunday as 0, so Monday is 1.
const MONDAY = 1
const MS_PER_HOUR = 60 * 60 * 1000

// dayAt returns the moment dayOffset days from base at the given whole hour, local time.
function dayAt(base: Date, dayOffset: number, hour: number): Date {
    const moment = new Date(base)
    moment.setDate(moment.getDate() + dayOffset)
    moment.setHours(hour, 0, 0, 0)
    return moment
}

function hoursFrom(base: Date, hours: number): Date {
    return new Date(base.getTime() + hours * MS_PER_HOUR)
}

// daysUntilNextMonday is how many days from now the NEXT Monday falls: always one to seven, so on a
// Monday it names the following week rather than today.
function daysUntilNextMonday(now: Date): number {
    const days = (MONDAY - now.getDay() + DAYS_PER_WEEK) % DAYS_PER_WEEK
    return days === 0 ? DAYS_PER_WEEK : days
}

// sendLaterChoices are the offered send-later moments: an hour from now, the coming evening (today's
// 18:00, or tomorrow's when that has passed) and tomorrow morning.
export function sendLaterChoices(now: Date): ScheduleChoice[] {
    const eveningToday = dayAt(now, 0, EVENING_HOUR)
    const eveningIsToday = eveningToday.getTime() > now.getTime()
    return [
        {label: 'In 1 hour', at: hoursFrom(now, SEND_SOON_HOURS)},
        {
            label: eveningIsToday ? 'This evening (18:00)' : 'Tomorrow evening (18:00)',
            at: eveningIsToday ? eveningToday : dayAt(now, 1, EVENING_HOUR),
        },
        {label: 'Tomorrow morning (09:00)', at: dayAt(now, 1, MORNING_HOUR)},
    ]
}

// snoozeChoices are the offered snooze moments: a few hours from now, tomorrow morning and next
// Monday morning.
export function snoozeChoices(now: Date): ScheduleChoice[] {
    return [
        {label: 'In 3 hours', at: hoursFrom(now, SNOOZE_SOON_HOURS)},
        {label: 'Tomorrow morning (09:00)', at: dayAt(now, 1, MORNING_HOUR)},
        {label: 'Next Monday (09:00)', at: dayAt(now, daysUntilNextMonday(now), MORNING_HOUR)},
    ]
}

// toDatetimeLocal formats a Date as a datetime-local input value (local time, minute precision).
export function toDatetimeLocal(date: Date): string {
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
        `T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// fromDatetimeLocal parses a datetime-local input value as local time, or null when empty or invalid.
export function fromDatetimeLocal(value: string): Date | null {
    if (value === '') {
        return null
    }
    const parsed = new Date(value)
    return isNaN(parsed.getTime()) ? null : parsed
}

// isSchedulable reports whether a chosen moment can be scheduled: present and strictly in the future.
export function isSchedulable(at: Date | null, now: Date): boolean {
    return at !== null && at.getTime() > now.getTime()
}
