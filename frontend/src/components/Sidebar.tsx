import {useEffect, useState} from 'react'
import icon from '../assets/pigeonpost.png'
import {Account, Folder} from '../api'
import {messageDragType} from './MessageList'

interface SidebarProps {
    accounts: Account[]
    selectedAccount: string
    // unreadByAccount maps an account id to its unread message count. An account with no unread mail is
    // absent from the map.
    unreadByAccount: {[accountId: string]: number}
    folders: Folder[]
    selectedFolder: string
    onSelectAccount: (id: string) => void
    onSelectFolder: (id: string) => void
    onEditAccount: (account: Account) => void
    onDeleteAccount: (account: Account) => void
    onNewFolder: () => void
    onRenameFolder: (folder: Folder) => void
    onDeleteFolder: (folder: Folder) => void
    onDropMessage: (messageId: string, folderId: string) => void
    // canManageFolders is false for POP3 accounts, which have no server-side folders to create.
    canManageFolders: boolean
}

const folderIcon: Record<string, string> = {
    inbox: '\u{1F4E5}',
    sent: '\u{1F4E4}',
    drafts: '\u{1F4DD}',
    trash: '\u{1F5D1}\u{FE0F}',
    junk: '\u{1F6AB}',
    archive: '\u{1F5C3}\u{FE0F}',
    outbox: '\u{1F4EE}',
    custom: '\u{1F4C1}',
}

// specialFolderOrder is the canonical top-to-bottom order for the well-known mailboxes, so Inbox,
// Sent and the rest sit at the top rather than wherever the server happens to list them. Any kind not
// named here (custom folders) ranks after all of these and keeps its original relative order.
const specialFolderOrder = ['inbox', 'drafts', 'sent', 'archive', 'junk', 'trash']

function folderRank(kind: string): number {
    const idx = specialFolderOrder.indexOf(kind)
    return idx === -1 ? specialFolderOrder.length : idx
}

export function Sidebar(props: SidebarProps) {
    return (
        <aside className="pane sidebar">
            <img className="sidebar-brand" src={icon} alt="" aria-hidden="true"/>
            {props.accounts.length === 0 ? (
                <div className="empty-state">
                    <div className="empty-title">No accounts yet</div>
                    <p className="empty-body">
                        Use "Add account" to configure a mail account.
                    </p>
                </div>
            ) : (
                <SidebarContent {...props}/>
            )}
        </aside>
    )
}

function SidebarContent(props: SidebarProps) {
    const {accounts, selectedAccount, unreadByAccount, folders, selectedFolder} = props
    return (
        <>
            <div className="section-label">Accounts</div>
            <ul className="list">
                {accounts.map((account) => (
                    <li
                        key={account.id}
                        className={'list-item account' + (account.id === selectedAccount ? ' selected' : '')}
                        onClick={() => props.onSelectAccount(account.id)}
                    >
                        <span className="item-text">
                            <span className="item-title">{account.displayName}</span>
                            <span className="item-sub">{account.email}</span>
                        </span>
                        {(unreadByAccount[account.id] ?? 0) > 0 && (
                            <span
                                className="badge account-badge"
                                title={`${unreadByAccount[account.id]} unread`}
                            >
                                {unreadByAccount[account.id]}
                            </span>
                        )}
                        <span className="account-actions">
                            <button
                                className="account-action"
                                aria-label={`Edit ${account.email}`}
                                title="Edit account"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    props.onEditAccount(account)
                                }}
                            >
                                &#9998;
                            </button>
                            <button
                                className="account-action delete"
                                aria-label={`Remove ${account.email}`}
                                title="Remove account"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    props.onDeleteAccount(account)
                                }}
                            >
                                &times;
                            </button>
                        </span>
                    </li>
                ))}
            </ul>

            {selectedAccount && (
                <>
                    <div className="section-header">
                        <span className="section-label">Folders</span>
                        {props.canManageFolders && (
                            <button
                                className="section-action"
                                title="New folder"
                                aria-label="New folder"
                                onClick={props.onNewFolder}
                            >
                                &#43;
                            </button>
                        )}
                    </div>
                    {folders.length === 0 ? (
                        <p className="empty-body indented">No folders cached. Press Sync to fetch them.</p>
                    ) : (
                        <FolderTree
                            folders={folders}
                            selectedFolder={selectedFolder}
                            selectedAccount={selectedAccount}
                            onSelectFolder={props.onSelectFolder}
                            onRenameFolder={props.onRenameFolder}
                            onDeleteFolder={props.onDeleteFolder}
                            onDropMessage={props.onDropMessage}
                        />
                    )}
                </>
            )}
        </>
    )
}

