// Behaviour test for the compose window's template picker, the path that had no coverage at all and
// so went unnoticed: a template could be written in the manager and never used, because the button
// that inserts one wore the word "Template" among a row of glyphs and did not read as a control.
// These tests hold that it is an icon, that it announces itself, that it lists what the user has and
// that inserting one never overwrites a subject already typed.
//
// ../api is stubbed (the Wails seam) and @tiptap/react too, since ProseMirror does not run in jsdom.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import type {ComponentProps} from 'react'
import {ComposeModal} from './ComposeModal'
import type {Template} from '../api'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    send: vi.fn(),
    saveDraft: vi.fn(),
    clearDraftRecovery: vi.fn(),
    saveDraftRecovery: vi.fn(),
    pickAttachments: vi.fn(),
    listContacts: vi.fn(),
    collectContacts: vi.fn(),
    listTemplates: vi.fn(),
}))

const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

// The editor stub records what was inserted, which is how a body reaching the document is asserted
// without ProseMirror. insertContent is part of the chain here because the template path calls it.
const editorSpies = vi.hoisted(() => ({inserted: [] as string[]}))

vi.mock('@tiptap/react', () => {
    const chain = () => {
        const c: Record<string, (arg?: unknown) => unknown> = {}
        for (const m of [
            'focus', 'toggleBold', 'toggleItalic', 'toggleStrike', 'toggleHeading', 'toggleBulletList',
            'toggleOrderedList', 'toggleBlockquote', 'extendMarkRange', 'setLink', 'unsetLink', 'run',
        ]) {
            c[m] = () => c
        }
        c.insertContent = (html?: unknown) => {
            editorSpies.inserted.push(String(html))
            return c
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

const templates = [
    {id: 't1', name: 'Chasing an invoice', subject: 'Invoice 41 is overdue', body: '<p>As above.</p>'},
    {id: 't2', name: 'Bodyless one', subject: 'Just a subject', body: ''},
] as Template[]

function renderCompose(overrides: Partial<ComposeProps> = {}) {
    const props: ComposeProps = {
        accountId: 'acc1',
        senders: [{name: 'Me', address: 'me@x.com'}],
        canSaveDraft: true,
        onClose: vi.fn(),
        onMarkReplied: vi.fn(),
        onMarkForwarded: vi.fn(),
        onDraftSuperseded: vi.fn(),
        ...overrides,
    }
    return render(<ComposeModal {...props}/>)
}

// openPicker clicks the tool and waits for the loaded list, since the templates are fetched on first
// open rather than when the window is built.
async function openPicker() {
    fireEvent.click(screen.getByRole('button', {name: 'Insert template'}))
    await waitFor(() => expect(screen.getByRole('menu', {name: 'Message templates'})).toBeTruthy())
}

beforeEach(() => {
    vi.clearAllMocks()
    editorSpies.inserted = []
    apiSpies.send.mockResolvedValue('')
    apiSpies.saveDraft.mockResolvedValue(undefined)
    apiSpies.clearDraftRecovery.mockResolvedValue(undefined)
    apiSpies.saveDraftRecovery.mockResolvedValue(undefined)
    apiSpies.pickAttachments.mockResolvedValue([])
    apiSpies.listContacts.mockResolvedValue([])
    apiSpies.collectContacts.mockResolvedValue(0)
    apiSpies.listTemplates.mockResolvedValue(templates)
})

afterEach(() => {
    cleanup()
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})

describe('the compose template picker', () => {
    // The regression this file exists for: a word in a strip of glyphs reads as a label, not a button.
    it('offers the picker as an icon that still announces itself', () => {
        renderCompose()
        const button = screen.getByRole('button', {name: 'Insert template'})
        expect(button.textContent).toBe('📄')
        expect(button.getAttribute('aria-haspopup')).toBe('menu')
        expect(button.getAttribute('aria-expanded')).toBe('false')
    })

    it('loads the templates when the picker is first opened', async () => {
        renderCompose()
        expect(apiSpies.listTemplates).not.toHaveBeenCalled()
        await openPicker()
        expect(apiSpies.listTemplates).toHaveBeenCalledTimes(1)
        expect(screen.getByText('Chasing an invoice')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Insert template'}).getAttribute('aria-expanded')).toBe('true')
    })

    it('fills an empty subject and inserts the body', async () => {
        renderCompose()
        await openPicker()
        fireEvent.click(screen.getByText('Chasing an invoice'))
        expect(screen.getByDisplayValue('Invoice 41 is overdue')).toBeTruthy()
        expect(editorSpies.inserted).toEqual(['<p>As above.</p>'])
    })

    // A template is inserted into a message being written, so it must not overwrite what is there.
    it('leaves a subject the user has already typed', async () => {
        renderCompose()
        const subject = screen.getByLabelText('Subject')
        fireEvent.change(subject, {target: {value: 'Mine'}})
        await openPicker()
        fireEvent.click(screen.getByText('Chasing an invoice'))
        expect(screen.getByDisplayValue('Mine')).toBeTruthy()
    })

    it('inserts nothing into the body for a template that carries none', async () => {
        renderCompose()
        await openPicker()
        fireEvent.click(screen.getByText('Bodyless one'))
        expect(editorSpies.inserted).toEqual([])
    })

    it('says so rather than showing an empty menu when there are no templates', async () => {
        apiSpies.listTemplates.mockResolvedValue([])
        renderCompose()
        await openPicker()
        expect(screen.getByText('No templates yet.')).toBeTruthy()
    })
})
