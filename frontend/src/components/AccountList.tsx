import type {Account} from '../api'
import {swapId} from '../sidebarDnd'
import {useAccountReorder} from '../hooks/useAccountReorder'

interface AccountListProps {
    accounts: Account[]
    selectedAccount: string
    syncingAccountIds: ReadonlySet<string>
    unreadByAccount: {[accountId: string]: number}
    onSelectAccount: (id: string) => void
    onEditAccount: (account: Account) => void
    onDeleteAccount: (account: Account) => void
    onReorderAccounts: (orderedIds: string[]) => void
}

// AccountList renders the sidebar's accounts section: one row per account with its unread badge, name and
// address, a syncing cue and the reorder, edit and remove actions. The list is a single focus-ring stop
// (only one row is tabbable) and Up/Down move between rows. Reordering (the drag and the up/down buttons)
// is offered only with more than one account.
export function AccountList({
    accounts, selectedAccount, syncingAccountIds, unreadByAccount,
    onSelectAccount, onEditAccount, onDeleteAccount, onReorderAccounts,
}: AccountListProps) {
    const accountIds = accounts.map((a) => a.id)
    // Reordering is only meaningful with more than one account, so the drag and the up/down buttons are
    // enabled only then.
    const canReorder = accounts.length > 1
    // The account list is a single focus-ring stop: only one row is tabbable (the selected account,
    // otherwise the first when none is selected). Up/Down move between accounts, wrapping at the ends.
    const accountTabStopId = selectedAccount || (accounts.length > 0 ? accounts[0].id : '')
    const {dragId, dragOverId, rowDragProps} = useAccountReorder(accountIds, onReorderAccounts)
    return (
        <>
            <div className="section-label">Accounts</div>
            <ul className="list" data-account-list="">
                {accounts.map((account, index) => (
                    <li
                        key={account.id}
                        data-account-id={account.id}
                        tabIndex={account.id === accountTabStopId ? 0 : -1}
                        className={
                            'list-item account' +
                            (account.id === selectedAccount ? ' selected' : '') +
                            (account.id === dragOverId ? ' drag-over' : '') +
                            (account.id === dragId ? ' dragging' : '')
                        }
                        draggable={canReorder}
                        onClick={() => onSelectAccount(account.id)}
                        onKeyDown={(e) => {
                            // Only the row itself drives selection and Up/Down; a key on a child action
                            // button (edit, remove, reorder) is left to that button.
                            if (e.target !== e.currentTarget) {
                                return
                            }
                            if (e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
                                e.preventDefault()
                                onSelectAccount(account.id)
                                return
                            }
                            // Up/Down move between accounts within this one focus-ring stop, wrapping at the
                            // ends. Left/Right bubble to the window handler, which steps the focus ring.
                            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                                e.preventDefault()
                                e.stopPropagation()
                                const li = e.currentTarget
                                const parent = li.parentElement
                                let sibling = e.key === 'ArrowDown' ? li.nextElementSibling : li.previousElementSibling
                                if (!sibling && parent) {
                                    sibling = e.key === 'ArrowDown' ? parent.firstElementChild : parent.lastElementChild
                                }
                                if (sibling instanceof HTMLElement) {
                                    sibling.focus()
                                    const id = sibling.getAttribute('data-account-id')
                                    if (id) {
                                        onSelectAccount(id)
                                    }
                                }
                            }
                        }}
                        {...rowDragProps(account.id)}
                    >
                        <span className="account-badge-slot">
                            {(unreadByAccount[account.id] ?? 0) > 0 && (
                                <span
                                    className="badge account-badge"
                                    title={`${unreadByAccount[account.id]} unread`}
                                >
                                    {unreadByAccount[account.id]}
                                </span>
                            )}
                        </span>
                        <span className="item-text">
                            <span className="item-title" title={account.displayName}>{account.displayName}</span>
                            <span className="item-sub" title={account.email}>{account.email}</span>
                            {syncingAccountIds.has(account.id) && (
                                <span className="account-syncing">Synchronising…</span>
                            )}
                        </span>
                        <span className="account-actions">
                            {canReorder && (
                                <>
                                    <button
                                        className="account-action"
                                        tabIndex={account.id === accountTabStopId ? 0 : -1}
                                        aria-label={`Move ${account.email} up`}
                                        title="Move up"
                                        disabled={index === 0}
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            onReorderAccounts(swapId(accountIds, index, index - 1))
                                        }}
                                    >
                                        &#8593;
                                    </button>
                                    <button
                                        className="account-action"
                                        tabIndex={account.id === accountTabStopId ? 0 : -1}
                                        aria-label={`Move ${account.email} down`}
                                        title="Move down"
                                        disabled={index === accounts.length - 1}
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            onReorderAccounts(swapId(accountIds, index, index + 1))
                                        }}
                                    >
                                        &#8595;
                                    </button>
                                </>
                            )}
                            <button
                                className="account-action"
                                tabIndex={account.id === accountTabStopId ? 0 : -1}
                                aria-label={`Edit ${account.email}`}
                                title="Edit account"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onEditAccount(account)
                                }}
                            >
                                &#9998;
                            </button>
                            <button
                                className="account-action delete"
                                tabIndex={account.id === accountTabStopId ? 0 : -1}
                                aria-label={`Remove ${account.email}`}
                                title="Remove account"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onDeleteAccount(account)
                                }}
                            >
                                &times;
                            </button>
                        </span>
                    </li>
                ))}
            </ul>
        </>
    )
}
