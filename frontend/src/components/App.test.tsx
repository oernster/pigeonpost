// Characterization test for App at its stable outer interface. App is the root component and takes no
// props, so its interface is purely its observable behaviour: what it renders and which api calls fire in
// response to mount and the core user gestures. This suite pins that behaviour BEFORE Phase 3 decomposes App
// into hooks and sub-components (useMessageStore, useSelection, useFolders, useAccounts, TitleBar, AppModals
// and the rest). None of those extractions change what App does on screen, so this suite staying green is the
// proof each one preserved behaviour, exactly as the modal characterization tests were the proof in Phase 2.
//
// ../api is stubbed (the one Wails seam) and ../wailsjs/runtime is stubbed for Environment and EventsOn (the
// only two runtime bindings the tree reads). The pure modules (messageText, shortcuts, threads, outbox,
// tagColours, theme, focusRing) are real and run as-is. Every method fired on mount is given a safe default
// in beforeEach, so a test overrides only what it exercises; without those defaults App throws on mount.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor, within} from '@testing-library/react'
import App from '../App'
import type {Account, Folder, Message, OutboxItem} from '../api'

const apiSpies = vi.hoisted(() => ({
    version: vi.fn(), author: vi.fn(),
    listAccounts: vi.fn(), reorderAccounts: vi.fn(),
    draftRecovery: vi.fn(), clearDraftRecovery: vi.fn(),
    listRules: vi.fn(), listContacts: vi.fn(), listEvents: vi.fn(),
    unreadCounts: vi.fn(), listTags: vi.fn(), saveTag: vi.fn(),
    messageTags: vi.fn(), messageBody: vi.fn(), searchMessages: vi.fn(),
    setMessageTag: vi.fn(), listMessages: vi.fn(), syncFolder: vi.fn(),
    listFolders: vi.fn(), listOutbox: vi.fn(), cancelOutboxItem: vi.fn(),
    syncAccount: vi.fn(), replayOutbox: vi.fn(), removeAccount: vi.fn(),
    deleteMessage: vi.fn(), deleteMessagePermanent: vi.fn(), saveMessageAs: vi.fn(),
    markFlagged: vi.fn(), moveMessage: vi.fn(), markJunk: vi.fn(), copyMessage: vi.fn(),
    createFolder: vi.fn(), renameFolder: vi.fn(), deleteFolder: vi.fn(), moveFolder: vi.fn(),
    pickAttachments: vi.fn(), about: vi.fn(), licence: vi.fn(), openReleases: vi.fn(),
    markRead: vi.fn(), moveMessages: vi.fn(), deleteMessagesPermanent: vi.fn(),
    deleteMessages: vi.fn(), showDefaultAppSettings: vi.fn(), minimiseToTray: vi.fn(),
    requestQuit: vi.fn(),
}))

// The runtime seam: Environment resolves the platform (App reads env.platform) and EventsOn subscribes to a
// backend event and MUST return an unsubscribe function, because every listener effect calls it on cleanup
// (and ReminderNotifications returns EventsOn(...) directly as its cleanup).
const runtimeSpies = vi.hoisted(() => ({
    Environment: vi.fn(),
    EventsOn: vi.fn(),
}))

// EventScope is provided with its real integer values because App imports CalendarModal (which reads the
// enum), even though the calendar is never opened in these tests.
vi.mock('../api', () => ({
    api: apiSpies,
    EventScope: {This: 0, Future: 1, All: 2},
}))
// The runtime lives at frontend/wailsjs/runtime, which App reaches as ../wailsjs/runtime from src/; from
// this test one level deeper it is ../../wailsjs/runtime, the same absolute module both App and
// ReminderNotifications import.
vi.mock('../../wailsjs/runtime', () => ({
    Environment: runtimeSpies.Environment,
    EventsOn: runtimeSpies.EventsOn,
}))

function makeAccount(overrides: Partial<Account> = {}): Account {
    return {
        id: 'acc1', displayName: 'Me', email: 'me@example.com', protocol: 'imap',
        inHost: 'imap.example.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.example.com', outPort: 587, outSecurity: 'starttls',
        signature: '', auth: 'password', identities: [],
        ...overrides,
    } as Account
}

function makeFolder(id: string, name: string, kind: string, overrides: Partial<Folder> = {}): Folder {
    return {id, accountId: 'acc1', path: name, name, kind, unread: 0, total: 0, ...overrides}
}

function makeMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1', folderId: 'inbox', subject: 'Weekly report',
        fromName: 'Alice Example', fromAddress: 'alice@example.com',
        to: [{name: 'Me', address: 'me@example.com'}], cc: [],
        date: '2026-07-11T10:00:00.000Z', size: 1024, read: false, flagged: false,
        hasAttachments: false, snippet: 'A short snippet', tagColours: [],
        ...overrides,
    } as Message
}

function makeOutboxItem(overrides: Partial<OutboxItem> = {}): OutboxItem {
    return {
        id: 'ob1', accountId: 'acc1', to: ['bob@example.com'], subject: 'Queued note',
        body: 'Body text', failed: false, failure: '', createdMs: 0,
        ...overrides,
    } as OutboxItem
}

// Fill every mount-fired method with a safe default. selectAccount opens the first account's inbox on load,
// so listFolders and listMessages are given empty defaults here and overridden per test where the cascade
// matters.
beforeEach(() => {
    localStorage.clear()
    apiSpies.version.mockReset().mockResolvedValue('1.0.0')
    apiSpies.author.mockReset().mockResolvedValue('Oliver')
    apiSpies.listAccounts.mockReset().mockResolvedValue([])
    apiSpies.reorderAccounts.mockReset().mockResolvedValue(undefined)
    apiSpies.draftRecovery.mockReset().mockResolvedValue({
        present: false, accountId: '', to: '', cc: '', bcc: '', subject: '', bodyHtml: '', savedMs: 0,
    })
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.listRules.mockReset().mockResolvedValue([])
    apiSpies.listContacts.mockReset().mockResolvedValue([])
    apiSpies.listEvents.mockReset().mockResolvedValue([])
    apiSpies.unreadCounts.mockReset().mockResolvedValue({total: 0, byAccount: {}})
    apiSpies.listTags.mockReset().mockResolvedValue([])
    apiSpies.saveTag.mockReset().mockResolvedValue(undefined)
    apiSpies.messageTags.mockReset().mockResolvedValue([])
    apiSpies.messageBody.mockReset().mockResolvedValue({plain: '', html: '', hasInvite: false, attachments: []})
    apiSpies.searchMessages.mockReset().mockResolvedValue([])
    apiSpies.setMessageTag.mockReset().mockResolvedValue(undefined)
    apiSpies.listMessages.mockReset().mockResolvedValue([])
    apiSpies.syncFolder.mockReset().mockResolvedValue(undefined)
    apiSpies.listFolders.mockReset().mockResolvedValue([])
    apiSpies.listOutbox.mockReset().mockResolvedValue([])
    apiSpies.cancelOutboxItem.mockReset().mockResolvedValue(undefined)
    apiSpies.syncAccount.mockReset().mockResolvedValue(undefined)
    apiSpies.replayOutbox.mockReset().mockResolvedValue(0)
    apiSpies.removeAccount.mockReset().mockResolvedValue(undefined)
    apiSpies.deleteMessage.mockReset().mockResolvedValue(undefined)
    apiSpies.deleteMessagePermanent.mockReset().mockResolvedValue(undefined)
    apiSpies.saveMessageAs.mockReset().mockResolvedValue(undefined)
    apiSpies.markFlagged.mockReset().mockResolvedValue(undefined)
    apiSpies.moveMessage.mockReset().mockResolvedValue(undefined)
    apiSpies.markJunk.mockReset().mockResolvedValue(undefined)
    apiSpies.copyMessage.mockReset().mockResolvedValue(undefined)
    apiSpies.createFolder.mockReset().mockResolvedValue(undefined)
    apiSpies.renameFolder.mockReset().mockResolvedValue(undefined)
    apiSpies.deleteFolder.mockReset().mockResolvedValue(undefined)
    apiSpies.moveFolder.mockReset().mockResolvedValue(undefined)
    apiSpies.pickAttachments.mockReset().mockResolvedValue([])
    apiSpies.about.mockReset().mockResolvedValue({})
    apiSpies.licence.mockReset().mockResolvedValue('')
    apiSpies.openReleases.mockReset().mockResolvedValue(undefined)
    apiSpies.markRead.mockReset().mockResolvedValue(undefined)
    apiSpies.moveMessages.mockReset().mockResolvedValue({ids: [], failed: 0, error: ''})
    apiSpies.deleteMessagesPermanent.mockReset().mockResolvedValue({ids: [], failed: 0, error: ''})
    apiSpies.deleteMessages.mockReset().mockResolvedValue({ids: [], failed: 0, error: ''})
    apiSpies.showDefaultAppSettings.mockReset().mockResolvedValue(undefined)
    apiSpies.minimiseToTray.mockReset().mockResolvedValue(undefined)
    apiSpies.requestQuit.mockReset().mockResolvedValue(undefined)
    runtimeSpies.Environment.mockReset().mockResolvedValue({platform: 'windows'})
    runtimeSpies.EventsOn.mockReset().mockReturnValue(() => undefined)
})

