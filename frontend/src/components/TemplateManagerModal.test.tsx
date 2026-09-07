// Behaviour test for the message-template manager at its outer interface (templates, onChanged,
// onClose). It renders the real modal and the real TipTap editor; it holds the three things about a
// template row and its editor that were changed by hand and would otherwise drift back. The row's
// actions are icons sitting beside the text rather than words stacked under it; the body editor
// offers the whole formatting strip a message body can carry rather than three of its tools.
//
// One module is stubbed: ../api (the Wails seam), so the calls the modal makes can be asserted.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {TemplateManagerModal} from './TemplateManagerModal'
import type {Template} from '../api'
import {formattingTools} from '../editorTools'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    saveTemplate: vi.fn(),
    deleteTemplate: vi.fn(),
}))

const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

const templates = [
    {id: 't1', name: 'Chasing an invoice', subject: 'Invoice 41 is overdue', body: '<p>As above.</p>'},
    {id: 't2', name: 'No subject one', subject: '', body: ''},
] as Template[]

function renderModal() {
    const onChanged = vi.fn()
    const onClose = vi.fn()
    render(<TemplateManagerModal templates={templates} onChanged={onChanged} onClose={onClose}/>)
    return {onChanged, onClose}
}

afterEach(() => {
    cleanup()
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

// The opposite check to the drain above: a spy declared under a name the api does not have binds to
// nothing, so every test configuring it would be configuring a stub the code can never call.
describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})

describe('TemplateManagerModal', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        apiSpies.saveTemplate.mockResolvedValue(undefined)
        apiSpies.deleteTemplate.mockResolvedValue(undefined)
    })

    // Edit used to be the word "Edit" while every comparable row in the app carries a pencil. The
    // accessible name stays a sentence naming the template, since a glyph on its own says nothing to a
    // screen reader.
    it('offers editing as a pencil that still names its template', () => {
        renderModal()
        const edit = screen.getByRole('button', {name: 'Edit Chasing an invoice'})
        expect(edit.textContent).toBe('✎')
        expect(edit.getAttribute('title')).toBe('Edit template')
    })

    // The pencil and the cross belong beside the row's text. `.list-item` is a column by default, which
    // stacked them underneath it; the row asks for the modifier that turns it back into a row. Asserting
    // on the class rather than on layout is deliberate: jsdom computes no layout, so a geometry
    // assertion here would pass whatever the stylesheet said.
    it('lays a row out with its actions beside the text', () => {
        renderModal()
        const row = screen.getByRole('button', {name: 'Edit Chasing an invoice'}).closest('li')
        expect(row?.className).toContain('template-row')
    })

    it('confirms a delete before it removes anything', () => {
        renderModal()
        fireEvent.click(screen.getByRole('button', {name: 'Delete Chasing an invoice'}))
        expect(screen.getByText(/Delete "Chasing an invoice"\?/)).toBeTruthy()
        expect(apiSpies.deleteTemplate).not.toHaveBeenCalled()
    })

    // A template is a message body, so its editor carries what the compose window's does. The expected
    // set is read from the shared module rather than listed here, so adding a tool to the strip does not
    // need this test edited; what it holds is that the template editor renders THAT strip.
    it('gives the body editor the compose window formatting strip', () => {
        renderModal()
        const strip = screen.getByLabelText('Formatting')
        const names = [...strip.querySelectorAll('button')].map((b) => b.getAttribute('aria-label'))
        for (const tool of formattingTools(null, () => {})) {
            expect(names.some((name) => name?.startsWith(tool.name)), `${tool.name} is missing`).toBe(true)
        }
    })

    it('loads a template into the form when its pencil is clicked', () => {
        renderModal()
        fireEvent.click(screen.getByRole('button', {name: 'Edit Chasing an invoice'}))
        expect(screen.getByDisplayValue('Chasing an invoice')).toBeTruthy()
        expect(screen.getByDisplayValue('Invoice 41 is overdue')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Save template'})).toBeTruthy()
    })
})
