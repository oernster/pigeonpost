// Characterization test for the sidebar at its stable outer interface: its props and its observable
// behaviour. It renders the real Sidebar and drives each interaction, asserting the DOM plus which callback
// fired and what was written to localStorage. The interface it pins does not move as the sidebar is
// decomposed in Phase 2 (the persisted collapsed and order state, the account list with its reorder drag
// and the folder-tree drop split are lifted out beneath these same props), so this suite staying green is
// the proof each extraction preserved behaviour. The sidebar makes no api calls, so nothing is mocked; the
// drag math is exercised through the real sidebarDnd and folderPaths modules.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, within} from '@testing-library/react'
import type {ComponentProps} from 'react'
import {Sidebar} from './Sidebar'
import type {Account, Folder} from '../api'
import {accountDragType, folderDragType} from '../sidebarDnd'
import {messageDragType} from './MessageList'

type SidebarProps = ComponentProps<typeof Sidebar>

// These mirror the sidebar's private localStorage keys. They are part of the persisted contract (the state
// that survives a restart), so pinning the exact keys is deliberate: a change here is a behaviour change.
const collapseKey = (accountId: string) => `pigeonpost.collapsed.${accountId}`
const folderOrderKey = (accountId: string) => `pigeonpost.folderorder.${accountId}`

function makeAccount(id: string, displayName: string, email: string): Account {
    return {id, displayName, email, protocol: 'imap'} as Account
}

function makeFolder(id: string, path: string, kind: string, extra: Partial<Folder> = {}): Folder {
    return {
        id, accountId: 'a1', path, name: path.split('/').pop() ?? path, kind, unread: 0, total: 0, ...extra,
    }
}

// makeDataTransfer is a minimal stand-in for the drop event's DataTransfer: the drop handlers read getData
// and types only. A drag that originates elsewhere (a message row, a folder row) is modelled by seeding the
// data directly, so a drop can be fired without a matching dragstart.
function makeDataTransfer(data: Record<string, string> = {}): DataTransfer {
    const store: Record<string, string> = {...data}
    return {
        setData(type: string, value: string) {
            store[type] = value
        },
        getData(type: string) {
            return store[type] ?? ''
        },
        get types() {
            return Object.keys(store)
        },
        dropEffect: 'none',
        effectAllowed: 'all',
    } as unknown as DataTransfer
}

const ACCOUNTS = [makeAccount('a1', 'Alice', 'alice@x.com'), makeAccount('a2', 'Bob', 'bob@x.com')]

function renderSidebar(overrides: Partial<SidebarProps> = {}) {
    const handlers = {
        onSelectUnified: vi.fn(),
        onSelectSnoozed: vi.fn(),
        onSelectAccount: vi.fn(),
        onSelectFolder: vi.fn(),
        onEditAccount: vi.fn(),
        onDeleteAccount: vi.fn(),
        onReorderAccounts: vi.fn(),
        onNewFolder: vi.fn(),
        onRenameFolder: vi.fn(),
        onReparentFolder: vi.fn(),
        onDeleteFolder: vi.fn(),
        onDropMessage: vi.fn(),
        onFolderContextMenu: vi.fn(),
    }
    const props: SidebarProps = {
        accounts: ACCOUNTS,
        selectedAccount: 'a1',
        unifiedEnabled: false,
        unifiedSelected: false,
        unifiedUnread: 0,
        snoozedCount: 0,
        snoozedSelected: false,
        syncingAccountIds: new Set<string>(),
        unreadByAccount: {},
        folders: [],
        selectedFolder: '',
        canManageFolders: true,
        ...handlers,
        ...overrides,
    }
    const view = render(<Sidebar {...props}/>)
    const accountRow = (id: string) => view.container.querySelector<HTMLElement>(`[data-account-id="${id}"]`)
    const folderRow = (id: string) => view.container.querySelector<HTMLElement>(`[data-folder-id="${id}"]`)
    return {...view, ...handlers, accountRow, folderRow}
}

beforeEach(() => localStorage.clear())
afterEach(() => cleanup())

