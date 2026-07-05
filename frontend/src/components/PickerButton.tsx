import {type RefObject} from 'react'

// PickerButton opens a date field's native calendar. The browser's own picker icon is hidden because its
// focus state cannot be styled reliably, so this is a normal focusable button instead: a white glyph at
// rest, and a teal square when hovered or focused, so it is obvious when it holds keyboard focus.
export function PickerButton({target}: {target: RefObject<HTMLInputElement>}) {
    return (
        <button
            type="button"
            className="date-picker-btn"
            aria-label="Open the date picker"
            onClick={() => target.current?.showPicker()}
        >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor"
                 strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <rect x="3" y="4" width="18" height="18" rx="2"/>
                <line x1="16" y1="2" x2="16" y2="6"/>
                <line x1="8" y1="2" x2="8" y2="6"/>
                <line x1="3" y1="10" x2="21" y2="10"/>
            </svg>
        </button>
    )
}
