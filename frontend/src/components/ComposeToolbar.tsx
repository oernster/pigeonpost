import {Fragment} from 'react'
import type {Template} from '../api'
import type {EditorTool} from '../editorTools'
import {ToolButton} from './ToolButton'
import type {useLinkEditor} from '../hooks/useLinkEditor'
import type {useToolbarNav} from '../hooks/useToolbarNav'

interface ComposeToolbarProps {
    tools: EditorTool[]
    toolbar: ReturnType<typeof useToolbarNav>
    templatePicker: boolean
    templates: Template[]
    insertTemplate: (t: Template) => Promise<void>
    link: ReturnType<typeof useLinkEditor>
}

// ComposeToolbar is the editing strip above the message body: the formatting tools, the template picker they
// open and the inline link row.
export function ComposeToolbar({tools, toolbar, templatePicker, templates, insertTemplate, link}: ComposeToolbarProps) {
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
                        hasPopup={tool.hasPopup}
                        expanded={tool.hasPopup ? templatePicker : undefined}
                    />
                    {tool.sepAfter && <span className="compose-tool-sep"/>}
                </Fragment>
            ))}
        </div>
        {templatePicker && (
            <div className="compose-template-picker" role="menu" aria-label="Message templates">
                {templates.length === 0 ? (
                    <div className="compose-template-empty">No templates yet.</div>
                ) : (
                    templates.map((t) => (
                        <button
                            key={t.id}
                            type="button"
                            role="menuitem"
                            className="compose-template-option"
                            onMouseDown={(e) => e.preventDefault()}
                            onClick={() => void insertTemplate(t)}
                        >
                            <span className="compose-template-name">{t.name}</span>
                            <span className="compose-template-subject">{t.subject || '(no subject)'}</span>
                        </button>
                    ))
                )}
            </div>
        )}
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
                <button className="btn" onClick={link.removeLink}>Remove</button>
            </div>
        )}
        </>
    )
}