describe('Sidebar: shell', () => {
    it('shows the empty state when there are no accounts', () => {
        const {container} = renderSidebar({accounts: []})
        expect(container.querySelector('[data-account-list]')).toBeNull()
        expect(container.textContent).toContain('No accounts yet')
    })

    it('renders a row per account with its name and address', () => {
        const {accountRow} = renderSidebar()
        expect(accountRow('a1')).toHaveTextContent('Alice')
        expect(accountRow('a1')).toHaveTextContent('alice@x.com')
        expect(accountRow('a2')).toHaveTextContent('bob@x.com')
    })

    it('keeps the brand icon outside the scroll region so only accounts and folders scroll', () => {
        const {container, accountRow} = renderSidebar()
        const brand = container.querySelector('.sidebar-brand')!
        expect(brand.parentElement!.classList.contains('sidebar')).toBe(true)
        expect(brand.closest('.sidebar-scroll')).toBeNull()
        expect(accountRow('a1')!.closest('.sidebar-scroll')).not.toBeNull()
    })

    it('places the empty state inside the scroll region too', () => {
        const {container} = renderSidebar({accounts: []})
        const empty = container.querySelector('.empty-state')!
        expect(empty.closest('.sidebar-scroll')).not.toBeNull()
    })
})

describe('Sidebar: account selection', () => {
    it('selects an account on click', () => {
        const {accountRow, onSelectAccount} = renderSidebar()
        fireEvent.click(accountRow('a2')!)
        expect(onSelectAccount).toHaveBeenCalledWith('a2')
    })

    it('moves between accounts with the arrow keys, selecting as it goes', () => {
        const {accountRow, onSelectAccount} = renderSidebar()
        fireEvent.keyDown(accountRow('a1')!, {key: 'ArrowDown'})
        expect(onSelectAccount).toHaveBeenCalledWith('a2')
        expect(accountRow('a2')).toHaveFocus()
    })

    it('wraps from the first account to the last on ArrowUp', () => {
        const {accountRow, onSelectAccount} = renderSidebar()
        fireEvent.keyDown(accountRow('a1')!, {key: 'ArrowUp'})
        expect(onSelectAccount).toHaveBeenCalledWith('a2')
    })

    it('selects on Enter', () => {
        const {accountRow, onSelectAccount} = renderSidebar()
        fireEvent.keyDown(accountRow('a2')!, {key: 'Enter'})
        expect(onSelectAccount).toHaveBeenCalledWith('a2')
    })
})

describe('Sidebar: account management', () => {
    it('reorders with the up and down buttons', () => {
        const {getByLabelText, onReorderAccounts} = renderSidebar()
        fireEvent.click(getByLabelText('Move bob@x.com up'))
        expect(onReorderAccounts).toHaveBeenCalledWith(['a2', 'a1'])
        fireEvent.click(getByLabelText('Move alice@x.com down'))
        expect(onReorderAccounts).toHaveBeenCalledWith(['a2', 'a1'])
    })

    it('disables up on the first account and down on the last', () => {
        const {getByLabelText} = renderSidebar()
        expect(getByLabelText('Move alice@x.com up')).toBeDisabled()
        expect(getByLabelText('Move bob@x.com down')).toBeDisabled()
    })

    it('edits and removes an account', () => {
        const {getByLabelText, onEditAccount, onDeleteAccount} = renderSidebar()
        fireEvent.click(getByLabelText('Edit bob@x.com'))
        expect(onEditAccount).toHaveBeenCalledWith(ACCOUNTS[1])
        fireEvent.click(getByLabelText('Remove bob@x.com'))
        expect(onDeleteAccount).toHaveBeenCalledWith(ACCOUNTS[1])
    })

    it('shows an unread badge and a syncing cue per account', () => {
        const {accountRow} = renderSidebar({unreadByAccount: {a1: 5}, syncingAccountIds: new Set(['a2'])})
        expect(accountRow('a1')).toHaveTextContent('5')
        expect(accountRow('a2')).toHaveTextContent('Synchronising')
    })

    it('offers no reordering with a single account', () => {
        const {queryByLabelText, accountRow} = renderSidebar({accounts: [ACCOUNTS[0]], selectedAccount: 'a1'})
        expect(queryByLabelText('Move alice@x.com up')).toBeNull()
        expect(accountRow('a1')).not.toHaveAttribute('draggable', 'true')
    })

    it('reorders on a drag drop', () => {
        const {accountRow, onReorderAccounts} = renderSidebar()
        fireEvent.drop(accountRow('a1')!, {dataTransfer: makeDataTransfer({[accountDragType]: 'a2'})})
        expect(onReorderAccounts).toHaveBeenCalledWith(['a2', 'a1'])
    })
})

