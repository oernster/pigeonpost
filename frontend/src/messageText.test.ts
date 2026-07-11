import {describe, expect, it} from 'vitest'
import {Message} from './api'
import {emlFilename, escapeHtml, neighbourAfterRemoval, subjectWithPrefix} from './messageText'

const msg = (id: string): Message => ({id} as Message)

describe('escapeHtml', () => {
    it('escapes ampersand, less-than and greater-than', () => {
        expect(escapeHtml('a & b < c > d')).toBe('a &amp; b &lt; c &gt; d')
    })
})

describe('subjectWithPrefix', () => {
    it('adds the prefix when it is absent', () => {
        expect(subjectWithPrefix('Re:', 'Lunch')).toBe('Re: Lunch')
    })

    it('leaves an already-prefixed subject unchanged, case-insensitively', () => {
        expect(subjectWithPrefix('Re:', 're: Lunch')).toBe('re: Lunch')
    })

    it('falls back to a placeholder for an empty subject', () => {
        expect(subjectWithPrefix('Fwd:', '')).toBe('Fwd: (no subject)')
    })
})

describe('emlFilename', () => {
    it('replaces a filesystem-illegal character with a dash', () => {
        expect(emlFilename('report/2026')).toBe('report-2026.eml')
    })

    it('falls back to a default when the subject is only whitespace', () => {
        expect(emlFilename('   ')).toBe('message.eml')
    })
})

describe('neighbourAfterRemoval', () => {
    const list = [msg('a'), msg('b'), msg('c')]

    it('returns null when the id is not present', () => {
        expect(neighbourAfterRemoval(list, 'z')).toBeNull()
    })

    it('returns the following message when one exists', () => {
        expect(neighbourAfterRemoval(list, 'b')?.id).toBe('c')
    })

    it('returns the preceding message when the removed one was last', () => {
        expect(neighbourAfterRemoval(list, 'c')?.id).toBe('b')
    })

    it('returns null when the removed one was the only message', () => {
        expect(neighbourAfterRemoval([msg('only')], 'only')).toBeNull()
    })
})
