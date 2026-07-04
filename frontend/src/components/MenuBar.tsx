import {useEffect, useRef, useState} from 'react'

interface MenuBarProps {
    onShowAbout: () => void
    onShowLicence: () => void
    onCheckUpdates: () => void
}

// MenuBar is the Help menu at the end of the title tray: an info button that opens a small dropdown.
export function MenuBar(props: MenuBarProps) {
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
        document.addEventListener('mousedown', onDocClick)
        return () => document.removeEventListener('mousedown', onDocClick)
    }, [open])

    const choose = (action: () => void) => {
        setOpen(false)
        action()
    }

    return (
        <div className="menu" ref={menuRef}>
            <button
                className={'menu-title' + (open ? ' active' : '')}
                data-tip="Help"
                aria-label="Help"
                onClick={() => setOpen((v) => !v)}
            >
                {'\u{2139}\u{FE0F}'}
            </button>
            {open && (
                <div className="menu-dropdown" role="menu">
                    <button className="menu-item" role="menuitem" onClick={() => choose(props.onShowAbout)}>
                        About PigeonPost
                    </button>
                    <button className="menu-item" role="menuitem" onClick={() => choose(props.onShowLicence)}>
                        Licence
                    </button>
                    <button className="menu-item" role="menuitem" onClick={() => choose(props.onCheckUpdates)}>
                        Check for Updates
                    </button>
                </div>
            )}
        </div>
    )
}
