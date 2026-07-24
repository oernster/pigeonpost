// Automatic contact collection at send time: recipients go to the address book after a successful
// send, minus the sender's own addresses, gated by the persisted setting (on by default).
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {ComposeModal} from './ComposeModal'
import {AUTO_COLLECT_KEY} from '../autoCollect'

const apiSpies = vi.hoisted(() => ({
    send: vi.fn(),
    saveDraft: vi.fn(),
    clearDraftRecovery: vi.fn(),
    saveDraftRecovery: vi.fn(),
    pickAttachments: vi.fn(),
    listContacts: vi.fn(),
    collectContacts: vi.fn(),
}))

vi.mock('../api', () => ({
    api: {
        send: apiSpies.send,
        saveDraft: apiSpies.saveDraft,
        clearDraftRecovery: apiSpies.clearDraftRecovery,
        saveDraftRecovery: apiSpies.saveDraftRecovery,
        pickAttachments: apiSpies.pickAttachments,
        listContacts: apiSpies.listContacts,
        collectContacts: apiSpies.collectContacts,
    },
}))

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
