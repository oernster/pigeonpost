import {afterEach, describe, expect, it, vi} from 'vitest'
import {
    canCopy, canCut, canPaste, copySelection, cutSelection, pasteText, readEditContext,
} from './editClipboard'

afterEach(() => {
    document.body.innerHTML = ''
    document.getSelection()?.removeAllRanges()
})

// selectDocumentText selects the text content of the given element through the document
// selection, the way dragging over reader text does.
function selectDocumentText(el: HTMLElement): void {
    const range = document.createRange()
    range.selectNodeContents(el)
    const selection = document.getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)
}

describe('readEditContext', () => {
    it('reports a focused input with a selection as cuttable', () => {
        const input = document.createElement('input')
        input.value = 'hello'
        document.body.appendChild(input)
        input.focus()
        input.setSelectionRange(0, 3)
        const ctx = readEditContext(document)
        expect(ctx).toEqual({editableFocused: true, hasSelection: true, selectionInEditable: true})
    })

    it('reports a focused input without a selection as paste-only', () => {
        const input = document.createElement('input')
        input.value = 'hello'
        document.body.appendChild(input)
        input.focus()
        input.setSelectionRange(2, 2)
        const ctx = readEditContext(document)
        expect(ctx).toEqual({editableFocused: true, hasSelection: false, selectionInEditable: false})
    })

    it('reports a focused textarea with a selection as cuttable', () => {
        const area = document.createElement('textarea')
        area.value = 'hello'
        document.body.appendChild(area)
        area.focus()
        area.setSelectionRange(0, 5)
        const ctx = readEditContext(document)
        expect(ctx).toEqual({editableFocused: true, hasSelection: true, selectionInEditable: true})
    })

    it('treats a field without an applicable selection range as unselected', () => {
        const input = document.createElement('input')
        document.body.appendChild(input)
        input.focus()
        Object.defineProperty(input, 'selectionStart', {value: null})
        const ctx = readEditContext(document)
        expect(ctx).toEqual({editableFocused: true, hasSelection: false, selectionInEditable: false})
    })

    it('reports selected reader text as copyable but not cuttable', () => {
        const p = document.createElement('p')
        p.textContent = 'reader text'
        document.body.appendChild(p)
        selectDocumentText(p)
        const ctx = readEditContext(document)
        expect(ctx).toEqual({editableFocused: false, hasSelection: true, selectionInEditable: false})
    })

    it('reports a focused contenteditable with a selection as cuttable', () => {
        const div = document.createElement('div')
        div.textContent = 'rich text'
        div.tabIndex = 0
        Object.defineProperty(div, 'isContentEditable', {value: true})
        document.body.appendChild(div)
        div.focus()
        selectDocumentText(div)
        const ctx = readEditContext(document)
        expect(ctx).toEqual({editableFocused: true, hasSelection: true, selectionInEditable: true})
    })

    it('reports nothing to act on with no focus and no selection', () => {
        const ctx = readEditContext(document)
        expect(ctx).toEqual({editableFocused: false, hasSelection: false, selectionInEditable: false})
    })

    it('handles a document with no active element and no selection object', () => {
        const doc = {activeElement: null, getSelection: () => null} as unknown as Document
        const ctx = readEditContext(doc)
        expect(ctx).toEqual({editableFocused: false, hasSelection: false, selectionInEditable: false})
    })
})

describe('gating', () => {
    it('cut needs the selection inside an editable', () => {
        expect(canCut({editableFocused: true, hasSelection: true, selectionInEditable: true})).toBe(true)
        expect(canCut({editableFocused: false, hasSelection: true, selectionInEditable: false})).toBe(false)
    })

    it('copy needs any selection', () => {
        expect(canCopy({editableFocused: false, hasSelection: true, selectionInEditable: false})).toBe(true)
        expect(canCopy({editableFocused: true, hasSelection: false, selectionInEditable: false})).toBe(false)
    })

    it('paste needs a focused editable', () => {
        expect(canPaste({editableFocused: true, hasSelection: false, selectionInEditable: false})).toBe(true)
        expect(canPaste({editableFocused: false, hasSelection: true, selectionInEditable: false})).toBe(false)
    })
})

describe('commands', () => {
    it('cut and copy run the native editing commands', () => {
        const execCommand = vi.fn()
        const doc = {execCommand} as unknown as Document
        cutSelection(doc)
        copySelection(doc)
        expect(execCommand.mock.calls).toEqual([['cut'], ['copy']])
    })

    it('paste inserts the text at the caret', () => {
        const execCommand = vi.fn()
        const doc = {execCommand} as unknown as Document
        pasteText(doc, 'hello')
        expect(execCommand).toHaveBeenCalledWith('insertText', false, 'hello')
    })

    it('pasting an empty clipboard is a no-op', () => {
        const execCommand = vi.fn()
        const doc = {execCommand} as unknown as Document
        pasteText(doc, '')
        expect(execCommand).not.toHaveBeenCalled()
    })
})
