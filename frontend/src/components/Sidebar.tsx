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
                        <ul className="list">
                            {folders.map((folder) => (
                                <li
                                    key={folder.id}
                                    className={'list-item folder' + (folder.id === selectedFolder ? ' selected' : '')}
                                    onClick={() => props.onSelectFolder(folder.id)}
                                >
                                    <span className="folder-name">
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
                            ))}
                        </ul>
                    )}
                </>
            )}
        </>
    )
}
