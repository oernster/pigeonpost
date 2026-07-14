// Characterization test for the compose window at its stable outer interface (accountId, senders, initial,
// canSaveDraft, onClose). It renders the real modal and drives each flow, asserting the DOM plus which api
// call fired with what. The interface it pins does not move as the modal is decomposed in Phase 2 (the link
// editor moves to the shared useLinkEditor, the debounced draft autosave to useDraftAutosave and the
// separator-correction slice out), so this suite staying green is the proof each extraction preserved
// behaviour. ../api is stubbed (the Wails seam) and @tiptap/react too, since ProseMirror does not run in
// jsdom; the editor stub reports an empty body, which is enough for these flows.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import type {ComponentProps} from 'react'
import {ComposeModal} from './ComposeModal'

const apiSpies = vi.hoisted(() => ({
    send: vi.fn(),
    saveDraft: vi.fn(),
    clearDraftRecovery: vi.fn(),
    saveDraftRecovery: vi.fn(),
    pickAttachments: vi.fn(),
}))

vi.mock('../api', () => ({
    api: {
        send: apiSpies.send,
        saveDraft: apiSpies.saveDraft,
        clearDraftRecovery: apiSpies.clearDraftRecovery,
        saveDraftRecovery: apiSpies.saveDraftRecovery,
        pickAttachments: apiSpies.pickAttachments,
    },
}))

vi.mock('@tiptap/react', () => {
    const chain = () => {
        const c: Record<string, () => unknown> = {}
        for (const m of [
            'focus', 'toggleBold', 'toggleItalic', 'toggleStrike', 'toggleHeading', 'toggleBulletList',
            'toggleOrderedList', 'toggleBlockquote', 'extendMarkRange', 'setLink', 'unsetLink', 'run',
        ]) {
            c[m] = () => c
        }
        return c
    }
    const editor = {
        isActive: () => false,
        getText: () => '',
        getHTML: () => '<p></p>',
        getAttributes: () => ({}),
        chain,
    }
    return {useEditor: () => editor, EditorContent: () => null}
})

type ComposeProps = ComponentProps<typeof ComposeModal>

const TO_PLACEHOLDER = 'name@example.com, other@example.com'

function renderCompose(overrides: Partial<ComposeProps> = {}) {
    const onClose = vi.fn()
    const onMarkReplied = vi.fn()
    const onMarkForwarded = vi.fn()
    const onHeld = vi.fn()
    const props: ComposeProps = {
        accountId: 'acc1',
        senders: [{name: 'Me', address: 'me@x.com'}],
        canSaveDraft: true,
        holdSeconds: 0,
        onHeld,
        onClose,
        onMarkReplied,
        onMarkForwarded,
        ...overrides,
    }
    const view = render(<ComposeModal {...props}/>)
    const toInput = () => screen.getByPlaceholderText(TO_PLACEHOLDER)
    return {...view, onClose, onMarkReplied, onMarkForwarded, onHeld, toInput}
}

beforeEach(() => {
    apiSpies.send.mockReset().mockResolvedValue('')
    apiSpies.saveDraft.mockReset().mockResolvedValue(undefined)
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.saveDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.pickAttachments.mockReset().mockResolvedValue([])
})

afterEach(() => cleanup())

describe('ComposeModal: basics', () => {
    it('renders the dialog and the recipient fields', () => {
        renderCompose()
        expect(screen.getByRole('dialog', {name: 'New message'})).toBeInTheDocument()
        expect(screen.getByText('To')).toBeInTheDocument()
        expect(screen.getByText('Subject')).toBeInTheDocument()
    })

    it('prefills from the initial values', () => {
        renderCompose({initial: {to: 'x@y.com', subject: 'Hello'}})
        expect(screen.getByPlaceholderText(TO_PLACEHOLDER)).toHaveValue('x@y.com')
        expect(screen.getByDisplayValue('Hello')).toBeInTheDocument()
    })

    it('shows the From dropdown only with more than one sender', () => {
        const {rerender} = renderCompose()
        expect(screen.queryByText('From')).toBeNull()
        rerender(
            <ComposeModal
                accountId="acc1"
                senders={[{name: 'Me', address: 'me@x.com'}, {name: 'Alias', address: 'alias@x.com'}]}
                canSaveDraft
                holdSeconds={0}
                onHeld={vi.fn()}
                onMarkReplied={vi.fn()}
                onMarkForwarded={vi.fn()}
                onClose={vi.fn()}
            />,
        )
        expect(screen.getByText('From')).toBeInTheDocument()
    })
})

