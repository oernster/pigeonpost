// Characterisation test for the sidebar at its stable outer interface: its props and its observable
// behaviour. It renders the real Sidebar and drives each interaction, asserting the DOM plus which callback
// fired and what was written to localStorage. The interface it pins does not move as the sidebar is
// decomposed in Phase 2 (the persisted collapsed and order state, the account picker and the folder-tree
// drop split are lifted out beneath these same props), so this suite staying green is
// the proof each extraction preserved behaviour. The persisted folder state's durable home is the backend
// database (so it survives an application update) with localStorage as its warm cache; the two backend
// calls are mocked as benign no-ops below and the restore, write-through and migration tests assert
// against those mocks. The drag math is exercised through the real sidebarDnd and folderPaths modules.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, waitFor, within} from '@testing-library/react'
import type {ComponentProps} from 'react'
import {Sidebar} from './Sidebar'
import {api, type Account, type Folder} from '../api'
import {folderDragType} from '../sidebarDnd'
import {messageDragType} from './MessageList'

vi.mock('../api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../api')>()
    return {
        ...actual,
        api: {
            ...actual.api,
            folderUIState: vi.fn().mockResolvedValue({order: [], collapsed: []}),
            saveFolderUIState: vi.fn().mockResolvedValue(undefined),
        },
    }
})

type SidebarProps = ComponentProps<typeof Sidebar>

// These mirror the sidebar's private localStorage keys: the warm cache of the folder display state whose
// durable home is the backend store. They remain part of the persisted contract (the cache is what renders
// on the first paint), so pinning the exact keys is deliberate: a change here is a behaviour change.
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
        onNewFolder: vi.fn(),
        onRenameFolder: vi.fn(),
        onReparentFolder: vi.fn(),
        onDeleteFolder: vi.fn(),
        // The default drop is accepted, matching a real move being issued; a test wanting the rejected
        // path overrides it.
        onDropMessage: vi.fn().mockReturnValue(true),
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
        elsewhereCue: {unread: 0, accounts: 0},
        folders: [],
        selectedFolder: '',
        canManageFolders: true,
        ...handlers,
        ...overrides,
    }
    const view = render(<Sidebar {...props}/>)
    // The picker is a listbox: the trigger shows the active account and opens the list, whose options are
    // read back by their accessible name (the badge and the syncing caption do not read on their own, so
    // each option spells them out in an aria-label).
    const accountTrigger = () => view.container.querySelector<HTMLButtonElement>('.account-trigger')!
    const accountOptions = () => Array.from(view.container.querySelectorAll<HTMLElement>('.account-option'))
    const accountOptionLabels = () => accountOptions().map((o) => o.getAttribute('aria-label'))
    const folderRow = (id: string) => view.container.querySelector<HTMLElement>(`[data-folder-id="${id}"]`)
    return {...view, ...handlers, accountTrigger, accountOptions, accountOptionLabels, folderRow}
}

beforeEach(() => {
    localStorage.clear()
    // Clears recorded calls on the module-level api mocks (their benign resolved implementations are
    // kept), so each test asserts only its own backend traffic.
    vi.clearAllMocks()
})
afterEach(() => cleanup())

describe('Sidebar: shell', () => {
    it('shows the empty state when there are no accounts', () => {
        const {container} = renderSidebar({accounts: []})
        expect(container.querySelector('[data-account-picker]')).toBeNull()
        expect(container.textContent).toContain('No accounts yet')
    })

    it('lists every account in one dropdown, showing the name and the address', () => {
        const {accountTrigger, accountOptionLabels} = renderSidebar()
        fireEvent.click(accountTrigger())
        expect(accountOptionLabels()).toEqual(['Alice (alice@x.com)', 'Bob (bob@x.com)'])
    })

    it('scrolls the folders alone, with the brand, the picker and the Folders header pinned', () => {
        const {container, accountTrigger, folderRow} = renderSidebar({
            folders: [makeFolder('inbox', 'Inbox', 'inbox')],
        })
        const brand = container.querySelector('.sidebar-brand')!
        expect(brand.parentElement!.classList.contains('sidebar')).toBe(true)
        expect(brand.closest('.sidebar-scroll')).toBeNull()
        expect(accountTrigger().closest('.sidebar-scroll')).toBeNull()
        expect(container.querySelector('.section-header')!.closest('.sidebar-scroll')).toBeNull()
        expect(folderRow('inbox')!.closest('.sidebar-scroll')).not.toBeNull()
    })
})

