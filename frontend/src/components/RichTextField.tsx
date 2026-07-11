import type {Editor} from '@tiptap/react'
import {EditorContent} from '@tiptap/react'
import {normaliseSigUrl} from '../accountProviders'
import {useLinkEditor} from '../hooks/useLinkEditor'

interface RichTextFieldProps {
    editor: Editor | null
}

// toolButton is one formatting toggle in the toolbar. The onMouseDown preventDefault keeps the editor
// selection while the button is pressed, so a toggle applies to the selected text.
function toolButton(active: boolean, label: string, title: string, onClick: () => void) {
    return (
        <button
            type="button"
            className={'compose-tool' + (active ? ' active' : '')}
            title={title}
            aria-label={title}
            aria-pressed={active}
            onMouseDown={(e) => e.preventDefault()}
            onClick={onClick}
        >
            {label}
        </button>
    )
}

// RichTextField is the small rich-text editor used for a signature: a Bold, Italic and Link toolbar, an
// inline link-editing row and the editor body. The editor is created by the parent, which owns its content
// and reads its HTML on save; this renders the editing surface around it and drives the link row through the
// shared useLinkEditor hook.
export function RichTextField({editor}: RichTextFieldProps) {
    const link = useLinkEditor(editor, normaliseSigUrl)
    return (
        <>
            <div className="compose-toolbar">
                {toolButton(editor?.isActive('bold') ?? false, 'B', 'Bold', () => editor?.chain().focus().toggleBold().run())}
                {toolButton(editor?.isActive('italic') ?? false, 'I', 'Italic', () => editor?.chain().focus().toggleItalic().run())}
                {toolButton(editor?.isActive('link') ?? false, '🔗', 'Link', link.openLink)}
            </div>
            {link.open && (
                <div className="compose-link-row">
                    <input
                        className="tag-name-input"
                        value={link.url}
                        autoFocus
                        placeholder="https://example.com"
                        onChange={(e) => link.setUrl(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                link.applyLink()
                            }
                        }}
                    />
                    <button className="btn primary" onClick={link.applyLink}>Apply</button>
                </div>
            )}
            <EditorContent editor={editor} className="compose-editor signature-editor"/>
        </>
    )
}
