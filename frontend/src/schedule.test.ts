// The shared schedule module behind Send later and Snooze: the preset moments (with their rollover
// rules) and the datetime-local bridge. Everything takes an explicit now, so each rule is pinned with
// fixed dates.
import {describe, expect, it} from 'vitest'
import {fromDatetimeLocal, isSchedulable, sendLaterChoices, snoozeChoices, toDatetimeLocal} from './schedule'

// A Tuesday morning and a Tuesday night, local time, to drive the evening rollover both ways.
const tuesdayMorning = new Date(2026, 6, 14, 8, 30)
const tuesdayNight = new Date(2026, 6, 14, 21, 0)
const monday = new Date(2026, 6, 13, 10, 0)

describe('sendLaterChoices', () => {
    it('offers the hour, the coming evening and tomorrow morning before 18:00', () => {
        const choices = sendLaterChoices(tuesdayMorning)
        expect(choices.map((c) => c.label)).toEqual(['In 1 hour', 'This evening (18:00)', 'Tomorrow morning (09:00)'])
        expect(choices[0].at).toEqual(new Date(2026, 6, 14, 9, 30))
        expect(choices[1].at).toEqual(new Date(2026, 6, 14, 18, 0))
        expect(choices[2].at).toEqual(new Date(2026, 6, 15, 9, 0))
    })

    it('rolls a passed evening to tomorrow', () => {
        const choices = sendLaterChoices(tuesdayNight)
        expect(choices[1].label).toBe('Tomorrow evening (18:00)')
        expect(choices[1].at).toEqual(new Date(2026, 6, 15, 18, 0))
    })

    it('every choice is in the future', () => {
        for (const choice of sendLaterChoices(tuesdayNight)) {
            expect(choice.at.getTime()).toBeGreaterThan(tuesdayNight.getTime())
        }
    })
})

describe('snoozeChoices', () => {
    it('offers three hours, tomorrow morning and next Monday', () => {
        const choices = snoozeChoices(tuesdayMorning)
        expect(choices.map((c) => c.label)).toEqual(['In 3 hours', 'Tomorrow morning (09:00)', 'Next Monday (09:00)'])
        expect(choices[0].at).toEqual(new Date(2026, 6, 14, 11, 30))
        expect(choices[1].at).toEqual(new Date(2026, 6, 15, 9, 0))
        // 2026-07-14 is a Tuesday, so next Monday is the 20th.
        expect(choices[2].at).toEqual(new Date(2026, 6, 20, 9, 0))
    })

    it('on a Monday, next Monday means the following week', () => {
        const choices = snoozeChoices(monday)
        expect(choices[2].at).toEqual(new Date(2026, 6, 20, 9, 0))
    })
})

describe('datetime-local bridge', () => {
    it('round-trips a local moment at minute precision', () => {
        const moment = new Date(2026, 0, 5, 7, 4)
        expect(toDatetimeLocal(moment)).toBe('2026-01-05T07:04')
        expect(fromDatetimeLocal('2026-01-05T07:04')).toEqual(moment)
    })

    it('returns null for an empty or unparseable value', () => {
        expect(fromDatetimeLocal('')).toBeNull()
        expect(fromDatetimeLocal('not-a-date')).toBeNull()
    })
})

describe('isSchedulable', () => {
    it('requires a present, strictly future moment', () => {
        const now = tuesdayMorning
        expect(isSchedulable(null, now)).toBe(false)
        expect(isSchedulable(new Date(now.getTime() - 1), now)).toBe(false)
        expect(isSchedulable(now, now)).toBe(false)
        expect(isSchedulable(new Date(now.getTime() + 1), now)).toBe(true)
    })
})
