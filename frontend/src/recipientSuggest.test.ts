import {describe, expect, it} from 'vitest'
import {
    activeFragment,
    applySuggestion,
    MAX_SUGGESTIONS,
    Suggestion,
    suggestionPool,
    suggestionsFor,
} from './recipientSuggest'

function contact(formattedName: string, addresses: string[], given = '', family = '') {
    return {formattedName, givenName: given, familyName: family, emails: addresses.map((address) => ({address}))}
}

describe('suggestionPool', () => {
    it('flattens contacts into one entry per address with a display label', () => {
        const pool = suggestionPool([
            contact('Jane Doe', ['jane@example.com', 'jd@work.example']),
            contact('', ['bare@example.com']),
        ])
        expect(pool).toEqual([
            {address: 'jane@example.com', name: 'Jane Doe', display: 'Jane Doe <jane@example.com>'},
            {address: 'jd@work.example', name: 'Jane Doe', display: 'Jane Doe <jd@work.example>'},
            {address: 'bare@example.com', name: '', display: 'bare@example.com'},
        ])
    })

    it('derives a name from given and family when no formatted name exists', () => {
        const pool = suggestionPool([contact('', ['a@b.example'], 'Ada', 'Lovelace')])
        expect(pool[0].name).toBe('Ada Lovelace')
    })

    it('dedupes addresses case-insensitively, first holder winning', () => {
        const pool = suggestionPool([
            contact('First', ['Shared@Example.com']),
            contact('Second', ['shared@example.com', '  ']),
        ])
        expect(pool).toHaveLength(1)
        expect(pool[0].name).toBe('First')
    })
})

describe('activeFragment', () => {
    it('is the whole value for a single address', () => {
        expect(activeFragment('jan')).toEqual({start: 0, text: 'jan'})
    })

    it('starts after the last separator, skipping its space', () => {
        expect(activeFragment('a@b.example, jan')).toEqual({start: 13, text: 'jan'})
        expect(activeFragment('a@b.example;jan')).toEqual({start: 12, text: 'jan'})
        expect(activeFragment('a@b.example, ')).toEqual({start: 13, text: ''})
    })
})

const pool: Suggestion[] = suggestionPool([
    contact('Jane Doe', ['jane@example.com']),
    contact('Janet Frame', ['janet@books.example']),
    contact('Bob Stone', ['bob@quarry.example']),
    contact('', ['zeta@last.example']),
])

describe('suggestionsFor', () => {
    it('offers nothing until typing starts', () => {
        expect(suggestionsFor(pool, '')).toEqual([])
        expect(suggestionsFor(pool, 'a@b.example, ')).toEqual([])
    })

    it('matches name or address, ranking start-matches first', () => {
        const got = suggestionsFor(pool, 'jan').map((s) => s.address)
        expect(got).toEqual(['jane@example.com', 'janet@books.example'])
        // "quarry" sits mid-address, so it matches as an anywhere-match.
        expect(suggestionsFor(pool, 'quarry').map((s) => s.address)).toEqual(['bob@quarry.example'])
    })

    it('matches the fragment after the last separator only', () => {
        const got = suggestionsFor(pool, 'bob@quarry.example, jan').map((s) => s.address)
        expect(got).toEqual(['jane@example.com', 'janet@books.example'])
    })

    it('never re-offers an address already in the field', () => {
        const got = suggestionsFor(pool, 'jane@example.com, jan').map((s) => s.address)
        expect(got).toEqual(['janet@books.example'])
    })

    it('is case-insensitive', () => {
        // "jane" is also a prefix of "janet", so both match; case must not change that.
        expect(suggestionsFor(pool, 'JANE').map((s) => s.address))
            .toEqual(['jane@example.com', 'janet@books.example'])
    })

    it('caps the list', () => {
        const many = suggestionPool(
            Array.from({length: MAX_SUGGESTIONS + 3}, (_, i) => contact(`User ${i}`, [`user${i}@example.com`])),
        )
        expect(suggestionsFor(many, 'user')).toHaveLength(MAX_SUGGESTIONS)
        expect(suggestionsFor(many, 'user', 2)).toHaveLength(2)
    })
})

describe('applySuggestion', () => {
    const jane = pool[0]

    it('replaces a lone fragment', () => {
        expect(applySuggestion('jan', jane)).toBe('jane@example.com')
    })

    it('replaces only the fragment after the last separator', () => {
        expect(applySuggestion('bob@quarry.example, jan', jane)).toBe('bob@quarry.example, jane@example.com')
    })

    it('adds the missing space after a tight separator', () => {
        expect(applySuggestion('bob@quarry.example,jan', jane)).toBe('bob@quarry.example, jane@example.com')
    })
})
