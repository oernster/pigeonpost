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
    // canManageFolders is false for POP3 accounts, which have no server-side folders; the New
    // subfolder and Delete folder entries are left out entirely for them.
    canManageFolders: boolean
    onNewSubfolder: (folder: Folder) => void
    // onRenameFolder opens the same prompt the row's pencil does.
    onRenameFolder: (folder: Folder) => void
    // onDeleteFolder opens the same confirmation the row's red cross does; the menu never deletes
    // anything itself.
    onDeleteFolder: (folder: Folder) => void
    onClose: () => void
}

// Keep the menu at least this far inside the viewport edges when clamping its position.
const MENU_MARGIN = 8

// ICON_NEW, ICON_RENAME and ICON_DELETE are the same glyphs the folder surfaces already use: the
// plus on the Folders heading, the pencil and the cross on the row's hover toolbar. The menu wears
// them so an entry and the button that does the same thing are recognisably one action.
const ICON_NEW = '+'
const ICON_RENAME = '\u270E'
const ICON_DELETE = '\u00D7'

// FolderContextMenu is the folder row's right-click menu, the folder-side counterpart of the
// message context menu: New subfolder creates a folder directly under this one, Rename and Delete
// folder do what the row's pencil and cross do and Paste files the message clipboard's cut or
// copied emails into this folder. Rename and Delete are offered on exactly the folders whose row
// carries those buttons, custom ones, so a well-known folder (Inbox, Sent, Trash and the rest)
// cannot be renamed or deleted from here either. Each entry opens the same prompt or confirmation
// its button does rather than acting on the click. Paste is the one entry with no counterpart on
// the row, so it sits in its own group with an empty icon gutter rather than a borrowed glyph.
// It shares the .context-menu classes (and so the same styling and the accelerator suppression)
// and the same dismiss-and-clamp behaviour.
export function FolderContextMenu(props: FolderContextMenuProps) {
    const {folder, onClose} = props
    // The pencil and cross on the row appear on custom folders only, so the two entries that mirror
    // them match exactly. A POP3 account has no server-side folders to change at all.
    const canEditFolder = props.canManageFolders && folder.kind === 'custom'
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
            {props.canManageFolders && (
                <button
                    className="context-item"
                    role="menuitem"
                    onClick={() => {
                        props.onNewSubfolder(folder)
                        onClose()
                    }}
                >
                    <span className="context-item-icon" aria-hidden="true">{ICON_NEW}</span>
                    New subfolder
                </button>
            )}
            {canEditFolder && (
                <button
                    className="context-item"
                    role="menuitem"
                    onClick={() => {
                        props.onRenameFolder(folder)
                        onClose()
                    }}
                >
                    <span className="context-item-icon" aria-hidden="true">{ICON_RENAME}</span>
                    Rename folder
                </button>
            )}
            {/* The rule above Paste belongs to the folder group, so a POP3 account (which is offered no
                folder actions at all) does not get two rules with nothing between them. */}
            {props.canManageFolders && <div className="context-sep"/>}
            <button
                className="context-item"
                role="menuitem"
                disabled={!props.canPaste}
                onClick={() => {
                    props.onPaste(folder)
                    onClose()
                }}
            >
                <span className="context-item-icon" aria-hidden="true"/>
                Paste
            </button>
            {canEditFolder && (
                <>
                    <div className="context-sep"/>
                    <button
                        className="context-item danger"
                        role="menuitem"
                        onClick={() => {
                            props.onDeleteFolder(folder)
                            onClose()
                        }}
                    >
                        <span className="context-item-icon" aria-hidden="true">{ICON_DELETE}</span>
                        Delete folder
                    </button>
                </>
            )}
        </div>
    )
}
