import {useEffect, useRef, useState} from 'react'
import type {KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent} from 'react'
import {isTextEntry} from '../editClipboard'

// MenuItem is one entry in a dropdown menu. icon is an optional leading emoji; disabled greys the item
// out and blocks its action; checked, when defined, renders the item as a toggle that shows a tick when
// true.
export interface MenuItem {
    label: string
    // onClick is the leaf action. A submenu parent and a separator have none.
    onClick?: () => void
    icon?: string
    disabled?: boolean
    checked?: boolean
    // shortcut is the accelerator hint shown right-aligned in the item (for example "Ctrl+N" or "F9"). It
    // is display only; the key itself is wired centrally in App so it works whether or not the menu is open.
    shortcut?: string
    // hintOnly marks the shortcut as display only even for the central wiring: the key is owned elsewhere
    // (natively by the focused field for Ctrl+X/C/V, by the list keyboard handler for Del and Ctrl+A), so
    // binding it again would double-fire.
    hintOnly?: boolean
    // altShortcuts are extra accelerators that fire the item without being displayed (Redo accepts
    // Ctrl+Shift+Z alongside the shown hint).
    altShortcuts?: string[]
    // skipInText suppresses the accelerator while a text surface has focus, so a key the field handles
    // natively (Ctrl+Z as text undo) is not hijacked by the menu action.
    skipInText?: boolean
    // submenu, when present, makes this item a flyout parent: it has no action of its own; opening it
    // reveals these child items to the side.
    submenu?: MenuItem[]
    // separator, when true, renders a divider in place of an item (the other fields are ignored).
    separator?: boolean
    // swatch, when set, draws a small colour square before the label (used by the colour tag list).
    swatch?: string
}

interface MenuProps {
    // title is the trigger's tooltip and accessible label (File, Edit, View, Mail, Help).
    title: string
    // icon is the emoji shown on the trigger button.
    icon: string
    items: MenuItem[]
    // align sets which edge of the trigger the dropdown lines up with. Menus on the left of the tray open
    // left-aligned (rightwards); a menu on the right stays right-aligned so it does not run off screen.
    align?: 'left' | 'right'
}

// directItems returns the enabled, focusable item buttons that belong directly to a dropdown or submenu
// panel, skipping any nested one level deeper inside a submenu. A submenu parent's button sits inside its
// own .menu-sub-wrap, so both shapes are matched; the document-order result keeps them in view order.
function directItems(panel: HTMLElement | null): HTMLButtonElement[] {
    if (!panel) {
        return []
    }
    return Array.from(panel.querySelectorAll<HTMLButtonElement>(
        ':scope > .menu-item:not([disabled]), :scope > .menu-sub-wrap > .menu-item:not([disabled])',
    ))
}

// keepTextFocus stops a menu click from stealing focus while a text field holds it, so the selection
// Cut / Copy / Paste act on survives the click. Anywhere else the native focus move is kept: focusing
// the trigger is what lets Down step into the items after a mouse open.
function keepTextFocus(e: ReactMouseEvent): void {
    if (isTextEntry(document.activeElement)) {
        e.preventDefault()
    }
}

// MenuItemView renders one entry: a divider, a flyout parent (SubMenuItem) or a leaf button.
function MenuItemView({item, onChoose}: {item: MenuItem; onChoose: (item: MenuItem) => void}) {
    if (item.separator) {
        return <div className="menu-sep" role="separator"/>
    }
    if (item.submenu) {
        return <SubMenuItem item={item} onChoose={onChoose}/>
    }
    return (
        <button
            className="menu-item"
            role={item.checked === undefined ? 'menuitem' : 'menuitemcheckbox'}
            aria-checked={item.checked}
            disabled={item.disabled}
            onMouseDown={keepTextFocus}
            onClick={() => onChoose(item)}
        >
            <span className="menu-item-gutter" aria-hidden="true">
                {item.checked !== undefined ? (item.checked ? '✓' : '') : item.icon ?? ''}
            </span>
            {item.swatch !== undefined && (
                <span className="menu-item-swatch" aria-hidden="true" style={{backgroundColor: item.swatch}}/>
            )}
            <span className="menu-item-label">{item.label}</span>
            {item.shortcut !== undefined && (
                <span className="menu-item-shortcut" aria-hidden="true">{item.shortcut}</span>
            )}
        </button>
    )
}