afterEach(() => cleanup())

describe('App: mount and splash', () => {
    it('renders the titlebar and shows the splash on launch', () => {
        const {container} = render(<App/>)
        expect(container.querySelector('.splash')).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Mail'})).toBeInTheDocument()
        expect(apiSpies.listAccounts).toHaveBeenCalled()
    })

    it('shows the welcome empty-state after the splash when there are no accounts', async () => {
        render(<App/>)
        // The empty-state is gated on the splash having gone (a 2s timer), so wait past it.
        await waitFor(
            () => expect(screen.getByText('Welcome to PigeonPost')).toBeInTheDocument(),
            {timeout: 3000},
        )
        expect(screen.getByText(/Add a mail account to start/)).toBeInTheDocument()
    })
})

describe('App: account and folder cascade', () => {
    it('auto-selects the first account on load and opens its inbox', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledWith('acc1'))
        expect(await screen.findByText('Weekly report')).toBeInTheDocument()
        expect(apiSpies.listMessages).toHaveBeenCalledWith('inbox')
        expect(apiSpies.syncFolder).toHaveBeenCalledWith('inbox')
    })

    it('loads a folder\'s messages when a different folder is selected', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([
            makeFolder('inbox', 'Inbox', 'inbox'),
            makeFolder('archive', 'Archive', 'custom'),
        ])
        apiSpies.listMessages.mockImplementation((id: string) =>
            Promise.resolve(id === 'archive'
                ? [makeMessage({id: 'a1', folderId: 'archive', subject: 'Archived item'})]
                : [makeMessage({subject: 'Weekly report'})]))
        const {container} = render(<App/>)
        expect(await screen.findByText('Weekly report')).toBeInTheDocument()
        fireEvent.click(container.querySelector('[data-folder-id="archive"]')!)
        await waitFor(() => expect(apiSpies.listMessages).toHaveBeenCalledWith('archive'))
        expect(await screen.findByText('Archived item')).toBeInTheDocument()
    })
})

describe('App: reading a message', () => {
    it('fetches and shows the body when a message is selected (reading pane on)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        await waitFor(() => expect(apiSpies.messageBody).toHaveBeenCalledWith('m1'))
        // The reading pane on the right shows the selected message, so its reply control appears. Scope to
        // the reader pane, since the titlebar also carries a Reply control with the same accessible name.
        const reader = container.querySelector('.reader') as HTMLElement
        expect(within(reader).getByRole('button', {name: 'Reply'})).toBeInTheDocument()
        expect(apiSpies.messageTags).toHaveBeenCalledWith('m1')
    })

    it('toggles the reading pane off from the View menu', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        const {container} = render(<App/>)
        await screen.findByText('Weekly report')
        expect(container.querySelector('.panes.no-preview')).not.toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'View'}))
        fireEvent.click(screen.getByRole('menuitemcheckbox', {name: 'Reading pane'}))
        await waitFor(() => expect(container.querySelector('.panes.no-preview')).toBeInTheDocument())
        expect(localStorage.getItem('pigeonpost.readingPane')).toBe('off')
    })

    // useReaderTabs (Phase 3.5) also owns opening a message in its own reader tab and closing it. The
    // reading-pane toggle above already pins togglePreview and the persisted preference; this pins the tab
    // open and close path.
    it('opens a message in a reader tab and closes it (openInNewTab / closeTab)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        // Open in new tab pins the message as a reader tab, whose close cross is labelled Close <subject>.
        fireEvent.click(screen.getByRole('button', {name: 'Mail'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Open in new tab'}))
        const closeButton = await screen.findByRole('button', {name: 'Close Weekly report'})
        expect(closeButton).toBeInTheDocument()
        // Closing the tab removes it from the strip.
        fireEvent.click(closeButton)
        await waitFor(() => expect(screen.queryByRole('button', {name: 'Close Weekly report'})).not.toBeInTheDocument())
    })
})

