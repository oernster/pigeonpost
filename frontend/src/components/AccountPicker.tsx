import type {Account} from '../api'

interface AccountPickerProps {
    accounts: Account[]
    selectedAccount: string
    syncingAccountIds: ReadonlySet<string>
    unreadByAccount: {[accountId: string]: number}
    onSelectAccount: (id: string) => void
    onEditAccount: (account: Account) => void
    onDeleteAccount: (account: Account) => void
}

// accountOptionLabel is the text of one entry in the picker. The address is the identity that matters, so
// it is always shown; a display name that says something different is shown ahead of it. The unread count
// and the syncing cue ride on the same line, since an option carries text and nothing else.
export function accountOptionLabel(account: Account, unread: number, syncing: boolean): string {
    const base = account.displayName && account.displayName !== account.email
        ? `${account.displayName} (${account.email})`
        : account.email
    const marks: string[] = []
    if (unread > 0) {
        marks.push(`${unread} unread`)
    }
    if (syncing) {
        marks.push('synchronising')
    }
    return marks.length > 0 ? `${base} - ${marks.join(', ')}` : base
}

// AccountPicker renders the sidebar's accounts section as a single dropdown holding the active account,
// with the edit and remove actions beside it acting on whichever account is showing. One row of height
// whatever the number of accounts, so the folders below keep the rest of the pane. It is a native select,
// so it is one focus-ring stop and Up/Down move between accounts inside it.
export function AccountPicker({
    accounts, selectedAccount, syncingAccountIds, unreadByAccount,
    onSelectAccount, onEditAccount, onDeleteAccount,
}: AccountPickerProps) {
    // The active account is the selected one; before the first selection settles the picker shows the
    // first account rather than an empty box.
    const active = accounts.find((a) => a.id === selectedAccount) ?? accounts[0]
    return (
        <div className="account-picker" data-account-picker="">
            <div className="section-label">Accounts</div>
            <div className="account-picker-row">
                <select
                    className="account-select"
                    aria-label="Active account"
                    value={active ? active.id : ''}
                    onChange={(e) => onSelectAccount(e.target.value)}
                >
                    {accounts.map((account) => (
                        <option key={account.id} value={account.id}>
                            {accountOptionLabel(
                                account,
                                unreadByAccount[account.id] ?? 0,
                                syncingAccountIds.has(account.id),
                            )}
                        </option>
                    ))}
                </select>
                {active && (
                    <span className="account-picker-actions">
                        <button
                            className="account-action"
                            aria-label={`Edit ${active.email}`}
                            title="Edit account"
                            onClick={() => onEditAccount(active)}
                        >
                            &#9998;
                        </button>
                        <button
                            className="account-action delete"
                            aria-label={`Remove ${active.email}`}
                            title="Remove account"
                            onClick={() => onDeleteAccount(active)}
                        >
                            &times;
                        </button>
                    </span>
                )}
            </div>
        </div>
    )
}
