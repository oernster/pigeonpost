// The launcher's cross-account resolution: a reply, reply-all or forward opened from a unified-mailbox
// row (a message carrying its own accountId) must compose as that account, not the selected one: its
// From address, its signature and the ComposeInitial.accountId the modal sends from. Rows without an
// accountId (every per-folder listing) keep composing as the selected account. ../api is mocked (the
// Wails seam); the launch-time draft-recovery check resolves to nothing.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Account, Message} from '../api'
import {useComposeLauncher} from './useComposeLauncher'

const apiSpies = vi.hoisted(() => ({
    draftRecovery: vi.fn(),
    clearDraftRecovery: vi.fn(),
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