// SubMenuItem is a flyout parent inside a dropdown. It opens on hover. For the keyboard, Right or Enter
// opens it and moves focus to the first child. Inside the open panel Up and Down (wrapping) plus
// Home and End walk the children; Left or Escape closes the panel and returns focus to the parent. Every
// handled key is consumed so the containing list's handler does not also act on it.
function SubMenuItem({item, onChoose}: {item: MenuItem; onChoose: (item: MenuItem) => void}) {
    const [open, setOpen] = useState(false)
    const btnRef = useRef<HTMLButtonElement>(null)
    const panelRef = useRef<HTMLDivElement>(null)

    const openAndFocusFirst = () => {
        if (item.disabled) {
            return
        }
        setOpen(true)
        // Focus the first child once the panel has rendered.
        requestAnimationFrame(() => directItems(panelRef.current)[0]?.focus())
    }
    const closeToParent = () => {
        setOpen(false)
        btnRef.current?.focus()
    }

    const onParentKey = (e: ReactKeyboardEvent) => {
        if (e.key === 'ArrowRight' || e.key === 'Enter') {
            e.preventDefault()
            e.stopPropagation()
            openAndFocusFirst()
        }
    }

    const onPanelKey = (e: ReactKeyboardEvent) => {
        const list = directItems(panelRef.current)
        if (e.key === 'ArrowLeft' || e.key === 'Escape') {
            e.preventDefault()
            e.stopPropagation()
            closeToParent()
            return
        }
        if (list.length === 0) {
            return
        }
        const idx = list.indexOf(document.activeElement as HTMLButtonElement)
        if (e.key === 'ArrowDown') {
            e.preventDefault()
            e.stopPropagation()
            list[(idx + 1 + list.length) % list.length].focus()
        } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            e.stopPropagation()
            list[(idx - 1 + list.length) % list.length].focus()
        } else if (e.key === 'Home') {
            e.preventDefault()
            e.stopPropagation()
            list[0].focus()
        } else if (e.key === 'End') {
            e.preventDefault()
            e.stopPropagation()
            list[list.length - 1].focus()
        }
    }

    return (
        <div
            className="menu-sub-wrap"
            onMouseEnter={() => !item.disabled && setOpen(true)}
            onMouseLeave={() => setOpen(false)}
        >
            <button
                ref={btnRef}
                className="menu-item"
                role="menuitem"
                aria-haspopup="menu"
                aria-expanded={open}
                disabled={item.disabled}
                onMouseDown={keepTextFocus}
                onKeyDown={onParentKey}
            >
                <span className="menu-item-gutter" aria-hidden="true">{item.icon ?? ''}</span>
                <span className="menu-item-label">{item.label}</span>
                <span className="menu-sub-arrow" aria-hidden="true">▸</span>
            </button>
            {open && (
                <div className="menu-submenu" role="menu" ref={panelRef} onKeyDown={onPanelKey}>
                    {item.submenu!.map((child, i) => (
                        <MenuItemView key={i} item={child} onChoose={onChoose}/>
                    ))}
                </div>
            )}
        </div>
    )
}

// HOVER_CLOSE_DELAY_MS is the grace the pointer gets after leaving a hover-opened menu before it
// closes: long enough to travel diagonally from the trigger into the dropdown (or between them and
// back) without the menu slamming shut, short enough that the menu still feels dismissed on leave.
export const HOVER_CLOSE_DELAY_MS = 200

