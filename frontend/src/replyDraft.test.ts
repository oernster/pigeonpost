import {describe, it, expect} from 'vitest'
import {
    buildForward, buildReply, buildReplyAll, quoteFor, replyFromAddress, sendersFor, signatureHtmlFor,
} from './replyDraft'
import type {Account, Message, MessageBody} from './api'

function msg(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1', folderId: 'inbox', subject: 'Hello', fromName: 'Alice', fromAddress: 'alice@example.com',
        to: [], cc: [], date: '2026-07-11T10:00:00.000Z', size: 0, read: false, flagged: false,
        hasAttachments: false, snippet: 'snip', tagColours: [],
        ...overrides,
    } as Message
}

function acc(overrides: Partial<Account> = {}): Account {
    return {id: 'a1', displayName: 'Me', email: 'me@example.com', signature: '', identities: [], ...overrides} as Account
}

const ctx = {from: 'me@example.com', signatureHtml: '<p>sig</p>', quotedHtml: '<div>q</div>'}

describe('sendersFor', () => {
    it('lists the primary address then identities', () => {
        expect(sendersFor(acc({identities: [{name: 'Alias', address: 'alias@example.com'}]}))).toEqual([
            {name: 'Me', address: 'me@example.com'},
            {name: 'Alias', address: 'alias@example.com'},
        ])
    })
    it('is empty for no account', () => {
        expect(sendersFor(undefined)).toEqual([])
    })
})

describe('signatureHtmlFor', () => {
    it('returns the account signature', () => {
        expect(signatureHtmlFor(acc({signature: '<p>sig</p>'}))).toBe('<p>sig</p>')
    })
    it('is empty for no account', () => {
        expect(signatureHtmlFor(undefined)).toBe('')
    })
})

describe('quoteFor', () => {
    it('uses the HTML body when present', () => {
        expect(quoteFor(msg(), {plain: 'p', html: '<div>rich</div>', hasInvite: false, attachments: []} as MessageBody))
            .toBe('<div>rich</div>')
    })
    it('escapes the plain text when the HTML is blank', () => {
        expect(quoteFor(msg(), {plain: 'a & b', html: '   ', hasInvite: false, attachments: []} as MessageBody))
            .toBe('<p>a &amp; b</p>')
    })
    it('uses the snippet with no body, and empties to a bare paragraph', () => {
        expect(quoteFor(msg({snippet: 'snip'}), null)).toBe('<p>snip</p>')
        expect(quoteFor(msg({snippet: ''}), null)).toBe('<p></p>')
    })
})

describe('replyFromAddress', () => {
    const senders = [{address: 'me@example.com'}, {address: 'alias@example.com'}]
    it('returns the delivered-to alias found in To', () => {
        expect(replyFromAddress(msg({to: [{name: '', address: 'ALIAS@example.com'}]}), senders)).toBe('ALIAS@example.com')
    })
    it('finds the address in Cc too', () => {
        expect(replyFromAddress(msg({to: [], cc: [{name: '', address: 'me@example.com'}]}), senders)).toBe('me@example.com')
    })
    it('is empty when none of our addresses received it', () => {
        expect(replyFromAddress(msg({to: [{name: '', address: 'other@example.com'}]}), senders)).toBe('')
    })
    it('tolerates a message with no To or Cc arrays', () => {
        expect(replyFromAddress({...msg(), to: undefined, cc: undefined} as unknown as Message, senders)).toBe('')
    })
})

describe('buildReply', () => {
    it('addresses the sender with a Re: subject and quotes the original', () => {
        const d = buildReply(msg({fromName: 'Alice', fromAddress: 'alice@example.com', subject: 'Hi'}), ctx)
        expect(d.to).toBe('alice@example.com')
        expect(d.from).toBe('me@example.com')
        expect(d.subject).toBe('Re: Hi')
        expect(d.bodyHtml).toContain('<p>sig</p>')
        expect(d.bodyHtml).toContain('<blockquote><div>q</div></blockquote>')
        expect(d.bodyHtml).toContain('Alice wrote:')
    })
    it('uses the from address in the attribution when there is no display name', () => {
        expect(buildReply(msg({fromName: '', fromAddress: 'bob@example.com'}), ctx).bodyHtml).toContain('bob@example.com wrote:')
    })
    it('omits the date and falls back to "the sender" with neither name nor address', () => {
        const d = buildReply(msg({date: '', fromName: '', fromAddress: ''}), ctx)
        expect(d.bodyHtml).toContain('the sender wrote:')
        expect(d.bodyHtml).not.toContain('On ')
    })
})

describe('buildReplyAll', () => {
    it('collects sender, To and Cc, dropping self, duplicates and blanks', () => {
        const m = msg({
            fromAddress: 'alice@example.com',
            to: [
                {name: '', address: 'me@example.com'},
                {name: '', address: ''},
                {name: '', address: 'bob@example.com'},
                {name: '', address: 'bob@example.com'},
            ],
            cc: [{name: '', address: 'carol@example.com'}],
        })
        const d = buildReplyAll(m, 'me@example.com', ctx)
        expect(d.to).toBe('alice@example.com, bob@example.com')
        expect(d.cc).toBe('carol@example.com')
    })
    it('handles a message with no To or Cc arrays', () => {
        const m = {...msg({fromAddress: 'alice@example.com'}), to: undefined, cc: undefined} as unknown as Message
        const d = buildReplyAll(m, 'me@example.com', ctx)
        expect(d.to).toBe('alice@example.com')
        expect(d.cc).toBe('')
    })
})

describe('buildForward', () => {
    const fctx = {signatureHtml: '<p>sig</p>', quotedHtml: '<div>q</div>'}
    it('builds a Fwd: with the forwarded-message header', () => {
        const d = buildForward(msg({fromName: 'Alice', subject: 'Report'}), fctx)
        expect(d.to).toBe('')
        expect(d.subject).toBe('Fwd: Report')
        expect(d.bodyHtml).toContain('Forwarded message')
        expect(d.bodyHtml).toContain('From: Alice')
    })
    it('uses the from address when there is no display name', () => {
        expect(buildForward(msg({fromName: '', fromAddress: 'bob@example.com'}), fctx).bodyHtml).toContain('From: bob@example.com')
    })
    it('shows fallbacks when the message has no sender or subject', () => {
        const d = buildForward(msg({subject: '', fromName: '', fromAddress: ''}), fctx)
        expect(d.bodyHtml).toContain('Subject: (no subject)')
        expect(d.bodyHtml).toContain('From: unknown sender')
    })
})
