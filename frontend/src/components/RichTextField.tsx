import type {Editor} from '@tiptap/react'
import {EditorContent} from '@tiptap/react'
import {normaliseSigUrl} from '../accountProviders'
import {useLinkEditor} from '../hooks/useLinkEditor'
import {ToolButton} from './ToolButton'
import {useToolbarNav} from '../hooks/useToolbarNav'

interface RichTextFieldProps {
    editor: Editor | null
}

// RichTextField is the small rich-text editor used for a signature: a Bold, Italic and Link toolbar, an
// inline link-editing row and the editor body. The editor is created by the parent, which owns its content
// and reads its HTML on save; this renders the editing surface around it and drives the link row through the
// shared useLinkEditor hook. The toolbar is one focus stop with roving tabindex, the same model as the
// compose window's strip (see useToolbarNav).
export function RichTextField({editor}: RichTextFieldProps) {
    const link = useLinkEditor(editor, normaliseSigUrl)
    const tools = [
        {glyph: 'B', name: 'Bold', shortcut: 'Ctrl+B', active: editor?.isActive('bold') ?? false, run: () => editor?.chain().focus().toggleBold().run()},
        {glyph: 'I', name: 'Italic', shortcut: 'Ctrl+I', active: editor?.isActive('italic') ?? false, run: () => editor?.chain().focus().toggleItalic().run()},
        {glyph: '🔗', name: 'Link', active: editor?.isActive('link') ?? false, run: link.openLink},
    ]
    const toolbar = useToolbarNav(tools.length)
    return (
        <>
            <div className="compose-toolbar" aria-label="Formatting" {...toolbar.toolbarProps}>
                {tools.map((tool, index) => (
                    <ToolButton
                        key={tool.name}
                        active={tool.active}
                        glyph={tool.glyph}
                        name={tool.name}
                        shortcut={tool.shortcut}
                        tabIndex={toolbar.toolTabIndex(index)}
                        onActivate={tool.run}
                    />
                ))}
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
