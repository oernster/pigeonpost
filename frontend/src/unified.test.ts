// The unified mailbox's frontend seam: the synthetic folder id the api module routes on, and the
// per-account colour chips the combined list labels its rows with.
import {describe, expect, it} from 'vitest'
import type {Account} from './api'
import {TAG_PALETTE} from './tagColours'
import {UNIFIED_FOLDER_ID, accountChips, isUnifiedFolder} from './unified'

function makeAccount(id: string, email: string): Account {
    return {id, displayName: id, email, protocol: 'imap'} as Account
}

describe('isUnifiedFolder', () => {
    it('matches only the synthetic unified id', () => {
        expect(isUnifiedFolder(UNIFIED_FOLDER_ID)).toBe(true)
        expect(isUnifiedFolder('INBOX')).toBe(false)
        expect(isUnifiedFolder('')).toBe(false)
    })
})

describe('accountChips', () => {
    it('assigns palette colours by sidebar order with the email as the label', () => {
        const chips = accountChips([makeAccount('a1', 'alice@x.com'), makeAccount('a2', 'bob@x.com')])
        expect(chips.get('a1')).toEqual({colour: TAG_PALETTE[0].colour, label: 'alice@x.com'})
        expect(chips.get('a2')).toEqual({colour: TAG_PALETTE[1].colour, label: 'bob@x.com'})
    })

    it('wraps around the palette when accounts outnumber its colours', () => {
        const accounts = Array.from(
            {length: TAG_PALETTE.length + 1},
            (_, i) => makeAccount(`a${i}`, `user${i}@x.com`),
        )
        const chips = accountChips(accounts)
        expect(chips.get(`a${TAG_PALETTE.length}`)?.colour).toBe(TAG_PALETTE[0].colour)
    })

    it('is empty for no accounts', () => {
        expect(accountChips([]).size).toBe(0)
    })
})