describe('Sidebar: folders section', () => {
    it('hides the folders section until an account is selected', () => {
        const {container} = renderSidebar({selectedAccount: ''})
        expect(container.textContent).toContain('Accounts')
        expect(container.textContent).not.toContain('Folders')
    })

    it('creates a new folder when management is allowed', () => {
        const {getByLabelText, onNewFolder} = renderSidebar({folders: [makeFolder('inbox', 'Inbox', 'inbox')]})
        fireEvent.click(getByLabelText('New folder'))
        expect(onNewFolder).toHaveBeenCalled()
    })

    it('hides the new-folder action for an account that cannot manage folders', () => {
        const {queryByLabelText} = renderSidebar({
            folders: [makeFolder('inbox', 'Inbox', 'inbox')], canManageFolders: false,
        })
        expect(queryByLabelText('New folder')).toBeNull()
    })

    it('prompts to sync when the account has no cached folders', () => {
        const {container} = renderSidebar({folders: []})
        expect(container.textContent).toContain('No folders cached')
    })
})

describe('Sidebar: folder tree', () => {
    const nested = [
        makeFolder('inbox', 'Inbox', 'inbox', {unread: 3}),
        makeFolder('work', 'Work', 'custom'),
        makeFolder('reports', 'Work/Reports', 'custom'),
    ]

    it('renders the folders with their leaf names and unread counts', () => {
        const {folderRow} = renderSidebar({folders: nested})
        expect(folderRow('inbox')).toHaveTextContent('Inbox')
        expect(folderRow('inbox')).toHaveTextContent('3')
        expect(folderRow('reports')).toHaveTextContent('Reports')
    })

    it('selects a folder on click', () => {
        const {folderRow, onSelectFolder} = renderSidebar({folders: nested})
        fireEvent.click(folderRow('work')!)
        expect(onSelectFolder).toHaveBeenCalledWith('work')
    })

    it('collapses and expands a parent, persisting the collapsed set', () => {
        const {folderRow, getByLabelText} = renderSidebar({folders: nested})
        expect(folderRow('reports')).not.toBeNull()
        fireEvent.click(getByLabelText('Collapse Work'))
        expect(folderRow('reports')).toBeNull()
        expect(localStorage.getItem(collapseKey('a1'))).toBe('["Work"]')
        fireEvent.click(getByLabelText('Expand Work'))
        expect(folderRow('reports')).not.toBeNull()
    })

    it('starts collapsed when the stored state says so', () => {
        localStorage.setItem(collapseKey('a1'), '["Work"]')
        const {folderRow} = renderSidebar({folders: nested})
        expect(folderRow('reports')).toBeNull()
    })

    it('rolls the unread hidden in the subtree up onto a collapsed parent', () => {
        const folders = [
            makeFolder('work', 'Work', 'custom', {unread: 1}),
            makeFolder('reports', 'Work/Reports', 'custom', {unread: 2}),
            makeFolder('archive2026', 'Work/Reports/2026', 'custom', {unread: 3}),
        ]
        localStorage.setItem(collapseKey('a1'), '["Work"]')
        const {folderRow} = renderSidebar({folders})
        const badge = folderRow('work')!.querySelector('.badge')!
        expect(badge).toHaveTextContent('6')
        expect(badge.classList.contains('badge-rollup')).toBe(true)
        expect(badge.getAttribute('title')).toBe('6 unread including subfolders')
    })

    it('reverts to the folder\'s own unread once the parent is expanded', () => {
        const folders = [
            makeFolder('work', 'Work', 'custom', {unread: 1}),
            makeFolder('reports', 'Work/Reports', 'custom', {unread: 2}),
        ]
        const {folderRow} = renderSidebar({folders})
        const badge = folderRow('work')!.querySelector('.badge')!
        expect(badge).toHaveTextContent('1')
        expect(badge.classList.contains('badge-rollup')).toBe(false)
        expect(folderRow('reports')!.querySelector('.badge')).toHaveTextContent('2')
    })

    it('shows no badge on a collapsed parent whose subtree has no unread', () => {
        const folders = [
            makeFolder('work', 'Work', 'custom'),
            makeFolder('reports', 'Work/Reports', 'custom'),
        ]
        localStorage.setItem(collapseKey('a1'), '["Work"]')
        const {folderRow} = renderSidebar({folders})
        expect(folderRow('work')!.querySelector('.badge')).toBeNull()
    })

    it('expands a collapsed parent with ArrowRight', () => {
        localStorage.setItem(collapseKey('a1'), '["Work"]')
        const {folderRow} = renderSidebar({folders: nested, selectedFolder: 'work'})
        expect(folderRow('reports')).toBeNull()
        fireEvent.keyDown(folderRow('work')!, {key: 'ArrowRight'})
        expect(folderRow('reports')).not.toBeNull()
    })

    it('spring-loads a collapsed parent when a message is dragged over it', () => {
        vi.useFakeTimers()
        try {
            localStorage.setItem(collapseKey('a1'), '["Work"]')
            const {folderRow} = renderSidebar({folders: nested})
            expect(folderRow('reports')).toBeNull()
            fireEvent.dragOver(folderRow('work')!, {dataTransfer: makeDataTransfer({[messageDragType]: 'm1'})})
            // Still collapsed until the hover delay elapses.
            expect(folderRow('reports')).toBeNull()
            act(() => {
                vi.advanceTimersByTime(1000)
            })
            // The parent auto-expanded, so its sub-folder is now visible to drop into.
            expect(folderRow('reports')).not.toBeNull()
        } finally {
            vi.useRealTimers()
        }
    })

    it('spring-loads a collapsed parent when a folder is dragged over it', () => {
        vi.useFakeTimers()
        try {
            const folders = [
                makeFolder('work', 'Work', 'custom'),
                makeFolder('reports', 'Work/Reports', 'custom'),
                makeFolder('personal', 'Personal', 'custom'),
            ]
            localStorage.setItem(collapseKey('a1'), '["Work"]')
            const {folderRow} = renderSidebar({folders})
            expect(folderRow('reports')).toBeNull()
            fireEvent.dragStart(folderRow('personal')!, {dataTransfer: makeDataTransfer()})
            fireEvent.dragOver(folderRow('work')!, {dataTransfer: makeDataTransfer({[folderDragType]: 'personal'})})
            expect(folderRow('reports')).toBeNull()
            act(() => {
                vi.advanceTimersByTime(1000)
            })
            expect(folderRow('reports')).not.toBeNull()
        } finally {
            vi.useRealTimers()
        }
    })

    it('renames and deletes a custom folder', () => {
        const {getByLabelText, onRenameFolder, onDeleteFolder} = renderSidebar({folders: nested})
        fireEvent.click(getByLabelText('Rename Work'))
        expect(onRenameFolder).toHaveBeenCalledWith(nested[1])
        fireEvent.click(getByLabelText('Delete Work'))
        expect(onDeleteFolder).toHaveBeenCalledWith(nested[1])
    })

    it('offers no rename or delete on a well-known mailbox', () => {
        const {folderRow} = renderSidebar({folders: nested})
        expect(within(folderRow('inbox')!).queryByLabelText('Rename Inbox')).toBeNull()
        expect(folderRow('inbox')).not.toHaveAttribute('draggable', 'true')
    })
})

