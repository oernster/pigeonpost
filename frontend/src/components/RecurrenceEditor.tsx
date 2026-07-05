// RecurrenceEditor is a friendly builder for an RFC 5545 RRULE string. It holds the choices in local
// state and calls onChange with the rebuilt rule, re-syncing from the value only when a different rule
// arrives, because the rule string cannot represent every intermediate choice (an end-on-date with no
// date picked yet). It covers the common calendar cases (daily; weekly on chosen weekdays; monthly or
// yearly on a day-of-month or an Nth weekday derived from the event start; monthly presets for the last
// day, first weekday or last weekday of the month using BYSETPOS/BYMONTHDAY; an interval; and an end of
// never, after a count, or on a date). Rarer rule parts are not offered but are preserved only if the
// parent does not overwrite them, so this editor is used for app-authored rules; the monthly and yearly
// weekday patterns are recomputed from the event start rather than preserved verbatim.
import {useEffect, useRef, useState} from 'react'
import {PickerButton} from './PickerButton'

// WEEKDAYS lists the RFC 5545 weekday codes in display order with a short label for the weekly picker.
const WEEKDAYS: {code: string; label: string}[] = [
    {code: 'MO', label: 'Mon'},
    {code: 'TU', label: 'Tue'},
    {code: 'WE', label: 'Wed'},
    {code: 'TH', label: 'Thu'},
    {code: 'FR', label: 'Fri'},
    {code: 'SA', label: 'Sat'},
    {code: 'SU', label: 'Sun'},
]

const WEEKDAY_NAMES: Record<string, string> = {
    MO: 'Monday', TU: 'Tuesday', WE: 'Wednesday', TH: 'Thursday', FR: 'Friday', SA: 'Saturday', SU: 'Sunday',
}

const MONTH_NAMES = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
]

// ORDINAL_WORDS names the position of a weekday within the month; -1 is the last occurrence of that day.
const ORDINAL_WORDS: Record<number, string> = {1: 'first', 2: 'second', 3: 'third', 4: 'fourth', [-1]: 'last'}

const DEFAULT_COUNT = 10
const MIN_INTERVAL = 1
const DAYS_PER_WEEK = 7

// DAY_CODES maps JavaScript's getDay() (0 = Sunday) to the RFC 5545 weekday code.
const DAY_CODES = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']

type Frequency = '' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY'
type EndMode = 'never' | 'count' | 'until'
// MonthPattern selects how a monthly or yearly rule lands. The first two derive from the event start:
// the same day-of-month, or the same ordinal weekday (the third Tuesday, the last Friday). The last
// three are absolute monthly presets built with BYSETPOS/BYMONTHDAY: the last day, the first weekday
// (Mon-Fri) and the last weekday of the month.
type MonthPattern = 'day' | 'weekday' | 'lastDay' | 'firstWeekday' | 'lastWeekday'

// MONTH_PRESETS are the MonthPattern values that do not derive from the event start; they are offered
// only for monthly rules and reset when the frequency becomes yearly.
const MONTH_PRESETS: MonthPattern[] = ['lastDay', 'firstWeekday', 'lastWeekday']

// WEEKDAY_SET is the Monday-to-Friday BYDAY list a first/last-weekday preset pairs with a BYSETPOS.
const WEEKDAY_SET = ['MO', 'TU', 'WE', 'TH', 'FR']

// isWeekdaySet reports whether a BYDAY list is exactly the Monday-to-Friday set, in any order.
function isWeekdaySet(byday: string[]): boolean {
    return byday.length === WEEKDAY_SET.length && WEEKDAY_SET.every((d) => byday.includes(d))
}

interface RuleState {
    freq: Frequency
    interval: number
    byday: string[]
    pattern: MonthPattern
    endMode: EndMode
    count: number
    until: string
}

// StartFacts are the parts of the event start a monthly or yearly rule needs: the day-of-month, the
// month, the weekday code and the ordinal of that weekday within the month (-1 when it is the last).
interface StartFacts {
    day: number
    month: number
    weekday: string
    ordinal: number
}

function pad(n: number): string {
    return n < 10 ? '0' + n : String(n)
}