describe('App: deleting a message', () => {
    it('confirms before deleting the selected message, then calls the delete api', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        // The reader's Delete control asks for confirmation rather than deleting straight away.
        fireEvent.click(await screen.findByRole('button', {name: 'Delete'}))
        const dialog = await screen.findByRole('alertdialog', {name: 'Delete message'})
        expect(dialog).toBeInTheDocument()
        fireEvent.click(within(dialog).getByRole('button', {name: 'Delete'}))
        await waitFor(() => expect(apiSpies.deleteMessage).toHaveBeenCalledWith('m1'))
        await waitFor(() => expect(screen.queryByText('Weekly report')).not.toBeInTheDocument())
    })
})

// These two pin the coupled-lists behaviour that Phase 3.1 moves into useMessageStore: an in-place field
// change flows to the message wherever it appears (applyToAllLists) and a removal drops it from the lists
// (removeFromAllLists). The extraction must keep both identical.
describe('App: the coupled message lists', () => {
    it('marks a message read when it is opened, updating the row in step (applyToAllLists)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report', read: false})])
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        await waitFor(() => expect(apiSpies.markRead).toHaveBeenCalledWith('m1', true))
        await waitFor(() => expect(container.querySelector('[data-mid="m1"]')).not.toHaveClass('unread'))
    })

    it('bulk-deletes the selected messages, dropping them from the list (removeFromAllLists)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([
            makeMessage({id: 'm1', subject: 'Weekly report'}),
            makeMessage({id: 'm2', subject: 'Second message'}),
        ])
        apiSpies.deleteMessages.mockResolvedValue({ids: ['m1', 'm2'], failed: 0, error: ''})
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        // A Ctrl-click adds the second message, so the multi-selection summary replaces the reader.
        fireEvent.click(screen.getByText('Second message'), {ctrlKey: true})
        fireEvent.click(await screen.findByRole('button', {name: 'Delete'}))
        const dialog = await screen.findByRole('alertdialog', {name: 'Delete messages'})
        fireEvent.click(within(dialog).getByRole('button', {name: 'Delete 2'}))
        await waitFor(() => expect(apiSpies.deleteMessages).toHaveBeenCalledWith(['m1', 'm2']))
        await waitFor(() => expect(screen.queryByText('Weekly report')).not.toBeInTheDocument())
        expect(screen.queryByText('Second message')).not.toBeInTheDocument()
    })
})

// The Ctrl-toggle gesture (toggleId) is exercised by the bulk-delete test above. This pins the other half,
// the Shift range (rangeIds), the behaviour Phase 3.2 moves into useSelection.
describe('App: multi-selection gestures', () => {
    it('Shift-click selects the contiguous range from the anchor (rangeIds)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        // Distinct descending dates fix the newest-first list order at [m1, m2, m3].
        apiSpies.listMessages.mockResolvedValue([
            makeMessage({id: 'm1', subject: 'First', date: '2026-07-11T10:03:00.000Z'}),
            makeMessage({id: 'm2', subject: 'Second', date: '2026-07-11T10:02:00.000Z'}),
            makeMessage({id: 'm3', subject: 'Third', date: '2026-07-11T10:01:00.000Z'}),
        ])
        render(<App/>)
        // Click the first row to set the anchor, then Shift-click the third to range across all three.
        fireEvent.click(await screen.findByText('First'))
        fireEvent.click(screen.getByText('Third'), {shiftKey: true})
        expect(await screen.findByText('3 messages selected')).toBeInTheDocument()
    })
})

// The single-message actions that Phase 3.3 moves into useMessageActions. Delete and read are already
// covered above; these pin flag, move and junk.
describe('App: single-message actions', () => {
    it('stars a message from its row (toggleFlag)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report', flagged: false})])
        render(<App/>)
        await screen.findByText('Weekly report')
        // The row star toggles the flag without selecting the message.
        fireEvent.click(screen.getByRole('button', {name: 'Add star'}))
        await waitFor(() => expect(apiSpies.markFlagged).toHaveBeenCalledWith('m1', true))
    })

    it('moves a message via the Mail menu Move to submenu (moveMessage)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([
            makeFolder('inbox', 'Inbox', 'inbox'),
            makeFolder('archive', 'Archive', 'custom'),
        ])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'Mail'}))
        // Enter opens the Move to flyout, then its Archive child fires the move.
        fireEvent.keyDown(screen.getByRole('menuitem', {name: 'Move to'}), {key: 'Enter'})
        fireEvent.click(screen.getByRole('menuitem', {name: 'Archive'}))
        await waitFor(() => expect(apiSpies.moveMessage).toHaveBeenCalledWith('m1', 'archive'))
        await waitFor(() => expect(screen.queryByText('Weekly report')).not.toBeInTheDocument())
    })

    it('marks a message as junk from the Mail menu (markJunk)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'Mail'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Mark as junk'}))
        await waitFor(() => expect(apiSpies.markJunk).toHaveBeenCalledWith('m1'))
        await waitFor(() => expect(screen.queryByText('Weekly report')).not.toBeInTheDocument())
    })
})

