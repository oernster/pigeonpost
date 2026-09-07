// The compose discard guard at its outer interface: a compose the user has actually edited must
// confirm before any discard path (backdrop click, Escape, the close cross, Cancel) throws it away,
// because a stray click, such as the one that refocuses the window, lands on the backdrop and would
// otherwise silently lose the message. An untouched or emptied-out compose closes at once, as before.
// The api and TipTap stubs mirror ComposeModal.test.tsx; the editor stub reports an empty body, so
// content comes from the recipient and subject fields, which is exactly what marks the compose dirty.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen, within} from '@testing-library/react'
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
        useEditor: () => editor,
        EditorContent: () => null,
    }
})

type ComposeProps = ComponentProps<typeof ComposeModal>

const TO_PLACEHOLDER = 'name@example.com, other@example.com'
// The backdrop arms its dismiss shortly after the dialog opens (DISMISS_ARM_MS in
// useBackdropDismiss); advance past it so a backdrop click registers.
const ARM_WAIT_MS = 500

function renderCompose(overrides: Partial<ComposeProps> = {}) {
    const onClose = vi.fn()
    const props: ComposeProps = {
        accountId: 'acc1',
        senders: [{name: 'Me', address: 'me@x.com'}],
        canSaveDraft: true,
        onClose,
        onMarkReplied: vi.fn(),
        onMarkForwarded: vi.fn(),
        onDraftSuperseded: vi.fn(),
        ...overrides,
    }
    const view = render(<ComposeModal {...props}/>)
    const toInput = () => screen.getByPlaceholderText(TO_PLACEHOLDER)
    const backdrop = () => view.container.querySelector('.modal-backdrop') as HTMLElement
    const clickBackdrop = () => {
        act(() => {
            vi.advanceTimersByTime(ARM_WAIT_MS)
        })
        fireEvent.mouseDown(backdrop())
        fireEvent.click(backdrop())
    }
    return {...view, onClose, toInput, clickBackdrop}
}

const discardDialog = () => screen.queryByRole('alertdialog', {name: 'Discard message?'})

beforeEach(() => {
    vi.useFakeTimers()
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
    vi.useRealTimers()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('ComposeModal: discard guard', () => {
    it('asks before a backdrop click discards an edited compose', () => {
        const {onClose, toInput, clickBackdrop} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})
        clickBackdrop()
        expect(discardDialog()).toBeInTheDocument()
        expect(onClose).not.toHaveBeenCalled()
        expect((toInput() as HTMLInputElement).value).toBe('bob@example.com')
    })

    it('closes an untouched compose on a backdrop click without asking', () => {
        const {onClose, clickBackdrop} = renderCompose()
        clickBackdrop()
        expect(discardDialog()).toBeNull()
        expect(onClose).toHaveBeenCalled()
    })

    it('closes an emptied-out compose without asking', () => {
        const {onClose, toInput, clickBackdrop} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})
        fireEvent.change(toInput(), {target: {value: ''}})
        clickBackdrop()
        expect(discardDialog()).toBeNull()
        expect(onClose).toHaveBeenCalled()
    })

    it('asks before Escape discards an edited compose; Cancel keeps writing', () => {
        const {onClose, toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})
        fireEvent.keyDown(document, {key: 'Escape'})
        expect(discardDialog()).toBeInTheDocument()
        fireEvent.click(within(discardDialog()!).getByRole('button', {name: 'Cancel'}))
        expect(discardDialog()).toBeNull()
        expect(onClose).not.toHaveBeenCalled()
        expect((toInput() as HTMLInputElement).value).toBe('bob@example.com')
    })

    it('asks on the close cross and on the Cancel button; Discard then closes', () => {
        const {onClose, toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})
        fireEvent.mouseDown(screen.getByRole('button', {name: 'Close'}))
        expect(discardDialog()).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Discard'}))
        expect(onClose).toHaveBeenCalledTimes(1)
    })

    it('asks on the compose Cancel action too', () => {
        const {onClose, toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})
        const dialog = screen.getByRole('dialog', {name: 'New message'})
        const cancel = Array.from(dialog.querySelectorAll('button')).find((b) => b.textContent === 'Cancel')
        fireEvent.click(cancel!)
        expect(discardDialog()).toBeInTheDocument()
        expect(onClose).not.toHaveBeenCalled()
    })

    it('guards a subject-only compose too', () => {
        const {onClose} = renderCompose()
        const subject = screen.getByText('Subject').parentElement!.querySelector('input')!
        fireEvent.change(subject, {target: {value: 'Half-written thought'}})
        fireEvent.keyDown(document, {key: 'Escape'})
        expect(discardDialog()).toBeInTheDocument()
        expect(onClose).not.toHaveBeenCalled()
    })
})

