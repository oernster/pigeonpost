import {useState} from 'react'
import type {KeyboardEvent} from 'react'
import {DatePickerDialog} from './DatePickerDialog'
import {datePart, mergeDate} from '../datePicker'

interface DateFieldProps {
    // kind mirrors the input type: a plain date or a datetime-local whose typed time survives a pick.
    kind: 'date' | 'datetime-local'
    value: string
    onChange: (value: string) => void
    ariaLabel: string
    pickerTitle: string
    min?: string
    className?: string
    autoFocus?: boolean
    // compact keeps the field at the input's intrinsic width instead of stretching to fill its row,
    // for a field that sits alone on a row (the birthday) or in a packed row (send later).
    compact?: boolean
    // onEnter lets a host dialog submit from the field (the snooze picker), matching the input it
    // replaces.
    onEnter?: () => void
}

// DateField is the app's date entry: the native input (typing stays first-class) plus a calendar
// button that opens the themed DatePickerDialog instead of the engine's own minimal picker. Picking
// a day merges just the date into the value, so a datetime field keeps the time already typed.
export function DateField({kind, value, onChange, ariaLabel, pickerTitle, min, className, autoFocus, compact, onEnter}: DateFieldProps) {
    const [open, setOpen] = useState(false)

    const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
        if (onEnter && e.key === 'Enter') {
            e.preventDefault()
            onEnter()
        }
    }

    return (
        <div className={compact ? 'date-field compact' : 'date-field'}>
            <input
                className={className ?? 'tag-name-input'}
                type={kind}
                aria-label={ariaLabel}
                value={value}
                min={min}
                autoFocus={autoFocus}
                onChange={(e) => onChange(e.target.value)}
                onKeyDown={onKeyDown}
            />
            <button
                type="button"
                className="date-picker-btn"
                aria-label={`Open the ${pickerTitle.toLowerCase()} calendar`}
                onClick={() => setOpen(true)}
            >
                <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor"
                     strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <rect x="3" y="4" width="18" height="18" rx="2"/>
                    <line x1="16" y1="2" x2="16" y2="6"/>
                    <line x1="8" y1="2" x2="8" y2="6"/>
                    <line x1="3" y1="10" x2="21" y2="10"/>
                </svg>
            </button>
            {open && (
                <DatePickerDialog
                    title={pickerTitle}
                    value={datePart(value)}
                    min={min ?? ''}
                    onPick={(iso) => onChange(mergeDate(value, iso, kind))}
                    onClose={() => setOpen(false)}
                />
            )}
        </div>
    )
}
