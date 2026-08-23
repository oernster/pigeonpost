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
        holdSeconds: 0,
        onHeld: vi.fn(),
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
