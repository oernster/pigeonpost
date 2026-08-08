import {describe, expect, it} from 'vitest'
import {
    WATERMARKS_KEY, elsewhereCue, elsewhereCueLabel, ensureWatermarks, loadWatermarks,
    saveWatermarks, touchWatermarks, type Watermarks,
} from './newMail'

// fakeStorage is the minimal Storage slice the module reads and writes, backed by a plain map.
function fakeStorage(initial: {[key: string]: string} = {}) {
    const data = {...initial}
    return {
        data,
        getItem: (key: string) => (key in data ? data[key] : null),
        setItem: (key: string, value: string) => {
            data[key] = value
        },
    }
}

describe('loadWatermarks', () => {
    it('returns empty for a missing slot', () => {
        expect(loadWatermarks(fakeStorage())).toEqual({})
    })

    it('returns empty for unparseable JSON', () => {
        expect(loadWatermarks(fakeStorage({[WATERMARKS_KEY]: 'not json'}))).toEqual({})
    })

    it('returns empty for a non-object value', () => {
        expect(loadWatermarks(fakeStorage({[WATERMARKS_KEY]: '[1,2]'}))).toEqual({})
        expect(loadWatermarks(fakeStorage({[WATERMARKS_KEY]: 'null'}))).toEqual({})
    })

    it('keeps only finite numeric entries', () => {
        const raw = JSON.stringify({a1: 100, a2: 'soon', a3: null})
        expect(loadWatermarks(fakeStorage({[WATERMARKS_KEY]: raw}))).toEqual({a1: 100})
    })

    it('returns empty when the storage read throws', () => {
        const throwing = {
            getItem: () => {
                throw new Error('blocked')
            },
        }
        expect(loadWatermarks(throwing)).toEqual({})
    })
})

describe('saveWatermarks', () => {
    it('round-trips through loadWatermarks', () => {
        const storage = fakeStorage()
        saveWatermarks(storage, {a1: 100, a2: 200})
        expect(loadWatermarks(storage)).toEqual({a1: 100, a2: 200})
    })

    it('swallows a storage write failure', () => {
        const throwing = {
            setItem: () => {
                throw new Error('full')
            },
        }
        expect(() => saveWatermarks(throwing, {a1: 100})).not.toThrow()
    })
})

describe('touchWatermarks', () => {
    it('stamps each named account at now without mutating the input', () => {
        const marks: Watermarks = {a1: 100}
        const next = touchWatermarks(marks, ['a1', 'a2'], 500)
        expect(next).toEqual({a1: 500, a2: 500})
        expect(marks).toEqual({a1: 100})
    })

    it('skips empty ids, as when no account was selected yet', () => {
        expect(touchWatermarks({}, ['', 'a1'], 500)).toEqual({a1: 500})
    })
})

describe('ensureWatermarks', () => {
    it('returns the same object when the accounts already match', () => {
        const marks: Watermarks = {a1: 100, a2: 200}
        expect(ensureWatermarks(marks, ['a1', 'a2'], 500)).toBe(marks)
    })

    it('starts a first-seen account at now, so its backlog stays dark', () => {
        expect(ensureWatermarks({a1: 100}, ['a1', 'a2'], 500)).toEqual({a1: 100, a2: 500})
    })

    it('drops an account no longer in the list', () => {
        expect(ensureWatermarks({a1: 100, gone: 50}, ['a1'], 500)).toEqual({a1: 100})
    })
})

describe('elsewhereCue', () => {
    const unread = {active: 3, fresh: 2, stale: 4, quiet: 1}
    const newest = {active: 900, fresh: 800, stale: 300, quiet: 700}
    const marks: Watermarks = {active: 100, fresh: 500, stale: 500, quiet: 700}

    it('counts only non-selected accounts whose newest unread postdates the watermark', () => {
        // fresh: 800 > 500 lights. stale: 300 < 500 stays dark. quiet: 700 == 700 stays dark.
        // active is excluded outright even though its newest postdates its watermark.
        expect(elsewhereCue('active', unread, newest, marks)).toEqual({unread: 2, accounts: 1})
    })

    it('sums across several newly arrived accounts', () => {
        const cue = elsewhereCue('other', unread, newest, {...marks, active: 100})
        expect(cue).toEqual({unread: 5, accounts: 2})
    })

    it('stays dark for an account with no watermark yet', () => {
        expect(elsewhereCue('active', unread, newest, {})).toEqual({unread: 0, accounts: 0})
    })

    it('stays dark for an account with no newest-unread entry', () => {
        expect(elsewhereCue('active', {fresh: 2}, {}, marks)).toEqual({unread: 0, accounts: 0})
    })

    it('ignores a zero or negative count', () => {
        expect(elsewhereCue('active', {fresh: 0}, newest, marks)).toEqual({unread: 0, accounts: 0})
    })
})

describe('elsewhereCueLabel', () => {
    it('uses the singular for one account', () => {
        expect(elsewhereCueLabel({unread: 3, accounts: 1})).toBe('3 unread on 1 other account')
    })

    it('uses the plural for several accounts', () => {
        expect(elsewhereCueLabel({unread: 5, accounts: 2})).toBe('5 unread on 2 other accounts')
    })
})