// startFactsOf derives the recurrence anchors from a date-or-date-time input value, built from a local
// date so nothing shifts with the timezone. It is undefined when the input has no date part.
function startFactsOf(input: string): StartFacts | undefined {
    const match = input.match(/^(\d{4})-(\d{2})-(\d{2})/)
    if (!match) return undefined
    const year = Number(match[1])
    const month = Number(match[2])
    const day = Number(match[3])
    const weekday = DAY_CODES[new Date(year, month - 1, day).getDay()]
    const daysInMonth = new Date(year, month, 0).getDate()
    const nth = Math.ceil(day / DAYS_PER_WEEK)
    const isLast = day + DAYS_PER_WEEK > daysInMonth
    return {day, month, weekday, ordinal: isLast ? -1 : nth}
}

// tomorrowInput is the earliest date the series may end on: tomorrow, as a yyyy-mm-dd input value. An end
// date must be in the future so the rule keeps at least the next occurrence.
function tomorrowInput(): string {
    const d = new Date()
    d.setDate(d.getDate() + 1)
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// parseRule turns an RRULE value into editor state, defaulting the parts the rule does not specify.
function parseRule(value: string): RuleState {
    const state: RuleState = {
        freq: '', interval: MIN_INTERVAL, byday: [], pattern: 'day', endMode: 'never', count: DEFAULT_COUNT, until: '',
    }
    const trimmed = value.trim().replace(/^RRULE:/i, '')
    if (trimmed === '') return state
    let bysetpos = ''
    let bymonthday = ''
    for (const part of trimmed.split(';')) {
        const [rawKey, rawValue] = part.split('=')
        const key = (rawKey || '').toUpperCase()
        const val = rawValue || ''
        if (key === 'FREQ') state.freq = val.toUpperCase() as Frequency
        else if (key === 'INTERVAL') state.interval = Math.max(MIN_INTERVAL, parseInt(val, 10) || MIN_INTERVAL)
        else if (key === 'BYDAY') state.byday = val.split(',').map((d) => d.toUpperCase()).filter(Boolean)
        else if (key === 'BYSETPOS') bysetpos = val.trim()
        else if (key === 'BYMONTHDAY') bymonthday = val.trim()
        else if (key === 'COUNT') {
            state.endMode = 'count'
            state.count = Math.max(1, parseInt(val, 10) || DEFAULT_COUNT)
        } else if (key === 'UNTIL') {
            state.endMode = 'until'
            state.until = untilToDateInput(val)
        }
    }
    // The absolute monthly presets (BYMONTHDAY=-1, or a weekday set with BYSETPOS) take precedence; then a
    // BYDAY carrying an ordinal (3TU, -1FR) means the start-derived weekday pattern.
    if (bymonthday === '-1') state.pattern = 'lastDay'
    else if (bysetpos === '1' && isWeekdaySet(state.byday)) state.pattern = 'firstWeekday'
    else if (bysetpos === '-1' && isWeekdaySet(state.byday)) state.pattern = 'lastWeekday'
    else if (state.byday.some((d) => /\d/.test(d))) state.pattern = 'weekday'
    return state
}

// untilToDateInput converts an RRULE UNTIL value (a DATE or UTC DATE-TIME) to a yyyy-mm-dd input value.
function untilToDateInput(until: string): string {
    const match = until.match(/^(\d{4})(\d{2})(\d{2})/)
    return match ? `${match[1]}-${match[2]}-${match[3]}` : ''
}

// dateInputToUntil converts a yyyy-mm-dd input value to an RRULE UNTIL at the end of that day in UTC, so
// the whole chosen day is included.
function dateInputToUntil(value: string): string {
    const parts = value.split('-')
    if (parts.length !== 3) return ''
    return `${parts[0]}${pad(Number(parts[1]))}${pad(Number(parts[2]))}T235959Z`
}

// buildRule renders editor state back into an RRULE value, or an empty string when the event does not
// repeat. The monthly and yearly patterns are derived from the event start (facts), so they are omitted
// when the start is unknown.
function buildRule(state: RuleState, facts: StartFacts | undefined): string {
    if (state.freq === '') return ''
    const parts = [`FREQ=${state.freq}`]
    if (state.interval > MIN_INTERVAL) parts.push(`INTERVAL=${state.interval}`)
    if (state.freq === 'WEEKLY' && state.byday.length > 0) parts.push(`BYDAY=${state.byday.join(',')}`)
    if (state.freq === 'MONTHLY' && facts) {
        if (state.pattern === 'weekday') parts.push(`BYDAY=${facts.ordinal}${facts.weekday}`)
        else if (state.pattern === 'lastDay') parts.push('BYMONTHDAY=-1')
        else if (state.pattern === 'firstWeekday') parts.push(`BYDAY=${WEEKDAY_SET.join(',')}`, 'BYSETPOS=1')
        else if (state.pattern === 'lastWeekday') parts.push(`BYDAY=${WEEKDAY_SET.join(',')}`, 'BYSETPOS=-1')
        else parts.push(`BYMONTHDAY=${facts.day}`)
    }
    if (state.freq === 'YEARLY' && facts && state.pattern === 'weekday') {
        parts.push(`BYMONTH=${facts.month}`)
        parts.push(`BYDAY=${facts.ordinal}${facts.weekday}`)
    }
    if (state.endMode === 'count' && state.count > 0) parts.push(`COUNT=${state.count}`)
    else if (state.endMode === 'until' && state.until !== '') parts.push(`UNTIL=${dateInputToUntil(state.until)}`)
    return parts.join(';')
}

// unitLabel names the recurrence unit for a frequency, pluralised when the interval is not one, so the
// frequency dropdown reads as the unit next to the interval ("every 3 days") rather than a separate word
// that would contradict it ("Daily every 3 days").
function unitLabel(freq: Frequency, interval: number): string {
    const unit = freq === 'DAILY' ? 'day' : freq === 'WEEKLY' ? 'week' : freq === 'MONTHLY' ? 'month' : 'year'
    return interval === 1 ? unit : `${unit}s`
}

// patternDayLabel and patternWeekdayLabel describe the two monthly or yearly choices in plain English.
function patternDayLabel(freq: Frequency, facts: StartFacts): string {
    return freq === 'YEARLY' ? `On ${MONTH_NAMES[facts.month - 1]} ${facts.day}` : `On day ${facts.day}`
}

function patternWeekdayLabel(freq: Frequency, facts: StartFacts): string {
    const base = `On the ${ORDINAL_WORDS[facts.ordinal]} ${WEEKDAY_NAMES[facts.weekday]}`
    return freq === 'YEARLY' ? `${base} of ${MONTH_NAMES[facts.month - 1]}` : base
}

interface RecurrenceEditorProps {
    value: string
    onChange: (rule: string) => void
    // startDate is the event's start (a date or date-time input value). Weekly rules default to its
    // weekday, and monthly and yearly rules anchor their day and ordinal to it.
    startDate: string
}

export function RecurrenceEditor({value, onChange, startDate}: RecurrenceEditorProps) {
    const [state, setState] = useState<RuleState>(() => parseRule(value))
    const untilRef = useRef<HTMLInputElement>(null)
    const minUntil = tomorrowInput()
    const facts = startFactsOf(startDate)

    // Re-sync only when a genuinely different rule arrives (a different event opened). The rule string
    // cannot represent every intermediate choice (an until with no date yet, a pattern with no start), so
    // deriving state from the rule every render would drop those. buildRule(state) equals the last value
    // we emitted, so our own edits never trigger a resync.
    useEffect(() => {
        if (value !== buildRule(state, facts)) setState(parseRule(value))
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [value])

    const update = (patch: Partial<RuleState>) => {
        const next = {...state, ...patch}
        setState(next)
        onChange(buildRule(next, facts))
    }

    const toggleDay = (code: string) => {
        const next = state.byday.includes(code)
            ? state.byday.filter((d) => d !== code)
            : [...state.byday, code]
        // Keep BYDAY in canonical weekday order so the rule is stable regardless of click order.
        const ordered = WEEKDAYS.map((w) => w.code).filter((c) => next.includes(c))
        update({byday: ordered})
    }

    // With no weekday chosen, a weekly rule repeats on the event's start weekday, so that day is shown as
    // the active default until the user picks explicit days.
    const dayActive = (code: string) =>
        state.byday.includes(code) || (state.byday.length === 0 && code === facts?.weekday)

    // changeEndMode switches how the series ends. Choosing an end date with none set yet seeds a valid
    // future default so the rule carries an UNTIL immediately and the picker opens on a sensible day.
    const changeEndMode = (mode: EndMode) => {
        if (mode === 'until' && state.until === '') update({endMode: mode, until: minUntil})
        else update({endMode: mode})
    }

    // changeUntil keeps the chosen end date in the future, clamping an earlier or cleared value up to the
    // minimum. yyyy-mm-dd strings compare correctly as text.
    const changeUntil = (value: string) => update({until: value < minUntil ? minUntil : value})

    // changeFreq switches the frequency, resetting an absolute monthly preset when moving to yearly (where
    // those presets are not offered) so the rule never carries a pattern the yearly builder would ignore.
    const changeFreq = (freq: Frequency) =>
        freq === 'YEARLY' && MONTH_PRESETS.includes(state.pattern) ? update({freq, pattern: 'day'}) : update({freq})

    const showPattern = (state.freq === 'MONTHLY' || state.freq === 'YEARLY') && facts !== undefined

    return (
        <div className="recurrence-editor">
            <div className="rule-form-row recur-first">
                {state.freq !== '' && (
                    <>
                        <span className="recur-label">Repeat every</span>
                        <input className="tag-name-input recur-interval-num" type="number" min={MIN_INTERVAL}
                               aria-label="Interval" value={state.interval}
                               onChange={(e) => update({interval: Math.max(MIN_INTERVAL, Number(e.target.value) || MIN_INTERVAL)})}/>
                    </>
                )}
                <select className="tag-name-input" aria-label="Repeat" value={state.freq}
                        onChange={(e) => changeFreq(e.target.value as Frequency)}>
                    <option value="">Does not repeat</option>
                    <option value="DAILY">{unitLabel('DAILY', state.interval)}</option>
                    <option value="WEEKLY">{unitLabel('WEEKLY', state.interval)}</option>
                    <option value="MONTHLY">{unitLabel('MONTHLY', state.interval)}</option>
                    <option value="YEARLY">{unitLabel('YEARLY', state.interval)}</option>
                </select>
            </div>

            {showPattern && facts && (
                <select className="tag-name-input" aria-label="Repeat pattern" value={state.pattern}
                        onChange={(e) => update({pattern: e.target.value as MonthPattern})}>
                    <option value="day">{patternDayLabel(state.freq, facts)}</option>
                    <option value="weekday">{patternWeekdayLabel(state.freq, facts)}</option>
                    {state.freq === 'MONTHLY' && (
                        <>
                            <option value="lastDay">On the last day of the month</option>
                            <option value="firstWeekday">On the first weekday of the month</option>
                            <option value="lastWeekday">On the last weekday of the month</option>
                        </>
                    )}
                </select>
            )}

            {state.freq === 'WEEKLY' && (
                <div className="recur-weekdays">
                    {WEEKDAYS.map((w) => (
                        <button key={w.code} type="button"
                                className={'recur-day' + (dayActive(w.code) ? ' active' : '')}
                                aria-pressed={dayActive(w.code)}
                                onClick={() => toggleDay(w.code)}>{w.label}</button>
                    ))}
                </div>
            )}

            {state.freq !== '' && (
                <div className="rule-form-row recur-end">
                    <select className="tag-name-input" aria-label="Ends" value={state.endMode}
                            onChange={(e) => changeEndMode(e.target.value as EndMode)}>
                        <option value="never">Never ends</option>
                        <option value="count">Ends after</option>
                        <option value="until">Ends on date</option>
                    </select>
                    {state.endMode === 'count' && (
                        <label className="recur-interval">
                            <input className="tag-name-input recur-interval-num" type="number" min={1}
                                   value={state.count}
                                   onChange={(e) => update({count: Math.max(1, Number(e.target.value) || 1)})}/>
                            occurrences
                        </label>
                    )}
                    {state.endMode === 'until' && (
                        <div className="date-field">
                            <input ref={untilRef} className="tag-name-input" type="date" aria-label="Repeat until"
                                   min={minUntil} value={state.until} onChange={(e) => changeUntil(e.target.value)}/>
                            <PickerButton target={untilRef}/>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
