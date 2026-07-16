// The launcher's cross-account resolution: a reply, reply-all or forward opened from a unified-mailbox
// row (a message carrying its own accountId) must compose as that account, not the selected one: its
// From address, its signature and the ComposeInitial.accountId the modal sends from. Rows without an
// accountId (every per-folder listing) keep composing as the selected account. ../api is mocked (the
// Wails seam); the launch-time draft-recovery check resolves to nothing.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Account, Message} from '../api'
import {MailtoFields, mailtoComposeInitial, useComposeLauncher} from './useComposeLauncher'

const apiSpies = vi.hoisted(() => ({
    draftRecovery: vi.fn(),
    clearDraftRecovery: vi.fn(),
}))

// The launcher subscribes to the backend's mailto:open events through the Wails runtime; capture the
// handlers so tests can drive the events, mirroring App.test.tsx.
const runtimeSpies = vi.hoisted(() => ({
    EventsOn: vi.fn(() => () => undefined),
}))

vi.mock('../../wailsjs/runtime', () => ({
    EventsOn: runtimeSpies.EventsOn,
}))

vi.mock('../api', () => ({
    api: {
        draftRecovery: apiSpies.draftRecovery,
        clearDraftRecovery: apiSpies.clearDraftRecovery,
    },
}))

function makeAccount(id: string, signature: string): Account {
    return {id, displayName: id, email: id, protocol: 'imap', signature, identities: []} as unknown as Account
}

function makeMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1', folderId: 'inbox', accountId: '', subject: 'Weekly report',
        fromName: 'Alice Example', fromAddress: 'alice@example.com',
        to: [{name: '', address: 'me@one.com'}], cc: [],
        date: '2026-07-11T10:00:00.000Z', size: 1024, read: false, flagged: false,
        hasAttachments: false, answered: false, forwarded: false, snippet: 'A short snippet', tagColours: [],
        ...overrides,
    } as Message
}

const ACCOUNTS = [makeAccount('me@one.com', '<p>Sig one</p>'), makeAccount('me@two.com', '<p>Sig two</p>')]

function harness() {
    return renderHook(() => useComposeLauncher({
        accounts: ACCOUNTS,
        selectedAccount: 'me@one.com',
        setSelectedAccount: () => {},
        messageBody: null,
        setError: () => {},
    }))
}

beforeEach(() => {
    apiSpies.draftRecovery.mockReset().mockResolvedValue({present: false})
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
    runtimeSpies.EventsOn.mockClear()
})
afterEach(() => cleanup())

describe('useComposeLauncher: compose account resolution', () => {
    it('replies from the row\'s own account when the message carries one', () => {
        const {result} = harness()
        const message = makeMessage({accountId: 'me@two.com', to: [{name: '', address: 'me@two.com'}]})
        act(() => result.current.openReply(message))
        expect(result.current.composing).toBe(true)
        expect(result.current.composeInitial?.accountId).toBe('me@two.com')
        expect(result.current.composeInitial?.from).toBe('me@two.com')
        expect(result.current.composeInitial?.bodyHtml).toContain('Sig two')
    })

    it('falls back to the selected account for a per-folder row', () => {
        const {result} = harness()
        act(() => result.current.openReply(makeMessage()))
        expect(result.current.composeInitial?.accountId).toBe('me@one.com')
        expect(result.current.composeInitial?.from).toBe('me@one.com')
        expect(result.current.composeInitial?.bodyHtml).toContain('Sig one')
    })

    it('forwards with the row account\'s signature and accountId', () => {
        const {result} = harness()
        act(() => result.current.openForward(makeMessage({accountId: 'me@two.com'})))
        expect(result.current.composeInitial?.accountId).toBe('me@two.com')
        expect(result.current.composeInitial?.bodyHtml).toContain('Sig two')
        expect(result.current.composeInitial?.replyKind).toBe('forward')
    })

    it('reply-all drops the row account\'s own address from the recipients', () => {
        const {result} = harness()
        const message = makeMessage({
            accountId: 'me@two.com',
            to: [{name: '', address: 'me@two.com'}, {name: '', address: 'colleague@x.com'}],
        })
        act(() => result.current.openReplyAll(message))
        expect(result.current.composeInitial?.accountId).toBe('me@two.com')
        expect(result.current.composeInitial?.to).toContain('alice@example.com')
        expect(result.current.composeInitial?.to).toContain('colleague@x.com')
        expect(result.current.composeInitial?.to).not.toContain('me@two.com')
    })
})

describe('useComposeLauncher: mailto', () => {
    function capturedHandler(event: string): (arg: unknown) => void {
        const calls = runtimeSpies.EventsOn.mock.calls as unknown as
            [string, (arg: unknown) => void][]
        const call = calls.find(([name]) => name === event)
        expect(call, `handler for ${event}`).toBeDefined()
        return (call as [string, (arg: unknown) => void])[1]
    }

    it('opens the composer pre-filled from a mailto:open event, with the signature', () => {
        const {result} = harness()
        const fields: MailtoFields = {
            to: ['jane@example.org', 'joe@example.org'], cc: ['ann@example.org'], bcc: null,
            subject: 'Chess move', body: 'Knight takes.\r\nYour turn.',
        }
        act(() => capturedHandler('mailto:open')(fields))
        expect(result.current.composing).toBe(true)
        expect(result.current.composeInitial?.to).toBe('jane@example.org, joe@example.org')
        expect(result.current.composeInitial?.cc).toBe('ann@example.org')
        expect(result.current.composeInitial?.bcc).toBe('')
        expect(result.current.composeInitial?.subject).toBe('Chess move')
        expect(result.current.composeInitial?.bodyHtml).toBe(
            '<p>Knight takes.<br>Your turn.</p><p>Sig one</p>',
        )
    })

    it('reports a mailto parse failure through the error sink', () => {
        const errors: string[] = []
        renderHook(() => useComposeLauncher({
            accounts: ACCOUNTS,
            selectedAccount: 'me@one.com',
            setSelectedAccount: () => {},
            messageBody: null,
            setError: (message) => errors.push(message),
        }))
        act(() => capturedHandler('mailto:open-error')('bad uri'))
        expect(errors).toEqual(['bad uri'])
    })
})

describe('mailtoComposeInitial', () => {
    it('escapes markup in the body and keeps an empty body as one paragraph', () => {
        const evil = mailtoComposeInitial(
            {to: null, cc: null, bcc: null, subject: '', body: '<b>&</b>'}, '',
        )
        expect(evil.bodyHtml).toBe('<p>&lt;b&gt;&amp;&lt;/b&gt;</p>')
        const empty = mailtoComposeInitial(
            {to: null, cc: null, bcc: null, subject: '', body: ''}, '<p>Sig</p>',
        )
        expect(empty.bodyHtml).toBe('<p></p><p>Sig</p>')
        expect(empty.to).toBe('')
    })
})