interface FolderTreeProps {
    folders: Folder[]
    selectedFolder: string
    selectedAccount: string
    onSelectFolder: (id: string) => void
    onRenameFolder: (folder: Folder) => void
    onDeleteFolder: (folder: Folder) => void
    onDropMessage: (messageId: string, folderId: string) => void
}

function collapseKey(accountId: string): string {
    return `pigeonpost.collapsed.${accountId}`
}

// detectSeparator infers the server's mailbox hierarchy delimiter from the folder paths. A character
// is the delimiter when some folder's path, split on it, yields a parent that is itself a folder (e.g.
// "Archived.Debt" alongside "Archived" means the delimiter is "."). It checks the two common IMAP
// delimiters and falls back to "/".
function detectSeparator(paths: string[]): string {
    const set = new Set(paths)
    for (const sep of ['.', '/']) {
        for (const p of paths) {
            const idx = p.lastIndexOf(sep)
            if (idx > 0 && set.has(p.slice(0, idx))) {
                return sep
            }
        }
    }
    return '/'
}

// leafName returns the last segment of a path under the given separator.
function leafName(path: string, sep: string): string {
    const idx = path.lastIndexOf(sep)
    return idx >= 0 ? path.slice(idx + 1) : path
}

// ancestorPaths returns every parent path of a folder path under the given separator.
function ancestorPaths(path: string, sep: string): string[] {
    const parts = path.split(sep)
    const out: string[] = []
    for (let i = 1; i < parts.length; i++) {
        out.push(parts.slice(0, i).join(sep))
    }
    return out
}

// orderFolders reorders the folders for display so the well-known mailboxes lead (see
// specialFolderOrder) while every subtree stays contiguous under its parent. It walks the tree from
// the roots, sorting siblings at each level by folder rank; the sort is stable, so same-rank siblings
// (in particular custom folders) keep their original server order.
function orderFolders(folders: Folder[], sep: string): Folder[] {
    const byPath = new Map(folders.map((f) => [f.path, f]))
    const nearestParent = (f: Folder): string | null => {
        const ancestors = ancestorPaths(f.path, sep)
        for (let i = ancestors.length - 1; i >= 0; i--) {
            if (byPath.has(ancestors[i])) {
                return ancestors[i]
            }
        }
        return null
    }
    const childrenOf = new Map<string, Folder[]>()
    const roots: Folder[] = []
    folders.forEach((f) => {
        const parent = nearestParent(f)
        if (parent === null) {
            roots.push(f)
            return
        }
        const siblings = childrenOf.get(parent) ?? []
        siblings.push(f)
        childrenOf.set(parent, siblings)
    })
    const sortSiblings = (arr: Folder[]) =>
        [...arr].sort((a, b) => folderRank(a.kind) - folderRank(b.kind))
    const ordered: Folder[] = []
    const walk = (f: Folder) => {
        ordered.push(f)
        sortSiblings(childrenOf.get(f.path) ?? []).forEach(walk)
    }
    sortSiblings(roots).forEach(walk)
    return ordered
}