// The bulk actions over a multi-selection that Phase 3.4 moves into useBulkActions. Bulk delete is already
// covered above (the removeFromAllLists test); this pins the bulk read/unread path (bulkSetRead).
describe('App: bulk actions', () => {
    it('bulk-marks the selected messages unread from the summary (bulkSetRead)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([
            makeMessage({id: 'm1', subject: 'Weekly report'}),
            makeMessage({id: 'm2', subject: 'Second message'}),
        ])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        // A Ctrl-click adds the second message, so the multi-selection summary replaces the reader.
        fireEvent.click(screen.getByText('Second message'), {ctrlKey: true})
        // Mark unread persists read=false for each selected message. Opening a message auto-marks it read
        // (always true), so asserting the false calls pins the bulk action rather than that auto-read.
        fireEvent.click(await screen.findByRole('button', {name: 'Mark unread'}))
        await waitFor(() => expect(apiSpies.markRead).toHaveBeenCalledWith('m1', false))
        expect(apiSpies.markRead).toHaveBeenCalledWith('m2', false)
    })
})

// The outbox that Phase 3.6 moves into useOutbox: the queue is loaded on mount and, while the selected
// account has queued mail, a synthetic Outbox folder (id __outbox__) appears in the sidebar.
describe('App: the outbox', () => {
    it('surfaces a synthetic Outbox folder when the account has queued mail (useOutbox)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listOutbox.mockResolvedValue([makeOutboxItem({accountId: 'acc1'})])
        const {container} = render(<App/>)
        await waitFor(() => expect(container.querySelector('[data-folder-id="__outbox__"]')).toBeInTheDocument())
    })
})

// The folder create/rename/delete/reparent flow that Phase 3.7 moves into useFolders. A custom folder's row
// carries a Delete button; confirming it calls the delete api.
describe('App: folder management', () => {
    it('deletes a custom folder through the confirm dialog (useFolders)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([
            makeFolder('inbox', 'Inbox', 'inbox'),
            makeFolder('archive', 'Archive', 'custom'),
        ])
        render(<App/>)
        // The custom folder row carries a Delete <name> button; it asks for confirmation before deleting.
        fireEvent.click(await screen.findByRole('button', {name: 'Delete Archive'}))
        const dialog = await screen.findByRole('alertdialog', {name: 'Delete folder'})
        fireEvent.click(within(dialog).getByRole('button', {name: 'Delete folder'}))
        await waitFor(() => expect(apiSpies.deleteFolder).toHaveBeenCalledWith('archive'))
    })
})

// The account list and the load/reorder/remove operations that Phase 3.8 moves into useAccounts. An account
// row carries a Remove button; confirming it calls the remove api.
describe('App: account management', () => {
    it('removes an account through the confirm dialog (useAccounts)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        render(<App/>)
        // The account row carries a Remove <email> button; it asks for confirmation before removing.
        fireEvent.click(await screen.findByRole('button', {name: 'Remove me@example.com'}))
        const dialog = await screen.findByRole('alertdialog', {name: 'Remove account'})
        fireEvent.click(within(dialog).getByRole('button', {name: 'Remove account'}))
        await waitFor(() => expect(apiSpies.removeAccount).toHaveBeenCalledWith('acc1'))
    })
})

// The mailbox sync that Phase 3.9 moves into useSync. The titlebar Sync button syncs the selected account.
describe('App: syncing', () => {
    it('syncs the selected account from the titlebar Sync button (useSync)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        render(<App/>)
        // Wait for the account to auto-select so the Sync control is enabled.
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledWith('acc1'))
        fireEvent.click(screen.getByRole('button', {name: 'Sync'}))
        await waitFor(() => expect(apiSpies.syncAccount).toHaveBeenCalledWith('acc1'))
    })
})