describe('ComposeModal: send', () => {
    it('sends the built request, clears recovery and closes', async () => {
        const {onClose} = renderCompose({initial: {to: 'x@y.com', subject: 'Hi'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(apiSpies.send).toHaveBeenCalledWith(expect.objectContaining({
            accountId: 'acc1', from: 'me@x.com', to: ['x@y.com'], subject: 'Hi',
        }))
        expect(apiSpies.clearDraftRecovery).toHaveBeenCalled()
    })

    it('marks the original replied after sending a reply', async () => {
        const {onClose, onMarkReplied, onMarkForwarded} = renderCompose({initial: {to: 'x@y.com', inReplyToId: 'orig1', replyKind: 'reply'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(onMarkReplied).toHaveBeenCalledWith('orig1')
        expect(onMarkForwarded).not.toHaveBeenCalled()
    })

    it('marks the original forwarded after sending a forward', async () => {
        const {onClose, onMarkReplied, onMarkForwarded} = renderCompose({initial: {to: 'x@y.com', inReplyToId: 'orig2', replyKind: 'forward'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(onMarkForwarded).toHaveBeenCalledWith('orig2')
        expect(onMarkReplied).not.toHaveBeenCalled()
    })

    it('marks nothing after sending a fresh compose', async () => {
        const {onClose, onMarkReplied, onMarkForwarded} = renderCompose({initial: {to: 'x@y.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(onMarkReplied).not.toHaveBeenCalled()
        expect(onMarkForwarded).not.toHaveBeenCalled()
    })

    it('reports a held send with the queued id and the compose state, deferring the reply mark', async () => {
        apiSpies.send.mockResolvedValue('ob-1')
        const {onClose, onHeld, onMarkReplied} = renderCompose({
            holdSeconds: 10,
            initial: {to: 'x@y.com', subject: 'Hi', inReplyToId: 'orig1', replyKind: 'reply'},
        })
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(apiSpies.send).toHaveBeenCalledWith(expect.objectContaining({holdSeconds: 10}))
        expect(onHeld).toHaveBeenCalledWith('ob-1', expect.objectContaining({
            to: 'x@y.com', subject: 'Hi', inReplyToId: 'orig1', replyKind: 'reply',
        }))
        // The reply mark is deferred to the undo window's expiry: an undone send must not flag the original.
        expect(onMarkReplied).not.toHaveBeenCalled()
    })

    it('keeps Send disabled until there is a recipient', () => {
        renderCompose()
        expect(screen.getByRole('button', {name: 'Send'})).toBeDisabled()
    })

    it('surfaces a send error and stays open', async () => {
        apiSpies.send.mockRejectedValueOnce('smtp unreachable')
        const {onClose} = renderCompose({initial: {to: 'x@y.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        expect(await screen.findByText('smtp unreachable')).toBeInTheDocument()
        expect(onClose).not.toHaveBeenCalled()
    })

    it('sends on Ctrl+Enter from a recipient field', async () => {
        const {toInput} = renderCompose({initial: {to: 'x@y.com'}})
        fireEvent.keyDown(toInput(), {key: 'Enter', ctrlKey: true})
        await waitFor(() => expect(apiSpies.send).toHaveBeenCalled())
    })
})

describe('ComposeModal: save draft', () => {
    it('saves a draft when the account supports it', async () => {
        const {onClose} = renderCompose({initial: {to: 'x@y.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Save draft'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(apiSpies.saveDraft).toHaveBeenCalledWith(expect.objectContaining({to: ['x@y.com']}))
        expect(apiSpies.clearDraftRecovery).toHaveBeenCalled()
    })

    it('hides Save draft for an account that cannot save drafts', () => {
        renderCompose({canSaveDraft: false})
        expect(screen.queryByRole('button', {name: 'Save draft'})).toBeNull()
    })
})

describe('ComposeModal: attachments', () => {
    it('attaches picked files and removes a chip', async () => {
        apiSpies.pickAttachments.mockResolvedValueOnce(['C:\\Users\\report.pdf'])
        renderCompose()
        fireEvent.click(screen.getByRole('button', {name: 'Attach files'}))
        expect(await screen.findByText('report.pdf')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Remove report.pdf'}))
        expect(screen.queryByText('report.pdf')).toBeNull()
    })

    it('renders and removes an attached message', () => {
        renderCompose({initial: {messageAttachments: [{id: 'm1', name: 'Forwarded.eml'}]}})
        expect(screen.getByText('Forwarded.eml')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Remove Forwarded.eml'}))
        expect(screen.queryByText('Forwarded.eml')).toBeNull()
    })
})

describe('ComposeModal: attachment reminder', () => {
    it('warns when the message mentions an attachment but none is attached', async () => {
        const {onClose} = renderCompose({initial: {to: 'x@y.com', subject: 'see attached'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        expect(screen.getByRole('alertdialog', {name: 'Attachment reminder'})).toBeInTheDocument()
        expect(apiSpies.send).not.toHaveBeenCalled()
        fireEvent.click(screen.getByRole('button', {name: 'Send anyway'}))
        await waitFor(() => expect(apiSpies.send).toHaveBeenCalled())
        expect(onClose).toHaveBeenCalled()
    })

    it('does not warn when something is already attached', async () => {
        renderCompose({initial: {to: 'x@y.com', subject: 'see attached', attachmentPaths: ['/tmp/a.pdf']}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(apiSpies.send).toHaveBeenCalled())
        expect(screen.queryByRole('alertdialog', {name: 'Attachment reminder'})).toBeNull()
    })
})

describe('ComposeModal: separator correction', () => {
    it('offers to fix a wrong separator and applies it without sending', () => {
        const {toInput} = renderCompose({initial: {to: 'a@x.com b@y.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        expect(screen.getByText(/Addresses should be separated/)).toBeInTheDocument()
        expect(apiSpies.send).not.toHaveBeenCalled()
        fireEvent.click(screen.getByRole('button', {name: 'Use this'}))
        expect(toInput()).toHaveValue('a@x.com; b@y.com')
        expect(screen.queryByText(/Addresses should be separated/)).toBeNull()
    })

    it('dismisses the correction', () => {
        renderCompose({initial: {to: 'a@x.com b@y.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        fireEvent.click(screen.getByRole('button', {name: 'Dismiss'}))
        expect(screen.queryByText(/Addresses should be separated/)).toBeNull()
    })
})

describe('ComposeModal: link editor', () => {
    it('opens the link row with Apply and Remove, and closes on apply', () => {
        renderCompose()
        expect(screen.queryByPlaceholderText('https://example.com')).toBeNull()
        fireEvent.click(screen.getByRole('button', {name: 'Link'}))
        expect(screen.getByPlaceholderText('https://example.com')).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Remove'})).toBeInTheDocument()
        fireEvent.change(screen.getByPlaceholderText('https://example.com'), {target: {value: 'example.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Apply'}))
        expect(screen.queryByPlaceholderText('https://example.com')).toBeNull()
    })
})

describe('ComposeModal: draft autosave', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('writes a recovery snapshot a short pause after an edit', () => {
        const {toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'x@y.com'}})
        act(() => vi.advanceTimersByTime(1500))
        expect(apiSpies.saveDraftRecovery).toHaveBeenCalledWith({
            accountId: 'acc1', to: 'x@y.com', cc: '', bcc: '', subject: '', bodyHtml: '<p></p>',
        })
    })

    it('clears the recovery slot once the compose is emptied back out', () => {
        const {toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'x@y.com'}})
        fireEvent.change(toInput(), {target: {value: ''}})
        act(() => vi.advanceTimersByTime(1500))
        expect(apiSpies.clearDraftRecovery).toHaveBeenCalled()
        expect(apiSpies.saveDraftRecovery).not.toHaveBeenCalled()
    })
})

describe('ComposeModal: send later', () => {
    const MS_PER_HOUR = 60 * 60 * 1000

    it('schedules a preset moment and closes without the undo toast or the reply mark', async () => {
        apiSpies.send.mockResolvedValue('ob-9')
        const {onClose, onHeld, onMarkReplied} = renderCompose({
            holdSeconds: 10,
            initial: {to: 'x@y.com', subject: 'Hi', inReplyToId: 'orig1', replyKind: 'reply'},
        })
        fireEvent.click(screen.getByRole('button', {name: 'Send later'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Tomorrow morning (09:00)'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(apiSpies.send.mock.calls[0][0].sendAtMs).toBeGreaterThan(Date.now())
        // A scheduled send waits in the Outbox: no undo toast, and the reply mark stays honest (the
        // schedule may yet be cancelled), so neither callback fires.
        expect(onHeld).not.toHaveBeenCalled()
        expect(onMarkReplied).not.toHaveBeenCalled()
    })

    it('schedules a custom future moment from the date-time field', async () => {
        renderCompose({initial: {to: 'x@y.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send later'}))
        const scheduleButton = screen.getByRole('button', {name: 'Schedule'})
        expect(scheduleButton).toBeDisabled()
        const {toDatetimeLocal} = await import('../schedule')
        fireEvent.change(screen.getByLabelText('Send at'), {
            target: {value: toDatetimeLocal(new Date(Date.now() + MS_PER_HOUR))},
        })
        expect(scheduleButton).toBeEnabled()
        fireEvent.click(scheduleButton)
        await waitFor(() => expect(apiSpies.send).toHaveBeenCalled())
        expect(apiSpies.send.mock.calls[0][0].sendAtMs).toBeGreaterThan(Date.now())
    })

    it('keeps a past custom moment unschedulable', () => {
        renderCompose({initial: {to: 'x@y.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send later'}))
        fireEvent.change(screen.getByLabelText('Send at'), {target: {value: '2000-01-01T00:00'}})
        expect(screen.getByRole('button', {name: 'Schedule'})).toBeDisabled()
    })

    it('is unavailable without a recipient', () => {
        renderCompose()
        expect(screen.getByRole('button', {name: 'Send later'})).toBeDisabled()
    })
})
