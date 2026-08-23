import {describe, expect, it} from 'vitest'
import {buildDraftEdit} from './draftEdit'
import type {Message, MessageBody} from './api'

// makeMessage builds the fields buildDraftEdit reads off a stored draft's row; everything else on the
// row is irrelevant to it, so the cast keeps the fixture to what is under test.
function makeMessage(over: Partial<Message> = {}): Message {
    return {
        id: 'draft1',
        subject: 'Half written',
        to: [{name: 'Ann', address: 'ann@example.com'}],
        cc: [],
        ...over,
    } as Message
}

function makeBody(over: Partial<MessageBody> = {}): MessageBody {
    return {plain: '', html: '', hasInvite: false, attachments: [], ...over} as MessageBody
}

describe('buildDraftEdit', () => {
    it('carries the recipients, the subject and the stored HTML body', () => {
        const draft = buildDraftEdit(
            makeMessage({cc: [{name: 'Bob', address: 'bob@example.com'}]}),
            makeBody({html: '<p>Nearly done</p>'}),
        )
        expect(draft).toEqual({
            to: 'ann@example.com',
            cc: 'bob@example.com',
            subject: 'Half written',
            bodyHtml: '<p>Nearly done</p>',
        })
    })

    it('drops display names and joins several recipients', () => {
        const draft = buildDraftEdit(makeMessage({
            to: [{name: 'Ann, of Accounts', address: ' ann@example.com '}, {name: '', address: 'cat@example.com'}],
        }), makeBody())
        expect(draft.to).toBe('ann@example.com, cat@example.com')
    })

    it('skips an entry with no address', () => {
        const draft = buildDraftEdit(makeMessage({to: [{name: 'Nobody', address: '  '}]}), makeBody())
        expect(draft.to).toBe('')
    })

    it('treats a null recipient list as empty', () => {
        // Go serialises an empty address slice as null, so both lists arrive nulled on a draft with none.
        const draft = buildDraftEdit(
            makeMessage({to: null as unknown as Message['to'], cc: null as unknown as Message['cc']}),
            makeBody(),
        )
        expect(draft.to).toBe('')
        expect(draft.cc).toBe('')
    })

    it('falls back to the plain text when the draft has no HTML alternative', () => {
        const draft = buildDraftEdit(makeMessage(), makeBody({plain: 'first\r\nsecond\nthird'}))
        expect(draft.bodyHtml).toBe('<p>first<br>second<br>third</p>')
    })

    it('escapes the plain text it wraps', () => {
        const draft = buildDraftEdit(makeMessage(), makeBody({plain: '<b>5 & 6</b>'}))
        expect(draft.bodyHtml).toBe('<p>&lt;b&gt;5 &amp; 6&lt;/b&gt;</p>')
    })

    it('ignores a whitespace-only HTML body', () => {
        const draft = buildDraftEdit(makeMessage(), makeBody({html: '   ', plain: 'text'}))
        expect(draft.bodyHtml).toBe('<p>text</p>')
    })

    it('gives an empty body for an empty draft and for one with no body at all', () => {
        expect(buildDraftEdit(makeMessage(), makeBody({plain: '  '})).bodyHtml).toBe('')
        expect(buildDraftEdit(makeMessage(), null).bodyHtml).toBe('')
    })
})
