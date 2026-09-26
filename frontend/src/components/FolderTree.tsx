import {type CSSProperties} from 'react'
import {Folder} from '../api'
import {folderIcon} from '../icons'
import {
    detectSeparator,
    leafName,
    ancestorPaths,
    descendantUnread,
    nearestParentPath,
    orderFolders,
} from '../folderPaths'
import {usePersistedFolderState} from '../hooks/usePersistedFolderState'
import {useFolderTreeDrag} from '../hooks/useFolderTreeDrag'

interface FolderTreeProps {
    folders: Folder[]
    selectedFolder: string
    selectedAccount: string
    onSelectFolder: (id: string) => void
    onRenameFolder: (folder: Folder) => void
    // onNewSubfolder opens the create prompt with this folder as the parent, the row's counterpart of
    // the right-click menu's New subfolder entry.
    onNewSubfolder: (folder: Folder) => void
    onReparentFolder: (folderId: string, newParentId: string) => void
    onDeleteFolder: (folder: Folder) => void
    // onDropMessage moves a dragged message into the folder and reports whether the drop was taken.
    onDropMessage: (messageId: string, folderId: string) => boolean
    // onFolderContextMenu opens the folder right-click menu (Paste and friends) at the cursor.
    onFolderContextMenu: (folder: Folder, x: number, y: number) => void
}

// FOLDER_INDENT_STEP_PX is the left indent added per tree depth (and the base indent of a top-level row),
// so a folder at depth d sits at (d + 1) steps. The same value drives the drag insertion line's indent.
const FOLDER_INDENT_STEP_PX = 14

