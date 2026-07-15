// editClipboard holds the gating and DOM plumbing behind Edit > Cut / Copy / Paste. The menu acts
// on the main window's text surfaces only (the search box, and any selected text in the reader),
// matching Thunderbird's 3-pane Edit menu: messages are never cut or pasted; Delete is the
// message-level member of the group. No React, no api runtime, so it is unit-tested in isolation;
// reading the clipboard for a paste goes through the Wails runtime in the hook layer.

// EditContext is a snapshot of what the clipboard commands could act on: whether an editable text
// field holds focus (paste target), whether any text is selected anywhere (copy source) and
// whether that selection sits inside the focused editable (cut source).
export interface EditContext {
    editableFocused: boolean
    hasSelection: boolean
    selectionInEditable: boolean
}

// isTextEntry reports whether an element takes typed text: an input, a textarea or a
// contenteditable region. The menus use it too: clicking a menu must not steal focus from a text
// field (or the clipboard commands lose their target) and a menu accelerator that shadows a native
// editing key (Ctrl+Z) must stand down while one has focus.
export function isTextEntry(el: Element | null): el is HTMLElement {
    if (!el) {
        return false
    }
    return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || Boolean((el as HTMLElement).isContentEditable)
}

// readEditContext snapshots the document's focus and selection state for the menu gating. An
// input or textarea carries its own selection range; anything else is read from the document
// selection.
export function readEditContext(doc: Document): EditContext {
    const active = doc.activeElement
    const editableFocused = isTextEntry(active)
    if (active && (active.tagName === 'INPUT' || active.tagName === 'TEXTAREA')) {
        const field = active as HTMLInputElement | HTMLTextAreaElement
        const fieldSelection = field.selectionStart !== null && field.selectionStart !== field.selectionEnd
        return {editableFocused, hasSelection: fieldSelection, selectionInEditable: fieldSelection}
    }
    const hasSelection = (doc.getSelection()?.toString() ?? '') !== ''
    return {editableFocused, hasSelection, selectionInEditable: editableFocused && hasSelection}
}

// canCut, canCopy and canPaste gate the three menu items: cutting needs a selection it may remove
// (inside an editable), copying any selection at all and pasting a field to paste into. Paste
// does not inspect the clipboard (reading it is async and may prompt); pasting nothing is a no-op.
export function canCut(ctx: EditContext): boolean {
    return ctx.selectionInEditable
}

export function canCopy(ctx: EditContext): boolean {
    return ctx.hasSelection
}

export function canPaste(ctx: EditContext): boolean {
    return ctx.editableFocused
}

// cutSelection and copySelection run the native editing commands against the current selection,
// which the menu preserves by never taking focus (its buttons swallow mousedown). pasteText
// inserts clipboard text at the caret with insertText, so the field's own undo history records it.
export function cutSelection(doc: Document): void {
    doc.execCommand('cut')
}

export function copySelection(doc: Document): void {
    doc.execCommand('copy')
}

export function pasteText(doc: Document, text: string): void {
    if (text === '') {
        return
    }
    doc.execCommand('insertText', false, text)
}
