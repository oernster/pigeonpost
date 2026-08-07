import {useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent} from 'react'
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

// accountName is how an account is named in the picker. The address is the identity that matters, so it is
// always shown; a display name that says something different is shown ahead of it.
export function accountName(account: Account): string {
    return account.displayName && account.displayName !== account.email
        ? `${account.displayName} (${account.email})`
        : account.email
}

// accountOptionLabel is one entry's accessible name: the account's name plus whatever the row shows beside
// it. The unread count renders as a badge and the syncing cue as a small caption, neither of which reads
// usefully on its own, so both are spelled out here for the screen reader.
export function accountOptionLabel(account: Account, unread: number, syncing: boolean): string {
    const marks: string[] = []
    if (unread > 0) {
        marks.push(`${unread} unread`)
    }
    if (syncing) {
        marks.push('synchronising')
    }
    const base = accountName(account)
    return marks.length > 0 ? `${base} - ${marks.join(', ')}` : base
}

// LIST_ID and optionId tie the trigger to the popup and to the option the keyboard cursor is on, through
// aria-controls and aria-activedescendant. Focus never leaves the trigger, so the cursor has to be named
// rather than focused.
const LIST_ID = 'account-picker-list'
const optionId = (accountId: string) => `account-option-${accountId}`

// AccountPicker renders the sidebar's accounts section as a single dropdown holding the active account, with
// the edit and remove actions beside it acting on whichever account is showing. One row of height whatever
// the number of accounts, so the folders below keep the rest of the pane.
//
// It is a listbox built here rather than a native select, for one behavioural reason: a native select is
// silent when you re-pick the option already showing, so choosing the account you are already on did
// nothing, when what it should do is take you back to that account's inbox. Every pick reports its account,
// whether or not it is the current one, and App opens that account's inbox from there.
//
// It stays a single focus-ring stop, as the select was: focus rests on the trigger and never moves into the
// popup, with Up and Down walking the options from there.
export function AccountPicker({
    accounts, selectedAccount, syncingAccountIds, unreadByAccount,
    onSelectAccount, onEditAccount, onDeleteAccount,
}: AccountPickerProps) {
    // The active account is the selected one; before the first selection settles the picker shows the
    // first account rather than an empty box.
    const active = accounts.find((a) => a.id === selectedAccount) ?? accounts[0]
    const [open, setOpen] = useState(false)
    // cursor is the option the keyboard is on while the list is open, as an index into accounts. It starts
    // at the active account on each open, so Up and Down move from what is showing.
    const [cursor, setCursor] = useState(0)
    const rootRef = useRef<HTMLDivElement | null>(null)
    const triggerRef = useRef<HTMLButtonElement | null>(null)

    const activeIndex = Math.max(0, accounts.findIndex((a) => a.id === (active ? active.id : '')))

    // A press outside the picker dismisses the list, the same as Escape. Bound only while it is open, so
    // the closed picker adds no document listener.
    useEffect(() => {
        if (!open) {
            return
        }
        const onDocumentPointerDown = (e: MouseEvent) => {
            if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
                setOpen(false)
            }
        }
        document.addEventListener('mousedown', onDocumentPointerDown)
        return () => document.removeEventListener('mousedown', onDocumentPointerDown)
    }, [open])

    const openList = () => {
        setCursor(activeIndex)
        setOpen(true)
    }

    // close returns focus to the trigger, so dismissing the list never drops the focus ring out of the
    // sidebar and back to the document.
    const close = () => {
        setOpen(false)
        triggerRef.current?.focus()
    }

    // choose reports the pick and closes. It fires for the account already showing as well as for a
    // different one: re-picking the active account is how you get back to its inbox.
    const choose = (id: string) => {
        setOpen(false)
        onSelectAccount(id)
    }

    const moveCursor = (delta: number) => {
        setCursor((current) => (current + delta + accounts.length) % accounts.length)
    }

    // The account list can shrink under an open picker (an account removed elsewhere), so the cursor is
    // read back through the list rather than trusted as an index.
    const cursorAccount = accounts[cursor] ?? accounts[activeIndex]

    const onTriggerKeyDown = (e: ReactKeyboardEvent<HTMLButtonElement>) => {
        if (!open) {
            // Down, Up and Enter all open the list. Space is left to the button's own default, which is a
            // click, and the click handler opens it.
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Enter') {
                e.preventDefault()
                openList()
            }
            return
        }
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault()
            e.stopPropagation()
            moveCursor(e.key === 'ArrowDown' ? 1 : -1)
            return
        }
        if (e.key === 'Home' || e.key === 'End') {
            e.preventDefault()
            setCursor(e.key === 'Home' ? 0 : accounts.length - 1)
            return
        }
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            choose(cursorAccount.id)
            return
        }
        if (e.key === 'Escape') {
            e.preventDefault()
            close()
            return
        }
        // Tab and the horizontal arrows all leave the picker for the next ring stop, so the list must not
        // stay open behind the ring. The keys themselves are left alone: the ring is what moves focus.
        if (e.key === 'Tab' || e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
            setOpen(false)
        }
    }

    if (!active) {
        return null
    }

    return (
        <div className="account-picker" data-account-picker="" ref={rootRef}>
            <div className="section-label">Accounts</div>
            <div className="account-picker-row">
                <button
                    type="button"
                    className="account-trigger"
                    ref={triggerRef}
                    aria-label="Active account"
                    aria-haspopup="listbox"
                    aria-expanded={open}
                    aria-controls={LIST_ID}
                    aria-activedescendant={open ? optionId(cursorAccount.id) : undefined}
                    onClick={() => (open ? setOpen(false) : openList())}
                    onKeyDown={onTriggerKeyDown}
                >
                    <AccountRow
                        account={active}
                        unread={unreadByAccount[active.id] ?? 0}
                        syncing={syncingAccountIds.has(active.id)}
                    />
                    <span className="account-trigger-caret" aria-hidden="true">{open ? '▴' : '▾'}</span>
                </button>
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
            </div>
            {open && (
                <ul className="account-list" id={LIST_ID} role="listbox" aria-label="Accounts">
                    {accounts.map((account, index) => (
                        <li
                            key={account.id}
                            id={optionId(account.id)}
                            role="option"
                            aria-selected={account.id === active.id}
                            aria-label={accountOptionLabel(
                                account,
                                unreadByAccount[account.id] ?? 0,
                                syncingAccountIds.has(account.id),
                            )}
                            className={
                                'account-option' +
                                (account.id === active.id ? ' selected' : '') +
                                (index === cursor ? ' cursor' : '')
                            }
                            // The pointer press is taken here rather than on click, so the choice lands
                            // before the outside-press handler can see it as a press outside.
                            onMouseDown={(e) => {
                                e.preventDefault()
                                choose(account.id)
                            }}
                            onMouseEnter={() => setCursor(index)}
                        >
                            <AccountRow
                                account={account}
                                unread={unreadByAccount[account.id] ?? 0}
                                syncing={syncingAccountIds.has(account.id)}
                            />
                        </li>
                    ))}
                </ul>
            )}
        </div>
    )
}

// AccountRow is one account's content, shared by the trigger and the options so the closed picker shows
// exactly what its entry in the list shows. The unread count is the same yellow badge the folder rows
// carry, rather than words in the label, so the two read as the same thing.
function AccountRow({account, unread, syncing}: {account: Account; unread: number; syncing: boolean}) {
    return (
        <span className="account-row">
            <span className="account-row-text">
                <span className="account-row-name">{accountName(account)}</span>
                {syncing && <span className="account-row-sub">Synchronising</span>}
            </span>
            {unread > 0 && <span className="badge">{unread}</span>}
        </span>
    )
}