describe('Sidebar: folder drag and drop', () => {
    const siblings = [
        makeFolder('work', 'Work', 'custom'),
        makeFolder('personal', 'Personal', 'custom'),
    ]

    // dropOn dispatches a drop carrying clientY as an own property, which testing-library's fireEvent.drop
    // does not set on a drag event (clientY is a read-only getter there, so it stays undefined and the zone
    // maths degenerates to NaN). With jsdom's zero-height rect the pointer offset equals clientY, so
    // clientY 0 lands in the into zone (nest inside) and a positive clientY in the after zone (same level).
    function dropOn(row: HTMLElement, dataTransfer: DataTransfer, clientY: number) {
        const event = new Event('drop', {bubbles: true, cancelable: true})
        Object.defineProperty(event, 'dataTransfer', {value: dataTransfer})
        Object.defineProperty(event, 'clientY', {value: clientY})
        fireEvent(row, event)
    }

    it('drops a message onto a folder without touching the folder move', () => {
        const {folderRow, onDropMessage, onReparentFolder} = renderSidebar({folders: siblings})
        dropOn(folderRow('work')!, makeDataTransfer({[messageDragType]: 'm1'}), 0)
        expect(onDropMessage).toHaveBeenCalledWith('m1', 'work')
        expect(onReparentFolder).not.toHaveBeenCalled()
    })

    it('reparents a folder dropped into another', () => {
        const {folderRow, onReparentFolder} = renderSidebar({folders: siblings})
        dropOn(folderRow('work')!, makeDataTransfer({[folderDragType]: 'personal'}), 0)
        expect(onReparentFolder).toHaveBeenCalledWith('personal', 'work')
    })

    it('reorders same-level folders locally, persisting the order and not moving on the server', () => {
        const {folderRow, onReparentFolder} = renderSidebar({folders: siblings})
        dropOn(folderRow('work')!, makeDataTransfer({[folderDragType]: 'personal'}), 10)
        expect(onReparentFolder).not.toHaveBeenCalled()
        expect(localStorage.getItem(folderOrderKey('a1'))).toBe('["Work","Personal"]')
    })
})

