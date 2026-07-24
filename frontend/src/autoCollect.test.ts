import {describe, expect, it} from 'vitest'
import {autoCollectStored, collectableRecipients, shouldAutoCollect} from './autoCollect'

describe('the auto-collect setting', () => {
    it('is on by default: an absent key and unknown values mean on', () => {
        expect(shouldAutoCollect(null)).toBe(true)
        expect(shouldAutoCollect('1')).toBe(true)
        expect(shouldAutoCollect('anything')).toBe(true)
    })

    it('only the stored off marker disables it', () => {
        expect(shouldAutoCollect('0')).toBe(false)
    })

    it('round-trips through its encoding', () => {
        expect(shouldAutoCollect(autoCollectStored(true))).toBe(true)
        expect(shouldAutoCollect(autoCollectStored(false))).toBe(false)
    })
})

describe('collectableRecipients', () => {
    it('gathers To, Cc and Bcc, deduplicated case-insensitively', () => {
        const got = collectableRecipients('a@x.example, B@x.example', 'b@X.example', 'c@x.example', [])
        expect(got).toEqual(['a@x.example', 'B@x.example', 'c@x.example'])
    })

    it('drops the sender\'s own addresses', () => {
        const got = collectableRecipients('me@mine.example, other@x.example', '', '', ['ME@mine.example'])
        expect(got).toEqual(['other@x.example'])
    })

    it('is empty for empty fields', () => {
        expect(collectableRecipients('', '', '', ['me@mine.example'])).toEqual([])
    })
})
