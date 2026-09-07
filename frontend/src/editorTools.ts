import type {Editor} from '@tiptap/react'

// editorTools is the one home for the rich-text formatting strip. The compose window and the template
// editor offer the same formatting, so the strip is defined once here and each surface renders it;
// stating it twice is how the two drift, which is what happened before this module existed (the
// template editor had bold, italic and a link while the composer had eight tools).

// EditorTool is one entry in a formatting strip: its button face, its editor action and its place in
// the strip's visual grouping. A tool with sepAfter ends a visual group; one with hasPopup opens a menu
// rather than acting on the selection, so its button carries the popup semantics.
export interface EditorTool {
    glyph: string
    name: string
    shortcut?: string
    active: boolean
    run: () => void
    sepAfter?: boolean
    hasPopup?: boolean
}

// FormattingToolOptions asks for a separator after the last tool. A strip that continues with tools of
// its own wants one; a strip that ends there does not, since a trailing separator draws a divider with
// nothing after it.
export interface FormattingToolOptions {
    trailingSeparator?: boolean
}

// formattingTools builds the shared strip: the three character marks, the four block forms and the link.
// The editor may be null while it is being created, which reads as nothing active and an action that
// does nothing, so a surface can render its strip before the editor exists.
export function formattingTools(
    editor: Editor | null,
    openLink: () => void,
    options: FormattingToolOptions = {},
): EditorTool[] {
    const tools: EditorTool[] = [
        {glyph: 'B', name: 'Bold', shortcut: 'Ctrl+B', active: editor?.isActive('bold') ?? false, run: () => editor?.chain().focus().toggleBold().run()},
        {glyph: 'I', name: 'Italic', shortcut: 'Ctrl+I', active: editor?.isActive('italic') ?? false, run: () => editor?.chain().focus().toggleItalic().run()},
        {glyph: 'S', name: 'Strikethrough', shortcut: 'Ctrl+Shift+X', active: editor?.isActive('strike') ?? false, run: () => editor?.chain().focus().toggleStrike().run(), sepAfter: true},
        {glyph: 'H', name: 'Heading', shortcut: 'Ctrl+Alt+2', active: editor?.isActive('heading', {level: 2}) ?? false, run: () => editor?.chain().focus().toggleHeading({level: 2}).run()},
        {glyph: '•', name: 'Bullet list', shortcut: 'Ctrl+Shift+8', active: editor?.isActive('bulletList') ?? false, run: () => editor?.chain().focus().toggleBulletList().run()},
        {glyph: '1.', name: 'Numbered list', shortcut: 'Ctrl+Shift+7', active: editor?.isActive('orderedList') ?? false, run: () => editor?.chain().focus().toggleOrderedList().run()},
        {glyph: '”', name: 'Quote', shortcut: 'Ctrl+Shift+B', active: editor?.isActive('blockquote') ?? false, run: () => editor?.chain().focus().toggleBlockquote().run(), sepAfter: true},
        {glyph: '🔗', name: 'Link', active: editor?.isActive('link') ?? false, run: openLink, sepAfter: options.trailingSeparator === true},
    ]
    return tools
}

// signatureTools is the reduced strip for an account signature, which is a line or two of text rather
// than a message: bold, italic and a link, with no block forms.
export function signatureTools(editor: Editor | null, openLink: () => void): EditorTool[] {
    return [
        {glyph: 'B', name: 'Bold', shortcut: 'Ctrl+B', active: editor?.isActive('bold') ?? false, run: () => editor?.chain().focus().toggleBold().run()},
        {glyph: 'I', name: 'Italic', shortcut: 'Ctrl+I', active: editor?.isActive('italic') ?? false, run: () => editor?.chain().focus().toggleItalic().run()},
        {glyph: '🔗', name: 'Link', active: editor?.isActive('link') ?? false, run: openLink},
    ]
}
