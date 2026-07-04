import {useEffect, useRef, useState} from 'react'
import {Theme} from '../theme'

interface MenuBarProps {
    theme: Theme
    onToggleTheme: () => void
    previewEnabled: boolean
    onTogglePreview: () => void
    onShowAbout: () => void
    onShowLicence: () => void
    onCheckUpdates: () => void
}

export function MenuBar(props: MenuBarProps) {
    const {theme} = props
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
        <div className="menubar">
            <button
                className="icon-btn"
                title={props.previewEnabled ? 'Hide the reading pane' : 'Show the reading pane'}
                aria-pressed={props.previewEnabled}
                onClick={props.onTogglePreview}
            >
                {props.previewEnabled ? '◫' : '▯'}
            </button>
            <button
                className="icon-btn"
                title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                onClick={props.onToggleTheme}
            >
                {theme === 'dark' ? '☀️' : '\u{1F319}'}
            </button>
            <div className="menu" ref={menuRef}>
                <button className={'menu-title' + (open ? ' active' : '')} onClick={() => setOpen((v) => !v)}>
                    Help
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
        </div>
    )
}