describe('Sidebar: unified mailbox entry', () => {
    it('is absent while the View tick is off', () => {
        const {container} = renderSidebar()
        expect(container.querySelector('[data-unified-entry]')).toBeNull()
    })

    it('shows the badged All-inboxes entry and selects the combined view on click', () => {
        const {container, onSelectUnified} = renderSidebar({
            unifiedEnabled: true, unifiedSelected: true, unifiedUnread: 7,
        })
        const entry = container.querySelector<HTMLElement>('[data-unified-entry] .list-item')
        expect(entry).not.toBeNull()
        expect(entry!.classList.contains('selected')).toBe(true)
        expect(entry!.textContent).toContain('All inboxes')
        expect(entry!.textContent).toContain('7')
        fireEvent.click(entry!)
        expect(onSelectUnified).toHaveBeenCalled()
    })

    it('hides the badge at zero unread and selects on Enter', () => {
        const {container, onSelectUnified} = renderSidebar({unifiedEnabled: true})
        const entry = container.querySelector<HTMLElement>('[data-unified-entry] .list-item')
        expect(entry!.querySelector('.badge')).toBeNull()
        expect(entry!.classList.contains('selected')).toBe(false)
        fireEvent.keyDown(entry!, {key: 'Enter'})
        expect(onSelectUnified).toHaveBeenCalled()
    })
})

describe('Sidebar: snoozed entry', () => {
    it('is absent while nothing is snoozed', () => {
        const {container} = renderSidebar()
        expect(container.querySelector('[data-snoozed-entry]')).toBeNull()
    })

    it('shows the badged Snoozed entry and opens the view on click', () => {
        const {container, onSelectSnoozed} = renderSidebar({snoozedCount: 3, snoozedSelected: true})
        const entry = container.querySelector<HTMLElement>('[data-snoozed-entry] .list-item')
        expect(entry).not.toBeNull()
        expect(entry!.classList.contains('selected')).toBe(true)
        expect(entry!.textContent).toContain('Snoozed')
        expect(entry!.textContent).toContain('3')
        fireEvent.click(entry!)
        expect(onSelectSnoozed).toHaveBeenCalled()
    })

    it('opens the view on Enter', () => {
        const {container, onSelectSnoozed} = renderSidebar({snoozedCount: 1})
        fireEvent.keyDown(container.querySelector<HTMLElement>('[data-snoozed-entry] .list-item')!, {key: 'Enter'})
        expect(onSelectSnoozed).toHaveBeenCalled()
    })
})
