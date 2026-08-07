import {describe, expect, it} from 'vitest'
import type {Message} from './api'
import {putBack, takeOut} from './optimisticList'

function makeMessage(id: string): Message {
    return {
        id, folderId: 'inbox', accountId: '', snoozedUntilMs: 0, subject: id, fromName: '',
        fromAddress: 'a@b.c', to: [], cc: [], date: '2026-08-07T10:00:00.000Z', size: 1, read: true,
        flagged: false, hasAttachments: false, answered: false, forwarded: false, snippet: '',
        tagColours: [],
    } as Message
}

const list = ['a', 'b', 'c', 'd'].map(makeMessage)
const ids = (ms: readonly Message[]) => ms.map((m) => m.id)

describe('takeOut', () => {
    it('removes the named messages and anchors each to the row before it that stayed', () => {
        const {next, lifted} = takeOut(list, new Set(['b', 'd']))
        expect(ids(next)).toEqual(['a', 'c'])
        expect(lifted.map((l) => [l.message.id, l.afterId, l.index])).toEqual([['b', 'a', 1], ['d', 'c', 3]])
    })

    it('anchors a run taken from the head to nothing', () => {
        const {lifted} = takeOut(list, new Set(['a', 'b']))
        expect(lifted.map((l) => l.afterId)).toEqual(['', ''])
    })

    it('takes nothing for an empty id set and leaves the input alone', () => {
        const {next, lifted} = takeOut(list, new Set())
        expect(ids(next)).toEqual(['a', 'b', 'c', 'd'])
        expect(lifted).toEqual([])
        expect(ids(list)).toEqual(['a', 'b', 'c', 'd'])
    })
})

describe('putBack', () => {
    it('returns each message to the gap it came out of', () => {
        const {next, lifted} = takeOut(list, new Set(['b', 'd']))
        expect(ids(putBack(next, lifted))).toEqual(['a', 'b', 'c', 'd'])
    })

    it('restores in original order however the lifted entries are ordered', () => {
        const {next, lifted} = takeOut(list, new Set(['a', 'c']))
        expect(ids(putBack(next, [...lifted].reverse()))).toEqual(['a', 'b', 'c', 'd'])
    })

    it('keeps the order of adjacent rows sharing one anchor', () => {
        const {next, lifted} = takeOut(list, new Set(['b', 'c']))
        expect(ids(putBack(next, lifted))).toEqual(['a', 'b', 'c', 'd'])
    })

    it('restores a head run to the head, in order', () => {
        const {next, lifted} = takeOut(list, new Set(['a', 'b']))
        expect(ids(putBack(next, lifted))).toEqual(['a', 'b', 'c', 'd'])
    })

    it('puts back only the subset asked for, past a sibling that stayed removed', () => {
        // The partial-move case: a and b left together, the server moved only a, so only b comes back.
        const {next, lifted} = takeOut(list, new Set(['a', 'b']))
        const forB = lifted.filter((l) => l.message.id === 'b')
        expect(ids(putBack(next, forB))).toEqual(['b', 'c', 'd'])
    })

    it('skips a message the list already carries again', () => {
        const {lifted} = takeOut(list, new Set(['b']))
        expect(ids(putBack(list, lifted))).toEqual(['a', 'b', 'c', 'd'])
    })

    it('falls back to the remembered index when the anchor has itself gone', () => {
        const {lifted} = takeOut(list, new Set(['d']))
        expect(ids(putBack([makeMessage('a'), makeMessage('b')], lifted))).toEqual(['a', 'b', 'd'])
    })

    it('leaves the input list unchanged', () => {
        const {next, lifted} = takeOut(list, new Set(['b']))
        putBack(next, lifted)
        expect(ids(next)).toEqual(['a', 'c', 'd'])
    })
})
