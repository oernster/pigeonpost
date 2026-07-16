import {useEffect, useLayoutEffect, useRef, useState} from 'react'
import {Folder} from '../api'

interface FolderContextMenuProps {
    folder: Folder
    x: number
    y: number
    // canPaste is whether the message clipboard holds anything; the menu exists chiefly so a cut
    // or copied selection can be pasted straight onto a folder without opening it first.
    canPaste: boolean
    onPaste: (folder: Folder) => void
    onClose: () => void
}

// Keep the menu at least this far inside the viewport edges when clamping its position.
const MENU_MARGIN = 8

// FolderContextMenu is the folder row's right-click menu, the folder-side counterpart of the
// message context menu: Paste files the message clipboard's cut or copied emails into this folder.
// It shares the .context-menu classes (and so the same styling and the accelerator suppression)
// and the same dismiss-and-clamp behaviour.
export function FolderContextMenu(props: FolderContextMenuProps) {
    const {folder, onClose} = props
    const ref = useRef<HTMLDivElement>(null)
    const [pos, setPos] = useState({x: props.x, y: props.y})

    // Dismiss on an outside click or the Escape key.
    useEffect(() => {
        const onDown = (e: MouseEvent) => {
            if (ref.current && !ref.current.contains(e.target as Node)) {
                onClose()
            }
        }
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                onClose()
            }
        }
        document.addEventListener('mousedown', onDown)
        document.addEventListener('keydown', onKey)
        return () => {
            document.removeEventListener('mousedown', onDown)
            document.removeEventListener('keydown', onKey)
        }
    }, [onClose])

    // After the first render, nudge the menu back inside the viewport if the cursor was near an edge.
    useLayoutEffect(() => {
        const el = ref.current
        if (!el) {
            return
        }
        const rect = el.getBoundingClientRect()
        let nx = props.x
        let ny = props.y
        if (nx + rect.width > window.innerWidth - MENU_MARGIN) {
            nx = Math.max(MENU_MARGIN, window.innerWidth - rect.width - MENU_MARGIN)
        }
        if (ny + rect.height > window.innerHeight - MENU_MARGIN) {
            ny = Math.max(MENU_MARGIN, window.innerHeight - rect.height - MENU_MARGIN)
        }
        if (nx !== pos.x || ny !== pos.y) {
            setPos({x: nx, y: ny})
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [props.x, props.y])

    return (
        <div
            ref={ref}
            className="context-menu"
            role="menu"
            style={{left: pos.x, top: pos.y}}
            onClick={(e) => e.stopPropagation()}
        >
            <div className="context-header">{folder.name}</div>
            <div className="context-sep"/>
            <button
                className="context-item"
                role="menuitem"
                disabled={!props.canPaste}
                onClick={() => {
                    props.onPaste(folder)
                    onClose()
                }}
            >
                Paste
            </button>
        </div>
    )
}
