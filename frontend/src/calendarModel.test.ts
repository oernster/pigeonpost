import {describe, expect, it} from 'vitest'
import {
    attendeeStatusLabel,
    dateInput,
    dateTimeInput,
    extractUrls,
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
