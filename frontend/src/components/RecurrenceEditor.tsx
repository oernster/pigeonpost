// RecurrenceEditor is a friendly builder for an RFC 5545 RRULE string. It is stateless: it parses the
// current rule on every render and calls onChange with the rebuilt rule, so the parent owns the value.
// It covers the common calendar cases (daily, weekly with weekdays, monthly, yearly; an interval; and an
// end of never, after a count, or on a date); rarer rule parts are not offered but are preserved only if
// the parent does not overwrite them, so this editor is used for app-authored rules.

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
}

export function RecurrenceEditor({value, onChange}: RecurrenceEditorProps) {
    const state = parseRule(value)
    const update = (patch: Partial<RuleState>) => onChange(buildRule({...state, ...patch}))

    const toggleDay = (code: string) => {
        const next = state.byday.includes(code)
            ? state.byday.filter((d) => d !== code)
            : [...state.byday, code]
        // Keep BYDAY in canonical weekday order so the rule is stable regardless of click order.
        const ordered = WEEKDAYS.map((w) => w.code).filter((c) => next.includes(c))
        update({byday: ordered})
    }

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
                                className={'recur-day' + (state.byday.includes(w.code) ? ' active' : '')}
                                aria-pressed={state.byday.includes(w.code)}
                                onClick={() => toggleDay(w.code)}>{w.label}</button>
                    ))}
                </div>
            )}

            {state.freq !== '' && (
                <div className="rule-form-row recur-end">
                    <select className="tag-name-input" aria-label="Ends" value={state.endMode}
                            onChange={(e) => update({endMode: e.target.value as EndMode})}>
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
                        <input className="tag-name-input" type="date" aria-label="Repeat until" value={state.until}
                               onChange={(e) => update({until: e.target.value})}/>
                    )}
                </div>
            )}
        </div>
    )
}
