// RecurrenceEditor is a friendly builder for an RFC 5545 RRULE string. It holds the choices in local
// state and calls onChange with the rebuilt rule, re-syncing from the value only when a different rule
// arrives, because the rule string cannot represent every intermediate choice (an end-on-date with no
// date picked yet). It covers the common calendar cases (daily, weekly with weekdays, monthly, yearly; an
// interval; and an end of never, after a count, or on a date); rarer rule parts are not offered but are
// preserved only if the parent does not overwrite them, so this editor is used for app-authored rules.
import {useEffect, useRef, useState} from 'react'
import {PickerButton} from './PickerButton'

const FREQUENCIES: {value: Frequency; label: string}[] = [
    {value: '', label: 'Does not repeat'},
    {value: 'DAILY', label: 'Daily'},
    {value: 'WEEKLY', label: 'Weekly'},
    {value: 'MONTHLY', label: 'Monthly'},
    {value: 'YEARLY', label: 'Yearly'},
]

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

const DEFAULT_COUNT = 10
const MIN_INTERVAL = 1

// DAY_CODES maps JavaScript's getDay() (0 = Sunday) to the RFC 5545 weekday code, so the event's start
// weekday can be shown as the default selection.
const DAY_CODES = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']

type Frequency = '' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY'
type EndMode = 'never' | 'count' | 'until'

interface RuleState {
    freq: Frequency
    interval: number
    byday: string[]
    endMode: EndMode
    count: number
    until: string
}

function pad(n: number): string {
    return n < 10 ? '0' + n : String(n)
}

// weekdayOf returns the RFC 5545 weekday code of a date-or-date-time input value, built from a local date
// so the weekday does not shift with the timezone. It is undefined when the input has no date part.
function weekdayOf(input: string): string | undefined {
    const match = input.match(/^(\d{4})-(\d{2})-(\d{2})/)
    if (!match) return undefined
    return DAY_CODES[new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3])).getDay()]
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
    const state: RuleState = {freq: '', interval: MIN_INTERVAL, byday: [], endMode: 'never', count: DEFAULT_COUNT, until: ''}
    const trimmed = value.trim().replace(/^RRULE:/i, '')
    if (trimmed === '') return state
    for (const part of trimmed.split(';')) {
        const [rawKey, rawValue] = part.split('=')
        const key = (rawKey || '').toUpperCase()
        const val = rawValue || ''
        if (key === 'FREQ') state.freq = val.toUpperCase() as Frequency
        else if (key === 'INTERVAL') state.interval = Math.max(MIN_INTERVAL, parseInt(val, 10) || MIN_INTERVAL)
        else if (key === 'BYDAY') state.byday = val.split(',').map((d) => d.toUpperCase()).filter(Boolean)
        else if (key === 'COUNT') {
            state.endMode = 'count'
            state.count = Math.max(1, parseInt(val, 10) || DEFAULT_COUNT)
        } else if (key === 'UNTIL') {
            state.endMode = 'until'
            state.until = untilToDateInput(val)
        }
    }
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
// repeat.
function buildRule(state: RuleState): string {
    if (state.freq === '') return ''
    const parts = [`FREQ=${state.freq}`]
    if (state.interval > MIN_INTERVAL) parts.push(`INTERVAL=${state.interval}`)
    if (state.freq === 'WEEKLY' && state.byday.length > 0) parts.push(`BYDAY=${state.byday.join(',')}`)
    if (state.endMode === 'count' && state.count > 0) parts.push(`COUNT=${state.count}`)
    else if (state.endMode === 'until' && state.until !== '') parts.push(`UNTIL=${dateInputToUntil(state.until)}`)
    return parts.join(';')
}

// intervalUnit names the interval unit for the active frequency, pluralised when the interval is not one.
function intervalUnit(freq: Frequency, interval: number): string {
    const unit = freq === 'DAILY' ? 'day' : freq === 'WEEKLY' ? 'week' : freq === 'MONTHLY' ? 'month' : 'year'
    return interval === 1 ? unit : `${unit}s`
}

interface RecurrenceEditorProps {
    value: string
    onChange: (rule: string) => void
    // startDate is the event's start (a date or date-time input value). A weekly rule with no weekday
    // chosen repeats on this weekday, so it is shown as the default selection.
    startDate: string
}

export function RecurrenceEditor({value, onChange, startDate}: RecurrenceEditorProps) {
    const [state, setState] = useState<RuleState>(() => parseRule(value))

    // Re-sync only when a genuinely different rule arrives (a different event opened). The rule string
    // cannot represent "ends on a date that has not been picked yet" (an until with no date builds a rule
    // with no UNTIL), so deriving state from the rule every render would snap that choice back to "never".
    // Holding it in state keeps the selection until the user picks a date. buildRule(state) equals the last
    // value we emitted, so our own edits never trigger a resync.
    useEffect(() => {
        if (value !== buildRule(state)) setState(parseRule(value))
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [value])

    const untilRef = useRef<HTMLInputElement>(null)
    const minUntil = tomorrowInput()

    const update = (patch: Partial<RuleState>) => {
        const next = {...state, ...patch}
        setState(next)
        onChange(buildRule(next))
    }

    // changeEndMode switches how the series ends. Choosing an end date with none set yet seeds a valid
    // future default so the rule carries an UNTIL immediately and the picker opens on a sensible day.
    const changeEndMode = (mode: EndMode) => {
        if (mode === 'until' && state.until === '') update({endMode: mode, until: minUntil})
        else update({endMode: mode})
    }

    // changeUntil keeps the chosen end date in the future, clamping an earlier or cleared value up to the
    // minimum. yyyy-mm-dd strings compare correctly as text.
    const changeUntil = (value: string) => update({until: value < minUntil ? minUntil : value})

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
    const defaultDay = weekdayOf(startDate)
    const dayActive = (code: string) => state.byday.includes(code) || (state.byday.length === 0 && code === defaultDay)

    return (
        <div className="recurrence-editor">
            <div className="rule-form-row">
                <select className="tag-name-input" aria-label="Repeat" value={state.freq}
                        onChange={(e) => update({freq: e.target.value as Frequency})}>
                    {FREQUENCIES.map((f) => (<option key={f.value} value={f.value}>{f.label}</option>))}
                </select>
                {state.freq !== '' && (
                    <label className="recur-interval">
                        every
                        <input className="tag-name-input recur-interval-num" type="number" min={MIN_INTERVAL}
                               value={state.interval}
                               onChange={(e) => update({interval: Math.max(MIN_INTERVAL, Number(e.target.value) || MIN_INTERVAL)})}/>
                        {intervalUnit(state.freq, state.interval)}
                    </label>
                )}
            </div>

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