// Menu is a title-tray dropdown: an emoji trigger that opens a list of items. Hovering the trigger
// opens it too (the dropdown is headed with the menu's name, since the emoji alone does not carry
// it), and it closes once the pointer has left both the trigger and the open dropdown. It also
// closes on an outside click, on Escape and after a leaf item is chosen. It backs the File, Edit,
// View, Mail and Help menus so they all look and behave the same. Keyboard: Down, Enter or Space on
// the trigger opens the menu and moves focus to the first item; Up and Down (wrapping) plus Home
// and End walk the items; Right opens a submenu (or collapses a leaf); Escape, Tab or Left
// collapses back to the trigger so the window focus ring carries on from there.
export function Menu({title, icon, items, align = 'right'}: MenuProps) {
    const [open, setOpen] = useState(false)
    const menuRef = useRef<HTMLDivElement>(null)
    const triggerRef = useRef<HTMLButtonElement>(null)
    const dropdownRef = useRef<HTMLDivElement>(null)
    // openedByKeyboardRef records that the menu was opened with the keyboard, so the first item is focused
    // on open. A mouse open leaves focus on the trigger (no stray focus ring); Down from there still steps
    // into the items.
    const openedByKeyboardRef = useRef(false)
    // hoverCloseTimer holds the pending close scheduled when the pointer leaves the menu, cancelled
    // if it comes back within the grace period.
    const hoverCloseTimer = useRef<number | null>(null)

    const cancelHoverClose = () => {
        if (hoverCloseTimer.current !== null) {
            window.clearTimeout(hoverCloseTimer.current)
            hoverCloseTimer.current = null
        }
    }

    useEffect(() => cancelHoverClose, [])

    // When the menu opens by keyboard, move focus to its first item so the keyboard lands inside the
    // dropdown. Reset the flag once closed.
    useEffect(() => {
        if (open && openedByKeyboardRef.current) {
            directItems(dropdownRef.current)[0]?.focus()
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

    // choose runs a leaf item's action and closes the whole menu. A submenu parent has no action, so it is
    // never passed here.
    const choose = (item: MenuItem) => {
        if (item.disabled || !item.onClick) {
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
                directItems(dropdownRef.current)[0]?.focus()
            } else {
                openedByKeyboardRef.current = true
                setOpen(true)
            }
        }
    }

    // onDropdownKeyDown owns navigation at the top level: Up and Down (wrapping) and Home and End move
    // between items; Escape, Tab or a horizontal arrow collapses the menu to its trigger so the window
    // focus ring carries on from there. Right on a submenu parent is caught by that item first (it opens
    // the submenu), so Right only reaches here on a leaf. Every handled key is consumed.
    const onDropdownKeyDown = (e: ReactKeyboardEvent) => {
        const list = directItems(dropdownRef.current)
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
        <div
            className="menu"
            ref={menuRef}
            onMouseEnter={() => {
                cancelHoverClose()
                setOpen(true)
            }}
            onMouseLeave={() => {
                cancelHoverClose()
                hoverCloseTimer.current = window.setTimeout(() => setOpen(false), HOVER_CLOSE_DELAY_MS)
            }}
        >
            <button
                ref={triggerRef}
                className={'menu-title' + (open ? ' active' : '')}
                data-tip={title}
                aria-label={title}
                aria-haspopup="menu"
                aria-expanded={open}
                onMouseDown={keepTextFocus}
                onClick={() => setOpen((v) => !v)}
                onKeyDown={onTriggerKeyDown}
            >
                {icon}
            </button>
            {open && (
                <div
                    className={'menu-dropdown' + (align === 'left' ? ' align-left' : '')}
                    role="menu"
                    ref={dropdownRef}
                    onKeyDown={onDropdownKeyDown}
                >
                    <div className="menu-header" aria-hidden="true">{title}</div>
                    {items.map((item, i) => (
                        <MenuItemView key={i} item={item} onChoose={choose}/>
                    ))}
                </div>
            )}
        </div>
    )
}
