import {describe, expect, it} from 'vitest'
import {
    attendeeStatusLabel,
    contrastInk,
    dateInput,
    dateTimeInput,
    dayIndex,
    eventDaySpan,
    extractUrls,
    layoutWeek,
    MONTH_MAX_LANES,
    meetingProvider,
    monthCells,
    pad,
    sameDay,
    weekDays,
} from './calendarModel'

describe('extractUrls', () => {
    it('pulls a url out of text and trims trailing punctuation', () => {
        expect(extractUrls('Join at https://meet.google.com/abc.')).toEqual(['https://meet.google.com/abc'])
    })

    it('returns an empty list when there is no url', () => {
        expect(extractUrls('no links here')).toEqual([])
    })
})

describe('meetingProvider', () => {
    it('names Microsoft Teams for both teams hosts', () => {
        expect(meetingProvider('https://teams.microsoft.com/l/meetup')).toBe('Microsoft Teams')
        expect(meetingProvider('https://teams.live.com/meet/123')).toBe('Microsoft Teams')
    })

    it('names Google Meet', () => {
        expect(meetingProvider('https://meet.google.com/abc-defg-hij')).toBe('Google Meet')
    })

    it('names Zoom for the bare and subdomained hosts', () => {
        expect(meetingProvider('https://zoom.us/j/123')).toBe('Zoom')
        expect(meetingProvider('https://us02web.zoom.us/j/123')).toBe('Zoom')
    })

    it('names Webex', () => {
        expect(meetingProvider('https://acme.webex.com/meet/x')).toBe('Webex')
    })

    it('returns null for an unrecognised host', () => {
        expect(meetingProvider('https://example.com/x')).toBeNull()
    })

    it('returns null for an unparseable url', () => {
        expect(meetingProvider('not a url')).toBeNull()
    })
})

describe('attendeeStatusLabel', () => {
    it('maps a known PARTSTAT to a label', () => {
        expect(attendeeStatusLabel('ACCEPTED')).toBe('Accepted')
    })

    it('falls back to the raw value for an unknown status', () => {
        expect(attendeeStatusLabel('CUSTOM')).toBe('CUSTOM')
    })
})

describe('pad', () => {
    it('zero-pads a single digit', () => {
        expect(pad(3)).toBe('03')
    })

    it('leaves a two-digit number unchanged', () => {
        expect(pad(11)).toBe('11')
    })
})

describe('dateInput and dateTimeInput', () => {
    const d = new Date(2026, 0, 5, 9, 4)

    it('formats a date as YYYY-MM-DD', () => {
        expect(dateInput(d)).toBe('2026-01-05')
    })

    it('formats a date and time as YYYY-MM-DDTHH:MM', () => {
        expect(dateTimeInput(d)).toBe('2026-01-05T09:04')
    })
})

describe('sameDay', () => {
    it('is false when the year differs', () => {
        expect(sameDay(new Date(2025, 5, 1), new Date(2026, 5, 1))).toBe(false)
    })

    it('is false when the month differs', () => {
        expect(sameDay(new Date(2026, 4, 1), new Date(2026, 5, 1))).toBe(false)
    })

    it('is false when the day differs', () => {
        expect(sameDay(new Date(2026, 5, 1), new Date(2026, 5, 2))).toBe(false)
    })

    it('is true for two times on the same calendar day', () => {
        expect(sameDay(new Date(2026, 5, 1, 8, 0), new Date(2026, 5, 1, 20, 0))).toBe(true)
    })
})

describe('monthCells', () => {
    it('returns 42 cells starting on the Sunday on or before the first', () => {
        // January 2026: 1 Jan 2026 is a Thursday, so the grid starts Sunday 28 Dec 2025.
        const cells = monthCells(new Date(2026, 0, 15))
        expect(cells).toHaveLength(42)
        expect(cells[0].getDay()).toBe(0)
        expect(dateInput(cells[0])).toBe('2025-12-28')
    })
})

describe('weekDays', () => {
    it('returns the seven days from the Sunday on or before the date', () => {
        // 15 Jan 2026 is a Thursday, so its week starts Sunday 11 Jan 2026.
        const days = weekDays(new Date(2026, 0, 15))
        expect(days).toHaveLength(7)
        expect(days[0].getDay()).toBe(0)
        expect(dateInput(days[0])).toBe('2026-01-11')
    })
})

