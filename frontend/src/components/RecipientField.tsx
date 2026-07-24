import {useState} from 'react'
import type {KeyboardEvent} from 'react'
import {applySuggestion, Suggestion, suggestionsFor} from '../recipientSuggest'

interface RecipientFieldProps {
    label: string
    value: string
    placeholder?: string
    // pool is the shared contact suggestion pool; ensurePool triggers its lazy load on first touch.
    pool: Suggestion[]
    ensurePool: () => void
    onChange: (value: string) => void
}

// RecipientField is one To / Cc / Bcc row: an ordinary text input with a contact-suggestion
// dropdown under it while an address fragment is being typed. Accepting a suggestion (Enter, Tab
// or a click) only inserts text, so the field stays freely editable afterwards; Escape closes the
// dropdown without touching the field (and without closing the dialog, which owns Escape
// otherwise). Arrow keys move the highlight and wrap.
export function RecipientField({label, value, placeholder, pool, ensurePool, onChange}: RecipientFieldProps) {
    const [open, setOpen] = useState(false)
    const [highlight, setHighlight] = useState(0)

    const suggestions = open ? suggestionsFor(pool, value) : []
    // The highlight is clamped rather than reset on every keystroke, so it survives the list
    // shrinking as the fragment narrows.
    const active = Math.min(highlight, Math.max(suggestions.length - 1, 0))

    const accept = (suggestion: Suggestion) => {
        onChange(applySuggestion(value, suggestion))
        setOpen(false)
    }

    const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
        if (suggestions.length === 0 || e.ctrlKey || e.metaKey || e.altKey) {
            return
        }
        if (e.key === 'ArrowDown') {
            e.preventDefault()
            setHighlight((active + 1) % suggestions.length)
        } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            setHighlight((active - 1 + suggestions.length) % suggestions.length)
        } else if (e.key === 'Enter') {
            e.preventDefault()
            accept(suggestions[active])
        } else if (e.key === 'Tab') {
            // Accept without preventDefault, so Tab both takes the suggestion and moves on.
            accept(suggestions[active])
        } else if (e.key === 'Escape') {
            // Stop the dialog's own Escape handling: the first press closes the dropdown only.
            e.preventDefault()
            e.stopPropagation()
            setOpen(false)
        }
    }

    return (
        <label className="field">
            <span>{label}</span>
            <div className="recipient-wrap">
                <input
                    value={value}
                    placeholder={placeholder}
                    aria-autocomplete="list"
                    aria-expanded={suggestions.length > 0}
                    onFocus={ensurePool}
                    onChange={(e) => {
                        ensurePool()
                        setOpen(true)
                        setHighlight(0)
                        onChange(e.target.value)
                    }}
                    onBlur={() => setOpen(false)}
                    onKeyDown={onKeyDown}
                />
                {suggestions.length > 0 && (
                    <ul className="recipient-suggest" role="listbox" aria-label={`${label} suggestions`}>
                        {suggestions.map((s, index) => (
                            <li
                                key={s.address}
                                role="option"
                                aria-selected={index === active}
                                className={index === active ? 'recipient-option active' : 'recipient-option'}
                                // mousedown is swallowed so the input keeps focus and its blur-close
                                // never races the click that accepts.
                                onMouseDown={(e) => e.preventDefault()}
                                onClick={() => accept(s)}
                                onMouseEnter={() => setHighlight(index)}
                            >
                                <span className="recipient-option-name">{s.name || s.address}</span>
                                {s.name !== '' && <span className="recipient-option-address">{s.address}</span>}
                            </li>
                        ))}
                    </ul>
                )}
            </div>
        </label>
    )
}
