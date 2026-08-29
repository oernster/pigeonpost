// Automatic contact collection at send time: recipients go to the address book after a successful
// send, minus the sender's own addresses, gated by the persisted setting (on by default).
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {ComposeModal} from './ComposeModal'
import {AUTO_COLLECT_KEY} from '../autoCollect'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    send: vi.fn(),
    saveDraft: vi.fn(),
    clearDraftRecovery: vi.fn(),
    saveDraftRecovery: vi.fn(),
    pickAttachments: vi.fn(),
    listContacts: vi.fn(),
    collectContacts: vi.fn(),
}))

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy
// fails the test by name instead of throwing a TypeError into the nearest catch and passing. The
// afterEach below reports any that were reached. See src/test/apiMock.ts.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

vi.mock('@tiptap/react', () => ({
    useEditor: () => ({
        isActive: () => false,
        getText: () => 'hello',
        getHTML: () => '<p>hello</p>',
        getAttributes: () => ({}),
        chain: () => {
            const c: Record<string, () => unknown> = {}
            for (const m of ['focus', 'run', 'setImage']) {
                c[m] = () => c
            }
            return c
        },
    }),
    EditorContent: () => null,
}))

const TO_PLACEHOLDER = 'name@example.com, other@example.com'

function renderCompose() {
    render(<ComposeModal
        accountId="acc1"
        senders={[{name: 'Me', address: 'me@mine.example'}]}
        canSaveDraft={true}
        holdSeconds={0}
        onHeld={vi.fn()}
        onClose={vi.fn()}
        onMarkReplied={vi.fn()}
        onMarkForwarded={vi.fn()}
        onDraftSuperseded={vi.fn()}
    />)
}

async function sendTo(recipients: string) {
    fireEvent.change(screen.getByPlaceholderText(TO_PLACEHOLDER), {target: {value: recipients}})
    fireEvent.click(screen.getByRole('button', {name: 'Send'}))
    await waitFor(() => expect(apiSpies.send).toHaveBeenCalled())
}

beforeEach(() => {
    apiSpies.send.mockReset().mockResolvedValue('')
    apiSpies.saveDraft.mockReset().mockResolvedValue(undefined)
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.saveDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.pickAttachments.mockReset().mockResolvedValue([])
    apiSpies.listContacts.mockReset().mockResolvedValue([])
    apiSpies.collectContacts.mockReset().mockResolvedValue(1)
    window.localStorage.removeItem(AUTO_COLLECT_KEY)
})

afterEach(() => {
    window.localStorage.removeItem(AUTO_COLLECT_KEY)
    cleanup()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('ComposeModal: automatic contact collection', () => {
    it('collects recipients after a successful send, dropping the sender\'s own address', async () => {
        renderCompose()
        await sendTo('new@person.example, me@mine.example')
        expect(apiSpies.collectContacts).toHaveBeenCalledWith(['new@person.example'])
    })

    it('does not collect when the setting is off', async () => {
        window.localStorage.setItem(AUTO_COLLECT_KEY, '0')
        renderCompose()
        await sendTo('new@person.example')
        expect(apiSpies.collectContacts).not.toHaveBeenCalled()
    })

    it('does not collect when the send fails', async () => {
        apiSpies.send.mockRejectedValue(new Error('smtp down'))
        renderCompose()
        await sendTo('new@person.example')
        expect(apiSpies.collectContacts).not.toHaveBeenCalled()
    })

    it('never lets a collection failure disturb the send', async () => {
        apiSpies.collectContacts.mockRejectedValue(new Error('store broke'))
        renderCompose()
        await sendTo('new@person.example')
        // The send path completed: the draft-recovery slot was cleared as on every successful send.
        await waitFor(() => expect(apiSpies.clearDraftRecovery).toHaveBeenCalled())
    })
})

// The mock covers the api in both directions: the afterEach above catches a method reached with no
// spy; this catches the opposite, a spy declared under a name the api does not have, which binds to
// nothing, so every test configuring it would be configuring a stub the code can never call.
describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
