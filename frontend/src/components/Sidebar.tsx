import {useEffect, useState} from 'react'
import icon from '../assets/pigeonpost.png'
import {Account, Folder} from '../api'

interface SidebarProps {
    accounts: Account[]
    selectedAccount: string
    folders: Folder[]
    selectedFolder: string
    onSelectAccount: (id: string) => void
    onSelectFolder: (id: string) => void
    onEditAccount: (account: Account) => void
    onDeleteAccount: (account: Account) => void
    onNewFolder: () => void
    onRenameFolder: (folder: Folder) => void
    onDeleteFolder: (folder: Folder) => void
}

const folderIcon: Record<string, string> = {
    inbox: '\u{1F4E5}',
    sent: '\u{1F4E4}',
    drafts: '\u{1F4DD}',
    trash: '\u{1F5D1}\u{FE0F}',
    junk: '\u{1F6AB}',
    archive: '\u{1F5C3}\u{FE0F}',
    custom: '\u{1F4C1}',
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
    const {accounts, selectedAccount, folders, selectedFolder} = props
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
                        <button
                            className="section-action"
                            title="New folder"
                            aria-label="New folder"
                            onClick={props.onNewFolder}
                        >
                            &#43;
                        </button>
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

// FolderTree renders the folders as a nested, collapsible tree derived from their paths. The collapsed
// state is persisted per account in localStorage, so it survives restarts.
function FolderTree(props: FolderTreeProps) {
    const {folders, selectedFolder, selectedAccount} = props
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

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
    // A folder is visible only when none of its ancestors are collapsed.
    const visible = folders.filter((f) => ancestorPaths(f.path, sep).every((a) => !collapsed.has(a)))

    return (
        <ul className="list">
            {visible.map((folder) => {
                const leaf = leafName(folder.path, sep)
                const depth = ancestorPaths(folder.path, sep).length
                const parent = hasChildren(folder.path)
                const isCollapsed = collapsed.has(folder.path)
                return (
                    <li
                        key={folder.id}
                        className={'list-item folder' + (folder.id === selectedFolder ? ' selected' : '')}
                        style={{paddingLeft: 14 + depth * 14}}
                        onClick={() => props.onSelectFolder(folder.id)}
                    >
                        <span className="folder-name">
                            {parent ? (
                                <button
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
