import {useEffect, useRef, useState} from 'react'

// MenuItem is one entry in a dropdown menu. icon is an optional leading emoji; disabled greys the item
// out and blocks its action; checked, when defined, renders the item as a toggle that shows a tick when
// true.
export interface MenuItem {
    label: string
    onClick: () => void
    icon?: string
    disabled?: boolean
    checked?: boolean
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
// they all look and behave the same.
export function Menu({title, icon, items}: MenuProps) {
    const [open, setOpen] = useState(false)
    const menuRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        if (!open) {
            return
        }
        const onDocClick = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
                setOpen(false)
            }
        }
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                setOpen(false)
            }
        }
        document.addEventListener('mousedown', onDocClick)
        document.addEventListener('keydown', onKey)
        return () => {
            document.removeEventListener('mousedown', onDocClick)
            document.removeEventListener('keydown', onKey)
        }
    }, [open])

    const choose = (item: MenuItem) => {
        if (item.disabled) {
            return
        }
        setOpen(false)
        item.onClick()
    }

    return (
        <div className="menu" ref={menuRef}>
            <button
                className={'menu-title' + (open ? ' active' : '')}
                data-tip={title}
                aria-label={title}
                aria-haspopup="menu"
                aria-expanded={open}
                onClick={() => setOpen((v) => !v)}
            >
                {icon}
            </button>
            {open && (
                <div className="menu-dropdown" role="menu">
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
                        </button>
                    ))}
                </div>
            )}
        </div>
    )
}
