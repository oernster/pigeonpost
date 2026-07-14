// The Snoozed view's seam: the synthetic folder id and the due-time row label.
import {describe, expect, it} from 'vitest'
import {SNOOZED_FOLDER_ID, isSnoozedFolder, snoozedUntilLabel} from './snooze'

describe('isSnoozedFolder', () => {
    it('matches only the synthetic snoozed id', () => {
        expect(isSnoozedFolder(SNOOZED_FOLDER_ID)).toBe(true)
        expect(isSnoozedFolder('INBOX')).toBe(false)
        expect(isSnoozedFolder('')).toBe(false)
    })
})

describe('snoozedUntilLabel', () => {
    it('names the due moment', () => {
        const label = snoozedUntilLabel(new Date(2026, 6, 15, 9, 0).getTime())
        expect(label.startsWith('Until ')).toBe(true)
        expect(label).toContain('15')
        expect(label).toContain('09:00')
    })

    it('is empty without a snooze or for a broken timestamp', () => {
        expect(snoozedUntilLabel(0)).toBe('')
        expect(snoozedUntilLabel(-5)).toBe('')
        expect(snoozedUntilLabel(Number.NaN)).toBe('')
    })
})
