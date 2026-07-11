import {useEffect, useRef, useState} from 'react'
import type {KeyboardEvent as ReactKeyboardEvent} from 'react'

// useTagMenu owns the reader's colour-menu island: whether the swatch menu is open, the refs the menu
// wires, the outside-click and keyboard-open focus effects and the two keyboard handlers. The menu closes
// whenever the shown message changes (messageId), which is the reader's per-message reset for this island.
export function useTagMenu(messageId: string | undefined) {
    const [open, setOpen] = useState(false)
    const menuRef = useRef<HTMLDivElement>(null)
    // triggerRef is the Colour button and rowRef the swatch row; openedByKey records a keyboard open so
    // focus lands on the first swatch (a mouse open leaves focus on the trigger).
    const triggerRef = useRef<HTMLButtonElement>(null)
    const rowRef = useRef<HTMLDivElement>(null)
    const openedByKey = useRef(false)

    // Close on an outside click while the menu is open.
    useEffect(() => {
        if (!open) {
            return
        }
        const onDocClick = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
                setOpen(false)
            }
        }
        document.addEventListener('mousedown', onDocClick)
        return () => document.removeEventListener('mousedown', onDocClick)
    }, [open])

    // Close the menu whenever the selected message changes.
    useEffect(() => {
        setOpen(false)
    }, [messageId])

    // When the colour menu opens by keyboard, move focus to the first swatch so Left/Right walk them.
    useEffect(() => {
        if (open && openedByKey.current) {
            rowRef.current?.querySelector<HTMLButtonElement>('.tag-colour')?.focus()
        }
        if (!open) {
            openedByKey.current = false
        }
    }, [open])

    const toggle = () => setOpen((v) => !v)

    // onTriggerKeyDown drops the swatch menu open from the Colour button: Down, Enter or Space open it and
    // land on the first swatch, from where Left/Right walk them. Up or Escape retract it.
    const onTriggerKeyDown = (e: ReactKeyboardEvent<HTMLButtonElement>) => {
        if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
            e.preventDefault()
            e.stopPropagation()
            openedByKey.current = true
            setOpen(true)
        } else if (e.key === 'ArrowUp' || e.key === 'Escape') {
            e.preventDefault()
            e.stopPropagation()
            setOpen(false)
        }
    }

    // onRowKeyDown is the roving-focus handler for the swatch row. Left/Right walk the swatches, wrapping.
    // Escape or Up closes back to the Colour button. Tab and Shift+Tab are left to bubble so the window ring
    // steps out of the menu (they exit it). Enter/Space toggle a swatch via its own click. Up/Down are
    // swallowed so they never leak to the message list.
    const onRowKeyDown = (e: ReactKeyboardEvent<HTMLDivElement>) => {
        const swatches = Array.from(
            rowRef.current?.querySelectorAll<HTMLButtonElement>('.tag-colour') ?? [],
        )
        if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
            e.preventDefault()
            e.stopPropagation()
            if (swatches.length === 0) {
                return
            }
            const at = swatches.indexOf(document.activeElement as HTMLButtonElement)
            const next = e.key === 'ArrowRight'
                ? (at + 1) % swatches.length
                : (at - 1 + swatches.length) % swatches.length
            swatches[next].focus()
        } else if (e.key === 'Escape' || e.key === 'ArrowUp') {
            e.preventDefault()
            e.stopPropagation()
            setOpen(false)
            triggerRef.current?.focus()
        } else if (e.key === 'ArrowDown') {
            e.preventDefault()
            e.stopPropagation()
        } else if (e.key === 'Tab') {
            // Exit: close the menu and hand focus back to the Colour button, then let the window handler
            // step the ring on from there.
            setOpen(false)
            triggerRef.current?.focus()
        }
    }

    return {open, menuRef, triggerRef, rowRef, toggle, onTriggerKeyDown, onRowKeyDown}
}
