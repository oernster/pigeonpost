import type {MouseEvent} from 'react'

interface ToolButtonProps {
    active: boolean
    glyph: string
    name: string
    shortcut?: string
    tabIndex: number
    onActivate: () => void
    hasPopup?: boolean
    expanded?: boolean
}

// ToolButton is one tool in a roving-tabindex toolbar (see useToolbarNav). The onMouseDown
// preventDefault keeps the editor selection while the button is pressed, so a toggle applies to
// the selected text. A keyboard activation (click detail 0) returns focus to the button, so
// toggling two formats composes without re-entering the strip; a mouse activation leaves focus
// with the editor exactly as before. The tooltip carries the editor shortcut where one exists.
export function ToolButton({active, glyph, name, shortcut, tabIndex, onActivate, hasPopup, expanded}: ToolButtonProps) {
    return (
        <button
            type="button"
            className={'compose-tool' + (active ? ' active' : '')}
            title={shortcut ? `${name} (${shortcut})` : name}
            aria-label={name}
            aria-pressed={active}
            aria-haspopup={hasPopup ? 'menu' : undefined}
            aria-expanded={hasPopup ? expanded : undefined}
            tabIndex={tabIndex}
            onMouseDown={(e) => e.preventDefault()}
            onClick={(e: MouseEvent<HTMLButtonElement>) => {
                const fromKeyboard = e.detail === 0
                const button = e.currentTarget
                onActivate()
                if (fromKeyboard) {
                    button.focus()
                }
            }}
        >
            {glyph}
        </button>
    )
}