describe('Sidebar: account picker', () => {
    it('shows the selected account and switches on a pick', () => {
        const {accountTrigger, accountOptions, onSelectAccount} = renderSidebar()
        expect(accountTrigger().textContent).toContain('Alice (alice@x.com)')
        fireEvent.click(accountTrigger())
        fireEvent.mouseDown(accountOptions()[1])
        expect(onSelectAccount).toHaveBeenCalledWith('a2')
    })

    // The reason the picker is not a native select: a select is silent when you re-pick the option already
    // showing, so choosing the account you are already on did nothing at all, when it should take you back
    // to that account's inbox.
    it('reports the account already showing when it is picked again', () => {
        const {accountTrigger, accountOptions, onSelectAccount} = renderSidebar()
        fireEvent.click(accountTrigger())
        fireEvent.mouseDown(accountOptions()[0])
        expect(onSelectAccount).toHaveBeenCalledWith('a1')
    })

    it('closes the list once an account has been picked', () => {
        const {accountTrigger, accountOptions} = renderSidebar()
        fireEvent.click(accountTrigger())
        expect(accountOptions()).toHaveLength(2)
        fireEvent.mouseDown(accountOptions()[0])
        expect(accountOptions()).toHaveLength(0)
    })

    it('falls back to the first account before a selection settles', () => {
        const {accountTrigger} = renderSidebar({selectedAccount: ''})
        expect(accountTrigger().textContent).toContain('Alice (alice@x.com)')
    })

    it('carries the unread count and the syncing cue in the option label', () => {
        const {accountTrigger, accountOptionLabels} = renderSidebar({
            unreadByAccount: {a1: 5}, syncingAccountIds: new Set(['a2']),
        })
        fireEvent.click(accountTrigger())
        expect(accountOptionLabels()[0]).toBe('Alice (alice@x.com) - 5 unread')
        expect(accountOptionLabels()[1]).toBe('Bob (bob@x.com) - synchronising')
    })

    // The unread count is the same yellow badge the folder rows carry, not words in the row, so the two
    // read as the same thing. The words stay in the accessible label above.
    it('badges the unread count on the trigger and in the list', () => {
        const {accountTrigger, accountOptions} = renderSidebar({unreadByAccount: {a1: 5}})
        expect(within(accountTrigger()).getByText('5').classList.contains('badge')).toBe(true)
        fireEvent.click(accountTrigger())
        expect(within(accountOptions()[0]).getByText('5')).toBeInTheDocument()
    })

    // The elsewhere badge cues mail newly arrived on the accounts the closed picker hides. Outlined
    // (badge-elsewhere) so it reads as "not here", with the words in the tooltip and the trigger's
    // accessible name; the per-account breakdown is the open list's own badges.
    it('badges newly arrived mail on other accounts against the closed trigger', () => {
        const {accountTrigger} = renderSidebar({
            unreadByAccount: {a2: 4}, elsewhereCue: {unread: 4, accounts: 1},
        })
        const badge = within(accountTrigger()).getByText('4')
        expect(badge.classList.contains('badge-elsewhere')).toBe(true)
        expect(badge.getAttribute('title')).toBe('4 unread on 1 other account')
        expect(accountTrigger().getAttribute('aria-label'))
            .toBe('Active account, 4 unread on 1 other account')
    })

    it('shows no elsewhere badge when nothing new arrived on other accounts', () => {
        const {accountTrigger} = renderSidebar({unreadByAccount: {a2: 4}})
        expect(accountTrigger().querySelector('.badge-elsewhere')).toBeNull()
        expect(accountTrigger().getAttribute('aria-label')).toBe('Active account')
    })

    it('names an account by its address alone when the display name adds nothing', () => {
        const same = makeAccount('a3', 'sam@x.com', 'sam@x.com')
        const {accountTrigger} = renderSidebar({accounts: [same], selectedAccount: 'a3'})
        expect(accountTrigger().textContent).toContain('sam@x.com')
    })

    // The picker stays one focus-ring stop: focus rests on the trigger and Up/Down walk the options from
    // there, wrapping at the ends, with Enter picking and Escape dismissing.
    it('walks the options with Up and Down and picks with Enter', () => {
        const {accountTrigger, onSelectAccount} = renderSidebar()
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        fireEvent.keyDown(accountTrigger(), {key: 'Enter'})
        expect(onSelectAccount).toHaveBeenCalledWith('a2')
    })

    it('wraps the cursor past the last account', () => {
        const {accountTrigger, onSelectAccount} = renderSidebar({selectedAccount: 'a2'})
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        fireEvent.keyDown(accountTrigger(), {key: 'Enter'})
        expect(onSelectAccount).toHaveBeenCalledWith('a1')
    })

    it('dismisses the list on Escape without picking', () => {
        const {accountTrigger, accountOptions, onSelectAccount} = renderSidebar()
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        expect(accountOptions()).toHaveLength(2)
        fireEvent.keyDown(accountTrigger(), {key: 'Escape'})
        expect(accountOptions()).toHaveLength(0)
        expect(onSelectAccount).not.toHaveBeenCalled()
    })

    // Left and Right step the focus ring out of the picker, so the list must not be left open behind it.
    it('dismisses the list when the ring steps away with Left or Right', () => {
        const {accountTrigger, accountOptions} = renderSidebar()
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        expect(accountOptions()).toHaveLength(2)
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowRight'})
        expect(accountOptions()).toHaveLength(0)
    })

    it('dismisses the list when Tab leaves the picker', () => {
        const {accountTrigger, accountOptions} = renderSidebar()
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        fireEvent.keyDown(accountTrigger(), {key: 'Tab'})
        expect(accountOptions()).toHaveLength(0)
    })

    it('jumps the cursor to the ends with Home and End', () => {
        const {accountTrigger, onSelectAccount} = renderSidebar()
        fireEvent.keyDown(accountTrigger(), {key: 'ArrowDown'})
        fireEvent.keyDown(accountTrigger(), {key: 'End'})
        fireEvent.keyDown(accountTrigger(), {key: ' '})
        expect(onSelectAccount).toHaveBeenCalledWith('a2')
    })

    it('closes the open list when the trigger is pressed again', () => {
        const {accountTrigger, accountOptions} = renderSidebar()
        fireEvent.click(accountTrigger())
        expect(accountOptions()).toHaveLength(2)
        fireEvent.click(accountTrigger())
        expect(accountOptions()).toHaveLength(0)
    })

    it('dismisses the list on a press outside it', () => {
        const {accountTrigger, accountOptions} = renderSidebar()
        fireEvent.click(accountTrigger())
        expect(accountOptions()).toHaveLength(2)
        fireEvent.mouseDown(document.body)
        expect(accountOptions()).toHaveLength(0)
    })

    it('edits and removes the account showing in the picker', () => {
        const {getByLabelText, onEditAccount, onDeleteAccount} = renderSidebar({selectedAccount: 'a2'})
        fireEvent.click(getByLabelText('Edit bob@x.com'))
        expect(onEditAccount).toHaveBeenCalledWith(ACCOUNTS[1])
        fireEvent.click(getByLabelText('Remove bob@x.com'))
        expect(onDeleteAccount).toHaveBeenCalledWith(ACCOUNTS[1])
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

    it('flashes the folder a message landed in, so the right folder is obvious', async () => {
        const {folderRow} = renderSidebar({folders: siblings})
        dropOn(folderRow('work')!, makeDataTransfer({[messageDragType]: 'm1'}), 0)
        // The class is applied on the next frame, so the animation restarts on a repeat drop.
        await waitFor(() => expect(folderRow('work')!.classList.contains('drop-landed')).toBe(true))
        expect(folderRow('personal')!.classList.contains('drop-landed')).toBe(false)
    })

    it('does not flash a drop the app skipped', async () => {
        const onDropMessage = vi.fn().mockReturnValue(false)
        const {folderRow} = renderSidebar({folders: siblings, onDropMessage})
        dropOn(folderRow('work')!, makeDataTransfer({[messageDragType]: 'm1'}), 0)
        await waitFor(() => expect(onDropMessage).toHaveBeenCalled())
        expect(folderRow('work')!.classList.contains('drop-landed')).toBe(false)
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

    it('mirrors a reorder to the backend store, sending the full state', async () => {
        const {folderRow} = renderSidebar({folders: siblings})
        dropOn(folderRow('work')!, makeDataTransfer({[folderDragType]: 'personal'}), 10)
        await waitFor(() => {
            expect(api.saveFolderUIState).toHaveBeenCalledWith('a1', ['Work', 'Personal'], [])
        })
    })

    it('mirrors a collapse to the backend store, sending the full state', async () => {
        const {getByLabelText} = renderSidebar({folders: siblings.concat(makeFolder('reports', 'Work/Reports', 'custom'))})
        fireEvent.click(getByLabelText('Collapse Work'))
        await waitFor(() => {
            expect(api.saveFolderUIState).toHaveBeenCalledWith('a1', [], ['Work'])
        })
    })

    // The first launch after an application update: the WebView profile (and with it localStorage) has
    // been wiped while the backend row survived. The tree restores from it and re-warms the cache.
    it('restores order and collapse from the backend when localStorage is empty', async () => {
        vi.mocked(api.folderUIState).mockResolvedValueOnce({order: ['Personal', 'Work'], collapsed: ['Work']})
        const {folderRow, container} = renderSidebar({
            folders: siblings.concat(makeFolder('reports', 'Work/Reports', 'custom')),
        })
        await waitFor(() => expect(folderRow('reports')).toBeNull())
        const rows = Array.from(container.querySelectorAll<HTMLElement>('[data-folder-id]'))
        expect(rows.map((r) => r.getAttribute('data-folder-id'))).toEqual(['personal', 'work'])
        expect(localStorage.getItem(folderOrderKey('a1'))).toBe('["Personal","Work"]')
        expect(localStorage.getItem(collapseKey('a1'))).toBe('["Work"]')
    })

    // The first launch after this feature shipped: the state exists only in localStorage. It is pushed
    // up to the backend once, so the next application update cannot lose it.
    it('migrates cached-only state up to the backend', async () => {
        localStorage.setItem(folderOrderKey('a1'), '["Personal","Work"]')
        localStorage.setItem(collapseKey('a1'), '["Work"]')
        renderSidebar({folders: siblings})
        await waitFor(() => {
            expect(api.saveFolderUIState).toHaveBeenCalledWith('a1', ['Personal', 'Work'], ['Work'])
        })
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

describe('Sidebar: the folder row action toolbar', () => {
    it('keeps the hover toolbar on its own row rather than over the next row', async () => {
        // The toolbar is absolutely positioned, so where it lands is a stylesheet fact rather than a
        // DOM one: jsdom lays nothing out and Vitest serves CSS imports as empty modules, so the rule
        // is pinned by reading the stylesheet from disk (the same route as App.test.tsx's drag rule).
        // Anchored below the row at its left edge (top: 100%, left: 14px) the toolbar sat exactly over
        // the NEXT row's expand/collapse chevron: two controls on the same pixels, so which one a
        // click reached was close to chance. Measured on the real stylesheets, the toolbar occupied
        // x 16 to 74 / y 48 to 78 while that chevron held x 16 to 36 / y 56 to 88.
        const fsModule = 'node:' + 'fs'
        const {readFileSync} = (await import(fsModule)) as {readFileSync: (path: string, encoding: string) => string}
        const css = readFileSync('src/styles/accounts-and-folders.css', 'utf8')
        const rule = css.slice(css.indexOf('.account-actions {'))
        const block = rule.slice(0, rule.indexOf('}'))
        expect(block).toMatch(/position:\s*absolute;/)
        expect(block).toMatch(/right:\s*\d/)
        expect(block).not.toMatch(/top:\s*100%/)
        expect(block).not.toMatch(/left:/)
    })
})
