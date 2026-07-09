import {useEffect, useRef, useState} from 'react'
import type {KeyboardEvent as ReactKeyboardEvent} from 'react'

// MenuItem is one entry in a dropdown menu. icon is an optional leading emoji; disabled greys the item
// out and blocks its action; checked, when defined, renders the item as a toggle that shows a tick when
// true.
export interface MenuItem {
    label: string
    onClick: () => void
    icon?: string
    disabled?: boolean
    checked?: boolean
    // shortcut is the accelerator hint shown right-aligned in the item (for example "Ctrl+N" or "F9"). It
    // is display only; the key itself is wired centrally in App so it works whether or not the menu is open.
    shortcut?: string
}

interface MenuProps {
    // title is the trigger's tooltip and accessible label (File, Edit, View, Help).
    title: string
    // icon is the emoji shown on the trigger button.
    icon: string
    items: MenuItem[]
}

// Menu is a title-tray dropdown: an emoji trigger that opens a small list of items. It closes on an
// outside click, on Escape and after an item is chosen. It backs the File, Edit, View and Help menus so
// they all look and behave the same. Keyboard: Down, Enter or Space on the trigger opens the menu and
// moves focus to the first item; Up and Down (wrapping) plus Home and End walk the items; Escape, Tab or a
// horizontal arrow collapses back to the trigger so the window focus ring carries on from there.
export function Menu({title, icon, items}: MenuProps) {
    const [open, setOpen] = useState(false)
    const menuRef = useRef<HTMLDivElement>(null)
    const triggerRef = useRef<HTMLButtonElement>(null)
    const dropdownRef = useRef<HTMLDivElement>(null)
    // openedByKeyboardRef records that the menu was opened with the keyboard, so the first item is focused
    // on open. A mouse open leaves focus on the trigger (no stray focus ring); Down from there still
    // steps into the items.
    const openedByKeyboardRef = useRef(false)

    // enabledItems returns the dropdown's focusable (not disabled) item buttons in order.
    const enabledItems = (): HTMLButtonElement[] =>
        dropdownRef.current
            ? Array.from(dropdownRef.current.querySelectorAll<HTMLButtonElement>('.menu-item:not([disabled])'))
            : []

    // When the menu opens by keyboard, move focus to its first item so the keyboard lands inside the
    // dropdown. Reset the flag once closed.
    useEffect(() => {
        if (open && openedByKeyboardRef.current) {
            enabledItems()[0]?.focus()
        }
        if (!open) {
            openedByKeyboardRef.current = false
        }
    }, [open])

    // Close on an outside click while open.
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

    const closeToTrigger = () => {
        setOpen(false)
        triggerRef.current?.focus()
    }

    const choose = (item: MenuItem) => {
        if (item.disabled) {
            return
        }
        setOpen(false)
        item.onClick()
    }

    // onTriggerKeyDown opens the menu on Down, Enter or Space. Enter and Space also fire the trigger's
    // native click (which toggles open), so the open is handled here and the key is consumed to stop the
    // click from immediately toggling it back shut. When already open, Down steps into the items.
    const onTriggerKeyDown = (e: ReactKeyboardEvent) => {
        if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
            e.preventDefault()
            e.stopPropagation()
            if (open) {
                enabledItems()[0]?.focus()
            } else {
                openedByKeyboardRef.current = true
                setOpen(true)
            }
        }
    }

    // onDropdownKeyDown owns navigation while the menu is open: Up and Down (wrapping) and Home and End move
    // between items, Escape closes back to the trigger; Tab or a horizontal arrow collapses the menu to
    // its trigger so the window focus ring carries on from there. Every handled key is consumed so the
    // window key handler behind the menu never also acts on it.
    const onDropdownKeyDown = (e: ReactKeyboardEvent) => {
        const list = enabledItems()
        if (list.length === 0) {
            return
        }
        const index = list.indexOf(document.activeElement as HTMLButtonElement)
        if (e.key === 'ArrowDown') {
            e.preventDefault()
            e.stopPropagation()
            list[(index + 1 + list.length) % list.length].focus()
        } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            e.stopPropagation()
            list[(index - 1 + list.length) % list.length].focus()
        } else if (e.key === 'Home') {
            e.preventDefault()
            e.stopPropagation()
            list[0].focus()
        } else if (e.key === 'End') {
            e.preventDefault()
            e.stopPropagation()
            list[list.length - 1].focus()
        } else if (e.key === 'Escape' || e.key === 'Tab' || e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
            e.preventDefault()
            e.stopPropagation()
            closeToTrigger()
        }
    }

    return (
        <div className="menu" ref={menuRef}>
            <button
                ref={triggerRef}
                className={'menu-title' + (open ? ' active' : '')}
                data-tip={title}
                aria-label={title}
                aria-haspopup="menu"
                aria-expanded={open}
                onClick={() => setOpen((v) => !v)}
                onKeyDown={onTriggerKeyDown}
            >
                {icon}
            </button>
            {open && (
                <div className="menu-dropdown" role="menu" ref={dropdownRef} onKeyDown={onDropdownKeyDown}>
                    {items.map((item) => (
                        <button
                            key={item.label}
                            className="menu-item"
                            role={item.checked === undefined ? 'menuitem' : 'menuitemcheckbox'}
                            aria-checked={item.checked}
                            disabled={item.disabled}
                            onClick={() => choose(item)}
                        >
                            {item.checked !== undefined && (
                                <span className="menu-item-check" aria-hidden="true">
                                    {item.checked ? '✓' : ''}
                                </span>
                            )}
                            {item.icon !== undefined && (
                                <span className="menu-item-icon" aria-hidden="true">{item.icon}</span>
                            )}
                            <span>{item.label}</span>
                            {item.shortcut !== undefined && (
                                <span className="menu-item-shortcut" aria-hidden="true">{item.shortcut}</span>
                            )}
                        </button>
                    ))}
                </div>
            )}
        </div>
    )
}