// FolderTree renders the folders as a nested, collapsible tree derived from their paths. The collapsed
// state is persisted per account in localStorage, so it survives restarts.
function FolderTree(props: FolderTreeProps) {
    const {folders, selectedFolder, selectedAccount} = props
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
    const [dragOverId, setDragOverId] = useState<string>('')

    useEffect(() => {
        try {
            const raw = localStorage.getItem(collapseKey(selectedAccount))
            setCollapsed(new Set(raw ? (JSON.parse(raw) as string[]) : []))
        } catch {
            setCollapsed(new Set())
        }
    }, [selectedAccount])

    const toggle = (path: string) => {
        setCollapsed((prev) => {
            const next = new Set(prev)
            if (next.has(path)) {
                next.delete(path)
            } else {
                next.add(path)
            }
            try {
                localStorage.setItem(collapseKey(selectedAccount), JSON.stringify([...next]))
            } catch {
                // A storage failure just means the state is not remembered; the UI still works.
            }
            return next
        })
    }

    const paths = folders.map((f) => f.path)
    const sep = detectSeparator(paths)
    const hasChildren = (path: string) => paths.some((p) => p.startsWith(path + sep))
    const ordered = orderFolders(folders, sep)
    // A folder is visible only when none of its ancestors are collapsed.
    const visible = ordered.filter((f) => ancestorPaths(f.path, sep).every((a) => !collapsed.has(a)))
    // The folder list is a single focus-ring stop: only one folder is tabbable (the selected one, or the
    // first when none is selected), and Up/Down move between folders from there.
    const tabStopId = selectedFolder || (visible.length > 0 ? visible[0].id : '')

    return (
        <ul className="list" data-folder-list="">
            {visible.map((folder) => {
                const leaf = leafName(folder.path, sep)
                const depth = ancestorPaths(folder.path, sep).length
                const parent = hasChildren(folder.path)
                const isCollapsed = collapsed.has(folder.path)
                return (
                    <li
                        key={folder.id}
                        data-folder-id={folder.id}
                        className={
                            'list-item folder' +
                            (folder.id === selectedFolder ? ' selected' : '') +
                            (folder.id === dragOverId ? ' drag-over' : '')
                        }
                        style={{paddingLeft: 14 + depth * 14}}
                        tabIndex={folder.id === tabStopId ? 0 : -1}
                        onClick={() => props.onSelectFolder(folder.id)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                props.onSelectFolder(folder.id)
                                return
                            }
                            // Up/Down move between folders within this one focus-ring stop. Left/Right are
                            // left to bubble up to the window handler, which steps the focus ring.
                            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                                e.preventDefault()
                                e.stopPropagation()
                                const sibling = e.key === 'ArrowDown'
                                    ? e.currentTarget.nextElementSibling
                                    : e.currentTarget.previousElementSibling
                                if (sibling instanceof HTMLElement) {
                                    sibling.focus()
                                    const id = sibling.getAttribute('data-folder-id')
                                    if (id) {
                                        props.onSelectFolder(id)
                                    }
                                }
                            }
                        }}
                        onDragOver={(e) => {
                            if (e.dataTransfer.types.includes(messageDragType)) {
                                e.preventDefault()
                                e.dataTransfer.dropEffect = 'move'
                                setDragOverId(folder.id)
                            }
                        }}
                        onDragLeave={() => setDragOverId((id) => (id === folder.id ? '' : id))}
                        onDrop={(e) => {
                            e.preventDefault()
                            setDragOverId('')
                            const messageId = e.dataTransfer.getData(messageDragType)
                            if (messageId) {
                                props.onDropMessage(messageId, folder.id)
                            }
                        }}
                    >
                        <span className="folder-name">
                            {parent ? (
                                <button
                                    type="button"
                                    className="folder-toggle"
                                    aria-label={isCollapsed ? `Expand ${leaf}` : `Collapse ${leaf}`}
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        toggle(folder.path)
                                    }}
                                >
                                    {isCollapsed ? '▸' : '▾'}
                                </button>
                            ) : (
                                <span className="folder-toggle-spacer"/>
                            )}
                            <span className="folder-icon">{folderIcon[folder.kind] ?? folderIcon.custom}</span>
                            {leaf}
                        </span>
                        {folder.unread > 0 && <span className="badge">{folder.unread}</span>}
                        {folder.kind === 'custom' && (
                            <span className="account-actions">
                                <button
                                    className="account-action"
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