// FolderTree renders the folders as a nested, collapsible tree derived from their paths. A collapsed parent
// rolls the unread hidden in its subtree up onto its own badge (outlined, so a rolled-up count reads
// differently from a folder's own unread) and the badge reverts to the folder's own count on expand.
// Custom folders can be dragged to reparent them (a server move) or reorder amongst their siblings (a
// local, persisted order).
// Both the collapsed state and the local order are kept per account in localStorage, so they survive
// restarts. The folder-path and ordering helpers live in ../folderPaths, keeping the pure tree logic out of
// this component.
export function FolderTree(props: FolderTreeProps) {
    const {folders, selectedFolder, selectedAccount} = props
    const {collapsed, order, toggle, persistOrder, expand} = usePersistedFolderState(selectedAccount)
    const paths = folders.map((f) => f.path)
    const sep = detectSeparator(paths)
    const existing = new Set(paths)
    // byPath maps a folder path to its id, so a drop that reparents under a folder path can name the id.
    const byPath = new Map(folders.map((f) => [f.path, f.id]))
    const hasChildren = (path: string) => paths.some((p) => p.startsWith(path + sep))
    const ordered = orderFolders(folders, sep, order)
    // A folder is visible only when none of its ancestors are collapsed.
    const visible = ordered.filter((f) => ancestorPaths(f.path, sep).every((a) => !collapsed.has(a)))
    // The folder list is a single focus-ring stop: only one folder is tabbable (the selected one; the
    // first when none is selected). Up/Down move between folders from there.
    const tabStopId = selectedFolder || (visible.length > 0 ? visible[0].id : '')

    // customSiblingPaths returns the paths of the custom folders under parentPath in their current display
    // order, which is the sibling group a reorder or a gap reparent splices the moved folder into.
    const customSiblingPaths = (parentPath: string): string[] =>
        ordered
            .filter((f) => f.kind === 'custom' && nearestParentPath(f.path, existing, sep) === parentPath)
            .map((f) => f.path)

    // Everything the tree does under a drag (the cues, spring-loading, the drop itself and the flash) is
    // its own hook.
    const drag = useFolderTreeDrag({
        folders, sep, existing, byPath, collapsed, order, expand, persistOrder, hasChildren, customSiblingPaths,
        onReparentFolder: props.onReparentFolder, onDropMessage: props.onDropMessage,
    })
    const {dragOverId, draggingFolderId, droppedId, folderDrop} = drag

    return (
        <ul className="list" data-folder-list="">
            {visible.map((folder) => {
                const leaf = leafName(folder.path, sep)
                const depth = ancestorPaths(folder.path, sep).length
                const parent = hasChildren(folder.path)
                const isCollapsed = collapsed.has(folder.path)
                // A collapsed parent's children are not rendered, so their unread would otherwise vanish
                // from the sidebar (while still counting toward the account badge). Roll it up onto the
                // collapsed row; an expanded parent shows only its own unread, its children showing theirs.
                const hiddenUnread = parent && isCollapsed ? descendantUnread(folder, folders, sep) : 0
                const badgeCount = folder.unread + hiddenUnread
                const rowIndentPx = (depth + 1) * FOLDER_INDENT_STEP_PX
                const rowStyle = {
                    paddingLeft: rowIndentPx,
                    ['--row-indent']: `${rowIndentPx}px`,
                } as CSSProperties
                return (
                    <li
                        key={folder.id}
                        data-folder-id={folder.id}
                        className={
                            'list-item folder' +
                            (folder.id === selectedFolder ? ' selected' : '') +
                            (folder.id === dragOverId ? ' drag-over' : '') +
                            (folder.id === draggingFolderId ? ' dragging' : '') +
                            (folder.id === droppedId ? ' drop-landed' : '') +
                            (folderDrop && folderDrop.folderId === folder.id ? ' drag-' + folderDrop.zone : '')
                        }
                        draggable={folder.kind === 'custom'}
                        style={rowStyle}
                        tabIndex={folder.id === tabStopId ? 0 : -1}
                        onClick={() => props.onSelectFolder(folder.id)}
                        onContextMenu={(e) => {
                            e.preventDefault()
                            props.onFolderContextMenu(folder, e.clientX, e.clientY)
                        }}
                        onKeyDown={(e) => {
                            // Only the row itself drives navigation here; a key while focus is on a child
                            // button (the collapse toggle, rename or delete) is left to that button. The
                            // collapse toggle stays out of the Tab order (Left/Right drives it); the selected
                            // row's rename and delete ARE tabbable, so Tab steps the row then those two then
                            // out of the list, while Up/Down move between folders.
                            if (e.target !== e.currentTarget) {
                                return
                            }
                            const li = e.currentTarget
                            const moveFocus = (forward: boolean) => {
                                let sibling = forward ? li.nextElementSibling : li.previousElementSibling
                                if (!sibling && li.parentElement) {
                                    sibling = forward
                                        ? li.parentElement.firstElementChild
                                        : li.parentElement.lastElementChild
                                }
                                if (sibling instanceof HTMLElement) {
                                    sibling.focus()
                                    const id = sibling.getAttribute('data-folder-id')
                                    if (id) {
                                        props.onSelectFolder(id)
                                    }
                                }
                            }
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                props.onSelectFolder(folder.id)
                                return
                            }
                            // Up/Down move between folders, wrapping at the ends.
                            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                                e.preventDefault()
                                e.stopPropagation()
                                moveFocus(e.key === 'ArrowDown')
                                return
                            }
                            // Right expands a collapsed parent, else moves to the next folder; Left collapses
                            // an expanded parent, else moves to the previous. Consumed here (not bubbled to the
                            // ring) so the arrows navigate the tree and Tab is what leaves the list.
                            if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                                e.preventDefault()
                                e.stopPropagation()
                                if (e.key === 'ArrowRight' && parent && isCollapsed) {
                                    toggle(folder.path)
                                    return
                                }
                                if (e.key === 'ArrowLeft' && parent && !isCollapsed) {
                                    toggle(folder.path)
                                    return
                                }
                                moveFocus(e.key === 'ArrowRight')
                                return
                            }
                        }}
                        {...drag.rowDrag(folder)}
                    >
                        <span className="folder-name">
                            {parent ? (
                                <button
                                    type="button"
                                    className="folder-toggle"
                                    tabIndex={-1}
                                    aria-label={isCollapsed ? `Expand ${leaf}` : `Collapse ${leaf}`}
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        toggle(folder.path)
                                    }}
                                >
                                    {isCollapsed ? '\u25B6\uFE0E' : '\u25BC\uFE0E'}
                                </button>
                            ) : (
                                <span className="folder-toggle-spacer"/>
                            )}
                            <span className="folder-icon">
                                <img src={folderIcon[folder.kind] ?? folderIcon.custom} alt="" draggable={false}/>
                            </span>
                            {leaf}
                        </span>
                        {badgeCount > 0 && (
                            <span
                                className={'badge' + (hiddenUnread > 0 ? ' badge-rollup' : '')}
                                title={hiddenUnread > 0 ? `${badgeCount} unread including subfolders` : undefined}
                            >
                                {badgeCount}
                            </span>
                        )}
                        {folder.kind === 'custom' && (
                            <span className="account-actions">
                                <button
                                    className="account-action"
                                    tabIndex={folder.id === tabStopId ? 0 : -1}
                                    aria-label={`New subfolder in ${folder.name}`}
                                    title="New subfolder"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        props.onNewSubfolder(folder)
                                    }}
                                >
                                    &#43;
                                </button>
                                <button
                                    className="account-action"
                                    tabIndex={folder.id === tabStopId ? 0 : -1}
                                    aria-label={`Rename ${folder.name}`}
                                    title="Rename folder"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        props.onRenameFolder(folder)
                                    }}
                                >
                                    &#9998;
                                </button>
                                <button
                                    className="account-action delete"
                                    tabIndex={folder.id === tabStopId ? 0 : -1}
                                    aria-label={`Delete ${folder.name}`}
                                    title="Delete folder"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        props.onDeleteFolder(folder)
                                    }}
                                >
                                    &times;
                                </button>
                            </span>
                        )}
                    </li>
                )
            })}
        </ul>
    )
}
