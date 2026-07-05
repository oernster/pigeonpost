import {useEffect, useRef} from 'react'
import {CalendarEvent, CalendarEventInstance} from '../api'

const HOURS_IN_DAY = 24
const MINUTES_IN_HOUR = 60
const MINUTES_IN_DAY = HOURS_IN_DAY * MINUTES_IN_HOUR
const HOUR_ROW_PX = 44
const GUTTER_PX = 56
const SNAP_MINUTES = 30
const DEFAULT_EVENT_MINUTES = MINUTES_IN_HOUR
const MIN_EVENT_MINUTES = 24
const DEFAULT_SCROLL_HOUR = 7
const FULL_WIDTH_PERCENT = 100

const WEEKDAYS_SHORT = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

function pad(n: number): string {
    return n < 10 ? '0' + n : String(n)
}

function sameDay(a: Date, b: Date): boolean {
    return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
}

function minutesOfDay(d: Date): number {
    return d.getHours() * MINUTES_IN_HOUR + d.getMinutes()
}

function hhmm(minutes: number): string {
    return `${pad(Math.floor(minutes / MINUTES_IN_HOUR))}:${pad(minutes % MINUTES_IN_HOUR)}`
}

// occurrenceKey uniquely identifies an occurrence in the grid: the same recurring event yields many
// occurrences with the same event id, so the start time disambiguates them.
function occurrenceKey(i: CalendarEventInstance): string {
    return `${i.event.id}@${i.start}`
}

interface Positioned {
    inst: CalendarEventInstance
    startMin: number
    endMin: number
    lane: number
    lanes: number
}

// layoutDay places every timed occurrence that starts on `day` into a lane, assigning a shared lane
// count per overlap cluster so occurrences that clash in time sit side by side rather than hiding one
// another. An occurrence with no end (or an end past midnight) is clamped to a default height inside the
// day.
function layoutDay(day: Date, instances: CalendarEventInstance[]): Positioned[] {
    const items: Positioned[] = instances
        .filter((i) => !i.event.allDay && sameDay(new Date(i.start), day))
        .map((i) => {
            const start = new Date(i.start)
            const end = i.end ? new Date(i.end) : null
            const startMin = minutesOfDay(start)
            let endMin = end && sameDay(end, day) ? minutesOfDay(end) : MINUTES_IN_DAY
            if (endMin <= startMin) endMin = Math.min(startMin + DEFAULT_EVENT_MINUTES, MINUTES_IN_DAY)
            if (endMin - startMin < MIN_EVENT_MINUTES) endMin = Math.min(startMin + MIN_EVENT_MINUTES, MINUTES_IN_DAY)
            return {inst: i, startMin, endMin, lane: 0, lanes: 1}
        })
        .sort((a, b) => a.startMin - b.startMin || a.endMin - b.endMin)

    let cluster: Positioned[] = []
    let clusterEnd = -1
    const flush = () => {
        if (cluster.length === 0) return
        const laneEnds: number[] = []
        for (const it of cluster) {
            let placed = laneEnds.findIndex((end) => end <= it.startMin)
            if (placed === -1) {
                placed = laneEnds.length
                laneEnds.push(it.endMin)
            } else {
                laneEnds[placed] = it.endMin
            }
            it.lane = placed
        }
        for (const it of cluster) it.lanes = laneEnds.length
        cluster = []
        clusterEnd = -1
    }
    for (const it of items) {
        if (cluster.length > 0 && it.startMin >= clusterEnd) flush()
        cluster.push(it)
        clusterEnd = Math.max(clusterEnd, it.endMin)
    }
    flush()
    return items
}

function allDayFor(day: Date, instances: CalendarEventInstance[]): CalendarEventInstance[] {
    return instances.filter((i) => i.event.allDay && sameDay(new Date(i.start), day))
}

interface CalendarTimeGridProps {
    days: Date[]
    instances: CalendarEventInstance[]
    colourOf: (e: CalendarEvent) => string
    onNewAt: (start: Date) => void
    onEdit: (i: CalendarEventInstance) => void
}

