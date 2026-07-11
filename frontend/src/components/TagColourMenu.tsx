import type {Tag} from '../api'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {useTagMenu} from '../hooks/useTagMenu'

interface TagColourMenuProps {
    // messageId closes the menu when the shown message changes; see useTagMenu.
    messageId: string
    messageTags: Tag[]
    onToggleTag: (tagId: string, assigned: boolean) => void
}

// TagColourMenu is the reader's colour-tag control: a Colour button that drops a menu of palette swatches,
// each toggling that colour on the message. Its open state, focus refs and keyboard model live in
// useTagMenu; this renders the button and the swatch grid.
export function TagColourMenu({messageId, messageTags, onToggleTag}: TagColourMenuProps) {
    const {open, menuRef, triggerRef, rowRef, toggle, onTriggerKeyDown, onRowKeyDown} = useTagMenu(messageId)
    const assigned = new Set(messageTags.map((t) => t.id))
    return (
        <div className="tag-menu" ref={menuRef}>
            <button ref={triggerRef} className="btn" onClick={toggle} onKeyDown={onTriggerKeyDown}>
                Colour &#9662;
            </button>
            {open && (
                <div className="tag-menu-dropdown" role="menu">
                    <div
                        className="tag-colour-row"
                        role="group"
                        aria-label="Tag colour"
                        ref={rowRef}
                        onKeyDown={onRowKeyDown}
                    >
                        {TAG_PALETTE.map((c) => {
                            const id = colourTagId(c.colour)
                            const isOn = assigned.has(id)
                            return (
                                <button
                                    key={id}
                                    className={'tag-colour' + (isOn ? ' selected' : '')}
                                    role="menuitemcheckbox"
                                    aria-checked={isOn}
                                    tabIndex={-1}
                                    title={c.name}
                                    style={{backgroundColor: c.colour}}
                                    onClick={() => onToggleTag(id, !isOn)}
                                >
                                    {isOn ? '✓' : ''}
                                </button>
                            )
                        })}
                    </div>
                </div>
            )}
        </div>
    )
}
