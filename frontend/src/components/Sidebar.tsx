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

const FOLDER_SEPARATOR = '/'

function collapseKey(accountId: string): string {
    return `pigeonpost.collapsed.${accountId}`
}

// ancestorPaths returns every parent path of a folder path, e.g. "A/B/C" -> ["A", "A/B"].
function ancestorPaths(path: string): string[] {
    const parts = path.split(FOLDER_SEPARATOR)
    const out: string[] = []
    for (let i = 1; i < parts.length; i++) {
        out.push(parts.slice(0, i).join(FOLDER_SEPARATOR))
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
    const hasChildren = (path: string) => paths.some((p) => p.startsWith(path + FOLDER_SEPARATOR))
    // A folder is visible only when none of its ancestors are collapsed.
    const visible = folders.filter((f) => ancestorPaths(f.path).every((a) => !collapsed.has(a)))

    return (
        <ul className="list">
            {visible.map((folder) => {
                const depth = folder.path.split(FOLDER_SEPARATOR).length - 1
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
                                    aria-label={isCollapsed ? `Expand ${folder.name}` : `Collapse ${folder.name}`}
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
                            {folder.name}
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