describe('dayIndex', () => {
    it('gives consecutive local days consecutive integers', () => {
        expect(dayIndex(new Date(2026, 6, 14)) - dayIndex(new Date(2026, 6, 13))).toBe(1)
    })

    it('ignores the time of day', () => {
        expect(dayIndex(new Date(2026, 6, 13, 23, 59))).toBe(dayIndex(new Date(2026, 6, 13, 0, 0)))
    })
})

describe('eventDaySpan', () => {
    it('covers every day from start to end for a multi-day timed event', () => {
        const span = eventDaySpan(new Date(2026, 6, 13, 9, 0).getTime(), new Date(2026, 6, 22, 15, 0).getTime())
        expect(span.firstDay).toBe(dayIndex(new Date(2026, 6, 13)))
        expect(span.lastDay).toBe(dayIndex(new Date(2026, 6, 22)))
        expect(span.lastDay - span.firstDay).toBe(9)
    })

    it('does not occupy the day an event ends on at midnight', () => {
        const span = eventDaySpan(new Date(2026, 6, 13, 0, 0).getTime(), new Date(2026, 6, 14, 0, 0).getTime())
        expect(span.lastDay).toBe(span.firstDay)
    })

    it('occupies only the start day for a same-day event', () => {
        const span = eventDaySpan(new Date(2026, 6, 13, 9, 0).getTime(), new Date(2026, 6, 13, 10, 0).getTime())
        expect(span.lastDay).toBe(span.firstDay)
    })

    it('occupies the start day when the range is empty or reversed', () => {
        const day = dayIndex(new Date(2026, 6, 13))
        const span = eventDaySpan(new Date(2026, 6, 13, 9, 0).getTime(), new Date(2026, 6, 13, 9, 0).getTime())
        expect(span).toEqual({firstDay: day, lastDay: day})
    })
})

describe('layoutWeek', () => {
    it('spans a multi-day event across its columns and squares the clipped right end', () => {
        const {bars} = layoutWeek(0, [{key: 'v', firstDay: 1, lastDay: 9}], MONTH_MAX_LANES)
        expect(bars).toHaveLength(1)
        expect(bars[0]).toMatchObject({startCol: 1, endCol: 6, lane: 0, continuesLeft: false, continuesRight: true})
    })

    it('continues a bar from the previous week, squared on the left', () => {
        const {bars} = layoutWeek(7, [{key: 'v', firstDay: 1, lastDay: 10}], MONTH_MAX_LANES)
        expect(bars[0]).toMatchObject({startCol: 0, endCol: 3, continuesLeft: true, continuesRight: false})
    })

    it('stacks overlapping events into separate lanes and reuses a freed lane', () => {
        const {bars, lanes} = layoutWeek(0, [
            {key: 'a', firstDay: 0, lastDay: 3},
            {key: 'b', firstDay: 2, lastDay: 5},
            {key: 'c', firstDay: 4, lastDay: 6},
        ], MONTH_MAX_LANES)
        const laneOf = (k: string) => bars.find((b) => b.key === k)!.lane
        expect(laneOf('a')).toBe(0)
        expect(laneOf('b')).toBe(1)
        expect(laneOf('c')).toBe(0)
        expect(lanes).toBe(2)
    })

    it('drops events past maxLanes into per-day overflow', () => {
        const {bars, overflow, lanes} = layoutWeek(0, [
            {key: 'a', firstDay: 0, lastDay: 6},
            {key: 'b', firstDay: 0, lastDay: 6},
        ], 1)
        expect(bars.map((b) => b.key)).toEqual(['a'])
        expect(lanes).toBe(1)
        expect(overflow).toEqual([1, 1, 1, 1, 1, 1, 1])
    })

    it('ignores an event that falls outside the week', () => {
        const {bars} = layoutWeek(0, [{key: 'x', firstDay: 10, lastDay: 12}], MONTH_MAX_LANES)
        expect(bars).toHaveLength(0)
    })

    it('places a single-day event in one column', () => {
        const {bars} = layoutWeek(0, [{key: 's', firstDay: 3, lastDay: 3}], MONTH_MAX_LANES)
        expect(bars[0]).toMatchObject({startCol: 3, endCol: 3, continuesLeft: false, continuesRight: false})
    })
})

describe('contrastInk', () => {
    it('picks dark ink on a light fill and light ink on a dark fill', () => {
        expect(contrastInk('#7fb0ff')).toBe('#0b1220')
        expect(contrastInk('#0f6e56')).toBe('#ffffff')
    })

    it('falls back to dark ink for an unparseable colour', () => {
        expect(contrastInk('nope')).toBe('#0b1220')
    })
})
