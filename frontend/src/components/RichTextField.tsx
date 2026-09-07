import {Fragment} from 'react'
import type {Editor} from '@tiptap/react'
import {EditorContent} from '@tiptap/react'
import {normaliseSigUrl} from '../accountProviders'
import {formattingTools, signatureTools} from '../editorTools'
import {useLinkEditor} from '../hooks/useLinkEditor'
import {ToolButton} from './ToolButton'
import {useToolbarNav} from '../hooks/useToolbarNav'

interface RichTextFieldProps {
    editor: Editor | null
    // full asks for the compose window's whole formatting strip rather than the signature's three tools.
    // A template is a message body, so it wants everything a message body can carry; a signature is a
    // line or two under one, so it does not.
    full?: boolean
}

// RichTextField is the small rich-text editor used for an account signature and for a message template:
// a formatting toolbar, an inline link-editing row and the editor body. The editor is created by the
// parent, which owns its content and reads its HTML on save; this renders the editing surface around it
// and drives the link row through the shared useLinkEditor hook. The toolbar is one focus stop with
// roving tabindex, the same model as the compose window's strip (see useToolbarNav); its tools come
// from the same module the compose window's do (see editorTools), so the two cannot drift apart.
export function RichTextField({editor, full}: RichTextFieldProps) {
    const link = useLinkEditor(editor, normaliseSigUrl)
    const tools = full === true ? formattingTools(editor, link.openLink) : signatureTools(editor, link.openLink)
    const toolbar = useToolbarNav(tools.length)
    return (
        <>
            <div className="compose-toolbar" aria-label="Formatting" {...toolbar.toolbarProps}>
                {tools.map((tool, index) => (
                    <Fragment key={tool.name}>
                        <ToolButton
                            active={tool.active}
                            glyph={tool.glyph}
                            name={tool.name}
                            shortcut={tool.shortcut}
                            tabIndex={toolbar.toolTabIndex(index)}
                            onActivate={tool.run}
                        />
                        {tool.sepAfter === true && <span className="compose-tool-sep"/>}
                    </Fragment>
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
