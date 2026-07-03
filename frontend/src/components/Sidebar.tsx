import {Account, Folder} from '../api'

interface SidebarProps {
    accounts: Account[]
    selectedAccount: string
    folders: Folder[]
    selectedFolder: string
    onSelectAccount: (id: string) => void
    onSelectFolder: (id: string) => void
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
    const {accounts, selectedAccount, folders, selectedFolder} = props

    if (accounts.length === 0) {
        return (
            <aside className="pane sidebar">
                <div className="empty-state">
                    <div className="empty-title">No accounts yet</div>
                    <p className="empty-body">
                        Account setup arrives in a later phase. This is the phase 1 read-only skeleton.
                    </p>
                </div>
            </aside>
        )
    }

    return (
        <aside className="pane sidebar">
            <div className="section-label">Accounts</div>
            <ul className="list">
                {accounts.map((account) => (
                    <li
                        key={account.id}
                        className={'list-item' + (account.id === selectedAccount ? ' selected' : '')}
                        onClick={() => props.onSelectAccount(account.id)}
                    >
                        <span className="item-title">{account.displayName}</span>
                        <span className="item-sub">{account.email}</span>
                    </li>
                ))}
            </ul>

            {selectedAccount && (
                <>
                    <div className="section-label">Folders</div>
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
                                </li>
                            ))}
                        </ul>
                    )}
                </>
            )}
        </aside>
    )
}