// CalendarTimeGrid renders a Thunderbird-style day or week grid: an hour gutter, a pinned all-day strip and
// one column per day with timed occurrences positioned and sized by their start and end. Clicking empty
// space creates an event snapped to the nearest half hour; clicking an occurrence edits it.
export function CalendarTimeGrid({days, instances, colourOf, onNewAt, onEdit}: CalendarTimeGridProps) {
    const bodyRef = useRef<HTMLDivElement>(null)
    useEffect(() => {
        if (bodyRef.current) bodyRef.current.scrollTop = DEFAULT_SCROLL_HOUR * HOUR_ROW_PX
    }, [])

    const cols = `${GUTTER_PX}px repeat(${days.length}, 1fr)`
    const hours = Array.from({length: HOURS_IN_DAY}, (_, h) => h)
    const today = new Date()

    const createAt = (day: Date, offsetY: number) => {
        const raw = (offsetY / HOUR_ROW_PX) * MINUTES_IN_HOUR
        const snapped = Math.max(0, Math.min(MINUTES_IN_DAY - SNAP_MINUTES,
            Math.round(raw / SNAP_MINUTES) * SNAP_MINUTES))
        const start = new Date(day)
        start.setHours(0, snapped, 0, 0)
        onNewAt(start)
    }

    return (
        <div className="tg">
            <div className="tg-body" ref={bodyRef}>
                <div className="tg-sticky-top">
                    <div className="tg-header" style={{gridTemplateColumns: cols}}>
                        <div className="tg-corner"/>
                        {days.map((d, i) => (
                            <div key={i} className={'tg-dayhead' + (sameDay(d, today) ? ' tg-today' : '')}>
                                <div>{WEEKDAYS_SHORT[d.getDay()]}</div>
                                <div className="tg-dnum">{d.getDate()}</div>
                            </div>
                        ))}
                    </div>
                    <div className="tg-allday" style={{gridTemplateColumns: cols}}>
                        <div className="tg-allday-label">all-day</div>
                        {days.map((d, i) => (
                            <div key={i} className="tg-allday-col">
                                {allDayFor(d, instances).map((inst) => (
                                    <button key={occurrenceKey(inst)} className="tg-allday-ev" title={inst.event.summary}
                                            style={{borderLeft: `3px solid ${colourOf(inst.event)}`}}
                                            onClick={() => onEdit(inst)}>{inst.event.summary}</button>
                                ))}
                            </div>
                        ))}
                    </div>
                </div>
                <div className="tg-grid" style={{gridTemplateColumns: cols}}>
                    <div className="tg-gutter">
                        {hours.map((h) => (
                            <div key={h} className="tg-hour" style={{height: HOUR_ROW_PX}}>{pad(h)}:00</div>
                        ))}
                    </div>
                    {days.map((d, di) => {
                        const positioned = layoutDay(d, instances)
                        return (
                            <div key={di} className="tg-daycol"
                                 onClick={(ev) => createAt(d, ev.nativeEvent.offsetY)}>
                                {hours.map((h) => (
                                    <div key={h} className="tg-hourline" style={{height: HOUR_ROW_PX}}/>
                                ))}
                                {positioned.map((p) => {
                                    const top = (p.startMin / MINUTES_IN_HOUR) * HOUR_ROW_PX
                                    const height = ((p.endMin - p.startMin) / MINUTES_IN_HOUR) * HOUR_ROW_PX
                                    const width = FULL_WIDTH_PERCENT / p.lanes
                                    const left = p.lane * width
                                    return (
                                        <button key={occurrenceKey(p.inst)} className="tg-event" title={p.inst.event.summary}
                                                style={{top, height, left: `${left}%`, width: `${width}%`,
                                                    borderLeft: `3px solid ${colourOf(p.inst.event)}`}}
                                                onClick={(evt) => {
                                                    evt.stopPropagation()
                                                    onEdit(p.inst)
                                                }}>
                                            {hhmm(p.startMin)} {p.inst.event.summary}
                                        </button>
                                    )
                                })}
                            </div>
                        )
                    })}
                </div>
            </div>
        </div>
    )
}
