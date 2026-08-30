// Characterisation test for the compose window at its stable outer interface (accountId, senders, initial,
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

const editorSpies = vi.hoisted(() => ({
    options: undefined as {autofocus?: string} | undefined,
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
    return {
        useEditor: (options: {autofocus?: string}) => {
            editorSpies.options = options
            return editor
        },
        EditorContent: () => null,
    }
})

type ComposeProps = ComponentProps<typeof ComposeModal>

const TO_PLACEHOLDER = 'name@example.com, other@example.com'

function renderCompose(overrides: Partial<ComposeProps> = {}) {
    const onClose = vi.fn()
    const onMarkReplied = vi.fn()
    const onMarkForwarded = vi.fn()
    const props: ComposeProps = {
        accountId: 'acc1',
        senders: [{name: 'Me', address: 'me@x.com'}],
        canSaveDraft: true,
        onClose,
        onMarkReplied,
        onMarkForwarded,
        onDraftSuperseded: vi.fn(),
        ...overrides,
    }
    const view = render(<ComposeModal {...props}/>)
    const toInput = () => screen.getByPlaceholderText(TO_PLACEHOLDER)
    return {...view, onClose, onMarkReplied, onMarkForwarded, toInput}
}

beforeEach(() => {
    apiSpies.send.mockReset().mockResolvedValue('')
    apiSpies.saveDraft.mockReset().mockResolvedValue(undefined)
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.saveDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.pickAttachments.mockReset().mockResolvedValue([])
    apiSpies.listContacts.mockReset().mockResolvedValue([])
    apiSpies.collectContacts.mockReset().mockResolvedValue(0)
})

afterEach(() => {
    cleanup()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

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

    it('puts initial focus at the start of the body, not the To field', () => {
        const {toInput} = renderCompose()
        expect(editorSpies.options?.autofocus).toBe('start')
        expect(document.activeElement).not.toBe(toInput())
    })

    it('shows the From dropdown only with more than one sender', () => {
        const {rerender} = renderCompose()
        expect(screen.queryByText('From')).toBeNull()
        rerender(
            <ComposeModal
                accountId="acc1"
                senders={[{name: 'Me', address: 'me@x.com'}, {name: 'Alias', address: 'alias@x.com'}]}
                canSaveDraft
                onMarkReplied={vi.fn()}
                onMarkForwarded={vi.fn()}
                onDraftSuperseded={vi.fn()}
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
    it('opens the link row with Apply and Remove, then closes on apply', () => {
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

    it('schedules a preset moment and closes without marking the reply', async () => {
        apiSpies.send.mockResolvedValue('ob-9')
        const {onClose, onMarkReplied} = renderCompose({
            initial: {to: 'x@y.com', subject: 'Hi', inReplyToId: 'orig1', replyKind: 'reply'},
        })
        fireEvent.click(screen.getByRole('button', {name: 'Send later'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Tomorrow morning (09:00)'}))
        await waitFor(() => expect(onClose).toHaveBeenCalled())
        expect(apiSpies.send.mock.calls[0][0].sendAtMs).toBeGreaterThan(Date.now())
        // A scheduled send waits in the Outbox, where it may yet be cancelled, so the reply mark stays
        // honest and never fires.
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

describe('formatting toolbar keyboard navigation', () => {
    const toolbar = () => screen.getByRole('toolbar', {name: 'Formatting'})
    const toolButtons = () => Array.from(toolbar().querySelectorAll<HTMLButtonElement>('button.compose-tool'))

    it('is a single tab stop: exactly one tool is tabbable', () => {
        renderCompose()
        const tabbable = toolButtons().filter((b) => b.tabIndex === 0)
        expect(tabbable).toHaveLength(1)
        expect(tabbable[0]).toHaveAccessibleName('Bold')
    })

    it('moves the roving stop with the arrow keys, wrapping at the ends', () => {
        renderCompose()
        const buttons = toolButtons()
        buttons[0].focus()
        fireEvent.keyDown(toolbar(), {key: 'ArrowRight'})
        expect(document.activeElement).toBe(buttons[1])
        expect(buttons[1].tabIndex).toBe(0)
        expect(buttons[0].tabIndex).toBe(-1)
        fireEvent.keyDown(toolbar(), {key: 'ArrowLeft'})
        fireEvent.keyDown(toolbar(), {key: 'ArrowLeft'})
        expect(document.activeElement).toBe(buttons[buttons.length - 1])
        expect(buttons[buttons.length - 1].tabIndex).toBe(0)
    })

    it('jumps to the last tool with End and back with Home', () => {
        renderCompose()
        const buttons = toolButtons()
        buttons[0].focus()
        fireEvent.keyDown(toolbar(), {key: 'End'})
        expect(document.activeElement).toBe(buttons[buttons.length - 1])
        fireEvent.keyDown(toolbar(), {key: 'Home'})
        expect(document.activeElement).toBe(buttons[0])
    })

    it('keeps focus on the tool after a keyboard activation', () => {
        renderCompose()
        const bold = toolButtons()[0]
        bold.focus()
        // A keyboard-driven click carries detail 0, the browser's signal for a non-pointer click.
        fireEvent.click(bold, {detail: 0})
        expect(document.activeElement).toBe(bold)
    })

    it('leaves focus with the editor after a mouse activation', () => {
        renderCompose()
        const bold = toolButtons()[0]
        fireEvent.click(bold, {detail: 1})
        expect(document.activeElement).not.toBe(bold)
    })

    it('shows the editor shortcut in the tooltip', () => {
        renderCompose()
        expect(toolButtons()[0]).toHaveAttribute('title', 'Bold (Ctrl+B)')
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
