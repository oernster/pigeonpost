import {describe, expect, it} from 'vitest'
import {
    addMonths,
    beforeMin,
    datePart,
    isoDate,
    mergeDate,
    monthGrid,
    monthLabel,
    monthOf,
    timePart,
    WEEKDAY_LABELS,
} from './datePicker'

describe('value parts', () => {
    it('splits date and time from both field kinds', () => {
        expect(datePart('2026-07-24')).toBe('2026-07-24')
        expect(datePart('2026-07-24T09:30')).toBe('2026-07-24')
        expect(datePart('')).toBe('')
        expect(timePart('2026-07-24T09:30')).toBe('09:30')
        expect(timePart('2026-07-24')).toBe('')
    })

    it('formats zero-padded ISO dates', () => {
        expect(isoDate(2026, 7, 4)).toBe('2026-07-04')
        expect(isoDate(2026, 11, 30)).toBe('2026-11-30')
    })
})

describe('calendar pages', () => {
    it('reads the page a value sits on, with a fallback', () => {
        const fallback = {year: 2020, month: 1}
        expect(monthOf('2026-07-24', fallback)).toEqual({year: 2026, month: 7})
        expect(monthOf('', fallback)).toEqual(fallback)
        expect(monthOf('nonsense', fallback)).toEqual(fallback)
        expect(monthOf('2026-13-01', fallback)).toEqual(fallback)
    })

    it('steps months across year boundaries both ways', () => {
        expect(addMonths({year: 2026, month: 12}, 1)).toEqual({year: 2027, month: 1})
        expect(addMonths({year: 2026, month: 1}, -1)).toEqual({year: 2025, month: 12})
        expect(addMonths({year: 2026, month: 6}, -18)).toEqual({year: 2024, month: 12})
    })

    it('labels a page', () => {
        expect(monthLabel({year: 2026, month: 7})).toBe('July 2026')
    })
})

describe('monthGrid', () => {
    it('lays out July 2026 Monday-first with complete weeks', () => {
        const weeks = monthGrid({year: 2026, month: 7})
        expect(WEEKDAY_LABELS).toHaveLength(7)
        // 1 July 2026 is a Wednesday: the first week leads with 29 and 30 June.
        expect(weeks[0].map((c) => c.day)).toEqual([29, 30, 1, 2, 3, 4, 5])
        expect(weeks[0][0]).toEqual({iso: '2026-06-29', day: 29, inMonth: false})
        expect(weeks[0][2]).toEqual({iso: '2026-07-01', day: 1, inMonth: true})
        // 31 July is a Friday: the last week trails into August.
        const last = weeks[weeks.length - 1]
        expect(last.map((c) => c.day)).toEqual([27, 28, 29, 30, 31, 1, 2])
        expect(last[5]).toEqual({iso: '2026-08-01', day: 1, inMonth: false})
        for (const week of weeks) {
            expect(week).toHaveLength(7)
        }
    })

    it('handles a month starting on Monday and a leap February', () => {
        // June 2026 starts on a Monday: no leading cells.
        expect(monthGrid({year: 2026, month: 6})[0][0]).toEqual({iso: '2026-06-01', day: 1, inMonth: true})
        // February 2024 had 29 days.
        const feb = monthGrid({year: 2024, month: 2})
        const days = feb.flat().filter((c) => c.inMonth)
        expect(days).toHaveLength(29)
    })
})

describe('mergeDate', () => {
    it('replaces a date field outright', () => {
        expect(mergeDate('2020-01-01', '2026-07-24', 'date')).toBe('2026-07-24')
        expect(mergeDate('', '2026-07-24', 'date')).toBe('2026-07-24')
    })

    it('keeps a typed time on a datetime field and defaults an empty one', () => {
        expect(mergeDate('2020-01-01T17:45', '2026-07-24', 'datetime-local')).toBe('2026-07-24T17:45')
        expect(mergeDate('', '2026-07-24', 'datetime-local')).toBe('2026-07-24T09:00')
    })
})

describe('beforeMin', () => {
    it('disables only days before the minimum', () => {
        expect(beforeMin('2026-07-23', '2026-07-24')).toBe(true)
        expect(beforeMin('2026-07-24', '2026-07-24')).toBe(false)
        expect(beforeMin('2026-07-25', '2026-07-24T10:00')).toBe(false)
        expect(beforeMin('2026-07-23', '')).toBe(false)
    })
})