// The recovery slot is the local snapshot taken while a message is being written, so a crash or a
// stray close does not lose it. Discarding must clear it. It did not, so the failure only showed on
// the NEXT launch: the message is gone from the screen while its snapshot is still on disk, so the
// app opens offering to recover a message the user watched itself throw away.
//
// The autosave is debounced, so these drive the clock past it to get a snapshot written first,
// which is the state the bug needs.
const AUTOSAVE_WAIT_MS = 2000

describe('discarding a message', () => {
    // discardIt opens the guard and agrees to it.
    const discardIt = () => {
        fireEvent.keyDown(document, {key: 'Escape'})
        const dialog = discardDialog()!
        const confirm = Array.from(dialog.querySelectorAll('button')).find((b) => b.textContent === 'Discard')
        fireEvent.click(confirm!)
    }

    it('clears the recovery snapshot it had already written', () => {
        const {onClose, toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})
        act(() => {
            vi.advanceTimersByTime(AUTOSAVE_WAIT_MS)
        })
        expect(apiSpies.saveDraftRecovery).toHaveBeenCalled()

        discardIt()

        expect(onClose).toHaveBeenCalled()
        expect(apiSpies.clearDraftRecovery).toHaveBeenCalled()
    })

    // The debounce is the second half of the same bug: a snapshot already scheduled must not land after
    // the message has been thrown away, which would write the slot straight back.
    it('does not let a pending snapshot land after the discard', () => {
        const {toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})

        discardIt()
        apiSpies.saveDraftRecovery.mockClear()
        act(() => {
            vi.advanceTimersByTime(AUTOSAVE_WAIT_MS)
        })

        expect(apiSpies.saveDraftRecovery).not.toHaveBeenCalled()
    })

    // The other way to the same state, taking no confirmation at all: write something, so a
    // snapshot lands, then empty it out again. The compose is then dirty with no content, so closing
    // asks nothing and goes straight out. The autosave would have cleared the slot itself, yet only
    // after its debounce, which closing the window cancels.
    it('clears the snapshot when an emptied-out compose closes without asking', () => {
        const {onClose, toInput} = renderCompose()
        fireEvent.change(toInput(), {target: {value: 'bob@example.com'}})
        act(() => {
            vi.advanceTimersByTime(AUTOSAVE_WAIT_MS)
        })
        expect(apiSpies.saveDraftRecovery).toHaveBeenCalled()

        fireEvent.change(toInput(), {target: {value: ''}})
        fireEvent.keyDown(document, {key: 'Escape'})

        expect(discardDialog()).toBeNull()
        expect(onClose).toHaveBeenCalled()
        expect(apiSpies.clearDraftRecovery).toHaveBeenCalled()
    })

    // The limit on all of the above. The recovery slot is a single slot, so a compose that never wrote
    // anything must not clear it: an untouched compose opened and closed over the top of a snapshot left
    // by an earlier session would otherwise throw that message away, which is the very loss the slot
    // exists to prevent.
    it('leaves the slot alone when the compose wrote nothing', () => {
        const {onClose} = renderCompose()
        fireEvent.keyDown(document, {key: 'Escape'})

        expect(onClose).toHaveBeenCalled()
        expect(apiSpies.clearDraftRecovery).not.toHaveBeenCalled()
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
