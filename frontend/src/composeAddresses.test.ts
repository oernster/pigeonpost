import {describe, expect, it} from 'vitest'
import {basename, detectSeparatorFix, isValidAddress, normaliseUrl, splitAddresses} from './composeAddresses'

describe('normaliseUrl', () => {
    it('returns an empty string unchanged', () => {
        expect(normaliseUrl('   ')).toBe('')
    })

    it('leaves an http, https or mailto url unchanged', () => {
        expect(normaliseUrl('https://example.com')).toBe('https://example.com')
        expect(normaliseUrl('mailto:a@b.com')).toBe('mailto:a@b.com')
    })

    it('adds https to a bare host', () => {
        expect(normaliseUrl(' example.com ')).toBe('https://example.com')
    })
})

describe('splitAddresses', () => {
    it('splits on commas and semicolons, dropping empty entries', () => {
        expect(splitAddresses('a@b.com, c@d.com; e@f.com,')).toEqual(['a@b.com', 'c@d.com', 'e@f.com'])
    })
})

describe('isValidAddress', () => {
    it('accepts a well-formed address', () => {
        expect(isValidAddress('a@b.com')).toBe(true)
    })

    it('rejects a malformed address', () => {
        expect(isValidAddress('not-an-address')).toBe(false)
    })
})

describe('detectSeparatorFix', () => {
    it('returns null when the field holds no address at all', () => {
        expect(detectSeparatorFix('just some words')).toBeNull()
    })

    it('returns null for a single address', () => {
        expect(detectSeparatorFix('a@b.com')).toBeNull()
    })

    it('returns null when the field is already correctly separated', () => {
        expect(detectSeparatorFix('a@b.com; c@d.com')).toBeNull()
    })

    it('rejoins addresses split by a wrong separator', () => {
        expect(detectSeparatorFix('a@b.com c@d.com')).toBe('a@b.com; c@d.com')
    })

    it('rejoins when a token count matches but a token is not a clean address', () => {
        expect(detectSeparatorFix('a@b.com, c@d.com extra')).toBe('a@b.com; c@d.com')
    })
})

describe('basename', () => {
    it('returns the final segment of a Windows path', () => {
        expect(basename('C:\\mail\\report.pdf')).toBe('report.pdf')
    })

    it('returns the final segment of a POSIX path', () => {
        expect(basename('/home/user/notes.txt')).toBe('notes.txt')
    })

    it('returns the whole path when it ends in a separator', () => {
        expect(basename('a/b/')).toBe('a/b/')
    })
})
