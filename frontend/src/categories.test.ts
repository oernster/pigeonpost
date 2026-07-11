import {describe, expect, it} from 'vitest'
import {EVENT_CATEGORIES, categoryEmoji} from './categories'

describe('categoryEmoji', () => {
    it('returns the emoji for a known value', () => {
        expect(categoryEmoji('work')).toBe('💼')
        expect(categoryEmoji('celebration')).toBe('🎉')
    })

    it('returns an empty string for an unknown value', () => {
        expect(categoryEmoji('nonsense')).toBe('')
    })

    it('returns an empty string for an empty or whitespace value', () => {
        expect(categoryEmoji('')).toBe('')
        expect(categoryEmoji('   ')).toBe('')
    })

    it('matches case-insensitively and ignores surrounding whitespace', () => {
        expect(categoryEmoji('  WORK ')).toBe('💼')
        expect(categoryEmoji('Meeting')).toBe('👥')
    })

    it('has an emoji for every offered category', () => {
        for (const c of EVENT_CATEGORIES) {
            expect(categoryEmoji(c.value)).toBe(c.emoji)
        }
    })
})
