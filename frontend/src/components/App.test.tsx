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
import {act, cleanup, fireEvent, render, screen, waitFor, within} from '@testing-library/react'
import App from '../App'
import type {Account, Folder, Message, OutboxItem} from '../api'
import {SEARCH_MATCH_END, SEARCH_MATCH_START} from '../api'

const apiSpies = vi.hoisted(() => ({
    version: vi.fn(), author: vi.fn(),
    listAccounts: vi.fn(), reorderAccounts: vi.fn(),
    draftRecovery: vi.fn(), clearDraftRecovery: vi.fn(),
    listRules: vi.fn(), listContacts: vi.fn(), listEvents: vi.fn(),
    unreadCounts: vi.fn(), listTags: vi.fn(), saveTag: vi.fn(),
    messageTags: vi.fn(), messageBody: vi.fn(), loadRemoteImages: vi.fn(), searchMessages: vi.fn(),
    setMessageTag: vi.fn(), listMessages: vi.fn(), listMessagesPage: vi.fn(), syncFolder: vi.fn(),
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
// MESSAGE_PAGE_SIZE is a real runtime export of ../api (the flat view's page size), so the mock must
// supply it too or useFolderPagination's import throws. Its value is irrelevant here: the stubbed
// listMessagesPage ignores the limit. The search match markers are likewise real runtime exports the
// message list splits snippets on, so they carry their real control-character values.
vi.mock('../api', () => ({
    api: apiSpies,
    EventScope: {This: 0, Future: 1, All: 2},
    MESSAGE_PAGE_SIZE: 200,
    SEARCH_MATCH_START: '\u0001',
    SEARCH_MATCH_END: '\u0002',
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
        hasAttachments: false, answered: false, forwarded: false, snippet: 'A short snippet', tagColours: [],
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
    apiSpies.searchMessages.mockReset().mockResolvedValue({hits: [], degraded: false})
    apiSpies.setMessageTag.mockReset().mockResolvedValue(undefined)
    apiSpies.listMessages.mockReset().mockResolvedValue([])
    // The flat folder view now loads through listMessagesPage. Its default delegates to listMessages so a
    // test that stubs listMessages (the folder's rows) keeps working unchanged and the existing
    // "listMessages called with <folder>" assertions still hold; a page carries no more rows and no cursor.
    apiSpies.listMessagesPage.mockReset().mockImplementation(async (folderId: string) => ({
        messages: await apiSpies.listMessages(folderId),
        hasMore: false, nextCursorDateMs: 0, nextCursorId: '',
    }))
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
        expect(screen.getByText(/Add your mail account and you are in/)).toBeInTheDocument()
    })

    // The welcome empty state moves into WelcomeScreen.tsx (Phase 3.15). The render is pinned above; this pins
    // its one action, the Add account button, which opens account setup. It is scoped to the welcome card
    // because the titlebar also carries an Add account control.
    it('opens account setup from the welcome Add account button (WelcomeScreen)', async () => {
        const {container} = render(<App/>)
        await waitFor(
            () => expect(screen.getByText('Welcome to PigeonPost')).toBeInTheDocument(),
            {timeout: 3000},
        )
        const card = container.querySelector('.empty-card') as HTMLElement
        fireEvent.click(within(card).getByRole('button', {name: 'Add account'}))
        expect(await screen.findByRole('dialog', {name: 'Add account'})).toBeInTheDocument()
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

    it('toggling read from the reader flips it in place without reloading the folder (toggleRead)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({id: 'm1', subject: 'Weekly report', read: false})])
        // syncFolder rejects so the initial load is the only folder fetch; the read toggle must not add one.
        apiSpies.syncFolder.mockReset().mockRejectedValue(new Error('offline'))
        const {container} = render(<App/>)
        // Opening the message auto-marks it read, so the toolbar toggle then reads "Mark as unread".
        fireEvent.click(await screen.findByText('Weekly report'))
        await waitFor(() => expect(apiSpies.markRead).toHaveBeenCalledWith('m1', true))
        const folderFetches = apiSpies.listMessagesPage.mock.calls.length
        fireEvent.click(await screen.findByRole('button', {name: 'Mark as unread'}))
        // The flag flips back optimistically and the server is told, with no extra folder fetch.
        await waitFor(() => expect(apiSpies.markRead).toHaveBeenCalledWith('m1', false))
        await waitFor(() => expect(container.querySelector('[data-mid="m1"]')).toHaveClass('unread'))
        expect(apiSpies.listMessagesPage.mock.calls.length).toBe(folderFetches)
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

    it('shows the replied and forwarded glyphs on a message that has been answered or forwarded', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report', answered: true, forwarded: true})])
        const {container} = render(<App/>)
        await screen.findByText('Weekly report')
        const row = container.querySelector('[data-mid="m1"]') as HTMLElement
        expect(row.querySelector('.replied')).not.toBeNull()
        expect(row.querySelector('.forwarded')).not.toBeNull()
    })

    it('shows no replied or forwarded glyph on a plain message', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        const {container} = render(<App/>)
        await screen.findByText('Weekly report')
        const row = container.querySelector('[data-mid="m1"]') as HTMLElement
        expect(row.querySelector('.replied')).toBeNull()
        expect(row.querySelector('.forwarded')).toBeNull()
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

    // The multi-selection placeholder moves into SelectionSummary.tsx (Phase 3.16). The Mark unread path is
    // pinned above; this pins the count display and the Clear selection button, which returns to the reader.
    it('shows the selection count and clears it from the summary (SelectionSummary)', async () => {
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
        expect(await screen.findByText(/2 messages selected/)).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Clear selection'}))
        // Clearing drops the summary and returns to the single-message reader.
        await waitFor(() => expect(screen.queryByText(/2 messages selected/)).not.toBeInTheDocument())
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

// The colour-tagging that Phase 3.10 moves into useTags. The load of a message's tags is already covered by
// the reading-a-message test (it asserts api.messageTags); this pins the toggle path (toggleTag).
describe('App: tagging', () => {
    it('tags the selected message from the Mail menu colour submenu (useTags)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'Mail'}))
        // Tag with colour is a submenu; Enter opens it, then a colour applies that tag to the open message.
        fireEvent.keyDown(screen.getByRole('menuitem', {name: 'Tag with colour'}), {key: 'Enter'})
        // The colour items carry a checked state, so they render as menuitemcheckbox, not menuitem.
        fireEvent.click(screen.getByRole('menuitemcheckbox', {name: 'Red'}))
        await waitFor(() => expect(apiSpies.setMessageTag).toHaveBeenCalledWith('m1', expect.any(String), true))
    })
})

// The compose launchers that Phase 3.11 moves into useComposeLauncher: opening the composer to reply to the
// selected message (openReply) and the draft-recovery prompt offered on launch (the recovery effect with
// restoreDraft and discardDraft). Composing is observable as the ComposeModal ("New message" dialog) appearing.
describe('App: composing', () => {
    it('opens the composer to reply to the selected message (openReply)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        // The reader's Reply control opens the composer; scope to the reader since the titlebar duplicates it.
        const reader = container.querySelector('.reader') as HTMLElement
        fireEvent.click(within(reader).getByRole('button', {name: 'Reply'}))
        expect(await screen.findByRole('dialog', {name: 'New message'})).toBeInTheDocument()
    })

    it('offers to restore an autosaved draft on launch, then opens the composer (restoreDraft)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.draftRecovery.mockResolvedValue({
            present: true, accountId: 'acc1', to: 'bob@example.com', cc: '', bcc: '',
            subject: 'Half-written', bodyHtml: '<p>draft</p>', savedMs: 0,
        })
        render(<App/>)
        const dialog = await screen.findByRole('alertdialog', {name: 'Restore unsent message'})
        expect(dialog).toBeInTheDocument()
        fireEvent.click(within(dialog).getByRole('button', {name: 'Restore'}))
        expect(await screen.findByRole('dialog', {name: 'New message'})).toBeInTheDocument()
    })

    it('discards an autosaved draft, clearing it and dismissing the prompt (discardDraft)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.draftRecovery.mockResolvedValue({
            present: true, accountId: 'acc1', to: 'bob@example.com', cc: '', bcc: '',
            subject: 'Half-written', bodyHtml: '<p>draft</p>', savedMs: 0,
        })
        render(<App/>)
        const dialog = await screen.findByRole('alertdialog', {name: 'Restore unsent message'})
        fireEvent.click(within(dialog).getByRole('button', {name: 'Discard'}))
        await waitFor(() => expect(apiSpies.clearDraftRecovery).toHaveBeenCalled())
        expect(screen.queryByRole('alertdialog', {name: 'Restore unsent message'})).not.toBeInTheDocument()
    })

    // The recovery modal moves into DraftRecoveryDialog.tsx (Phase 3.17); the Restore and Discard paths are
    // pinned above. This pins its one bit of render logic, quoting the saved subject in the prompt.
    it('names the unsent message subject in the recovery prompt (DraftRecoveryDialog)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.draftRecovery.mockResolvedValue({
            present: true, accountId: 'acc1', to: 'bob@example.com', cc: '', bcc: '',
            subject: 'Half-written', bodyHtml: '<p>draft</p>', savedMs: 0,
        })
        render(<App/>)
        const dialog = await screen.findByRole('alertdialog', {name: 'Restore unsent message'})
        expect(within(dialog).getByText(/An unsent message "Half-written" was/)).toBeInTheDocument()
    })
})

// The backend-event wiring that Phase 3.12 moves into useAppEvents: the tray menu and app:close-request, the
// OS-handed .eml (eml:open), the mail:new poll refresh and calendar:changed. These fire from the backend, so
// the tests capture the EventsOn handlers as they register, then invoke them.
describe('App: backend events', () => {
    // captureEvents makes EventsOn record each handler by event name, so a test can drive a backend event.
    function captureEvents(): Record<string, (arg: unknown) => void> {
        const handlers: Record<string, (arg: unknown) => void> = {}
        runtimeSpies.EventsOn.mockImplementation((event: string, cb: (arg: unknown) => void) => {
            handlers[event] = cb
            return () => undefined
        })
        return handlers
    }

    it('shows an OS-handed .eml in the viewer on eml:open (useAppEvents)', async () => {
        const handlers = captureEvents()
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        render(<App/>)
        await waitFor(() => expect(handlers['eml:open']).toBeInstanceOf(Function))
        act(() => handlers['eml:open']({
            subject: 'Handed over', from: 'sender@example.com', to: 'me@example.com',
            date: '2026-07-11', html: '', plain: 'Body of the handed-over email',
        }))
        expect(await screen.findByRole('dialog', {name: 'Attached email'})).toBeInTheDocument()
        expect(screen.getByText('Handed over')).toBeInTheDocument()
    })

    it('offers minimise-or-quit on app:close-request (useAppEvents)', async () => {
        const handlers = captureEvents()
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        render(<App/>)
        await waitFor(() => expect(handlers['app:close-request']).toBeInstanceOf(Function))
        act(() => handlers['app:close-request'](undefined))
        expect(await screen.findByRole('alertdialog', {name: 'Close PigeonPost'})).toBeInTheDocument()
    })

    it('refreshes the unread counts and the open folder on mail:new (useAppEvents)', async () => {
        const handlers = captureEvents()
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        // Wait for the inbox to open so mail:new closes over selectedFolder, then clear the mount-time calls.
        expect(await screen.findByText('Weekly report')).toBeInTheDocument()
        apiSpies.listMessages.mockClear()
        apiSpies.unreadCounts.mockClear()
        act(() => handlers['mail:new'](undefined))
        await waitFor(() => expect(apiSpies.listMessages).toHaveBeenCalledWith('inbox'))
        expect(apiSpies.unreadCounts).toHaveBeenCalled()
    })
})

// The menu definitions and the accelerator effect that Phase 3.13 moves into useMenus. The menu-item onClick
// paths are already characterized (Move to, Mark as junk, Tag with colour, Reading pane); these pin the two
// pieces unique to this step: the Ctrl+N accelerator (menuShortcutsRef + matchesShortcut) and an uncovered
// File-menu item.
describe('App: menus', () => {
    it('opens the composer via the Ctrl+N menu accelerator (useMenus)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        render(<App/>)
        // Wait for the account to auto-select so the Compose accelerator is enabled (it needs a selected account).
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledWith('acc1'))
        fireEvent.keyDown(document.body, {key: 'n', ctrlKey: true})
        expect(await screen.findByRole('dialog', {name: 'New message'})).toBeInTheDocument()
    })

    it('saves the selected message as .eml from the File menu (useMenus)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        // Save as is gated on a selected message, so open one first.
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'File'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Save as...'}))
        await waitFor(() => expect(apiSpies.saveMessageAs).toHaveBeenCalledWith('m1', expect.any(String)))
    })
})

// The header that Phase 3.14 moves into TitleBar.tsx. The Sync button (already covered by the syncing test)
// and the menus stay wired through props; these pin two titlebar controls with no prior coverage: the theme
// toggle and the titlebar Compose icon button.
describe('App: titlebar', () => {
    it('toggles the theme from the titlebar (TitleBar)', () => {
        const {container} = render(<App/>)
        const toggle = container.querySelector('.theme-toggle') as HTMLElement
        const before = toggle.getAttribute('aria-label')
        fireEvent.click(toggle)
        // The toggle relabels itself to the opposite mode, so its accessible name flips.
        expect(toggle.getAttribute('aria-label')).not.toBe(before)
    })

    it('opens the composer from the titlebar Compose button (TitleBar)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        render(<App/>)
        // The Compose button is gated on a selected account, so wait for the auto-select.
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledWith('acc1'))
        fireEvent.click(screen.getByRole('button', {name: 'Compose'}))
        expect(await screen.findByRole('dialog', {name: 'New message'})).toBeInTheDocument()
    })
})

// The ~240-line window keydown effect that Phase 3.19 moves into useMessageListKeyboard. Its message-list
// actions are suppressed while any overlay is open, including the splash, so each test waits for the splash
// (a 2s timer) to clear before pressing a key. These pin two branches: Ctrl+A select-all and Delete.
describe('App: message-list keyboard', () => {
    it('selects every message with Ctrl+A, showing the count (useMessageListKeyboard)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([
            makeMessage({id: 'm1', subject: 'Weekly report'}),
            makeMessage({id: 'm2', subject: 'Second message'}),
        ])
        const {container} = render(<App/>)
        await screen.findByText('Weekly report')
        await waitFor(() => expect(container.querySelector('.splash')).toBeNull(), {timeout: 3000})
        fireEvent.keyDown(document.body, {key: 'a', ctrlKey: true})
        // Ctrl+A marks every message in the view, so the multi-selection summary shows the count.
        expect(await screen.findByText(/2 messages selected/)).toBeInTheDocument()
    })

    it('deletes the selected message with the Delete key (useMessageListKeyboard)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({id: 'm1', subject: 'Weekly report'})])
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        await waitFor(() => expect(container.querySelector('.splash')).toBeNull(), {timeout: 3000})
        fireEvent.keyDown(document.body, {key: 'Delete'})
        // Delete on the selected message asks for confirmation before removing it.
        expect(await screen.findByRole('alertdialog', {name: 'Delete message'})).toBeInTheDocument()
    })
})

// The flat folder view is keyset-paginated (useFolderPagination, wired through loadFolderMessages,
// loadMoreMessages and toggleSort). The list loads one page, appends the next as it nears the end, and
// reloads page one in the new direction when the sort is flipped, so a huge folder never loads every row at
// once. MESSAGE_PAGE_SIZE is 200 in the mock above.
describe('App: flat-view pagination', () => {
    it('reloads page one in the new direction when the date sort is toggled (toggleSort)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        // A single-page folder (no more pages), so no append fires and the reload is the only refetch.
        apiSpies.listMessagesPage.mockReset().mockResolvedValue({
            messages: [makeMessage({id: 'm1', subject: 'Weekly report'})],
            hasMore: false, nextCursorDateMs: 0, nextCursorId: '',
        })
        render(<App/>)
        await screen.findByText('Weekly report')
        // The default order is newest first (ascending false); toggling asks for oldest first.
        apiSpies.listMessagesPage.mockClear()
        fireEvent.click(screen.getByRole('button', {name: /Sort by date/}))
        // The reload starts a fresh page one (hasCursor false, cursor arguments ignored) in the new
        // ascending direction, rather than re-sorting the loaded prefix on the client.
        await waitFor(() =>
            expect(apiSpies.listMessagesPage).toHaveBeenCalledWith('inbox', false, 0, '', 200, true))
    })

    it('appends the next page as the list nears its end, dedupes and stops when no more remain (loadMoreMessages)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        // syncFolder rejects so loadFolderMessages leaves the cached first page in place (its post-sync
        // reload would reset pagination and discard the appended page mid-test); the append is then driven
        // purely by the list reaching its end.
        apiSpies.syncFolder.mockReset().mockRejectedValue(new Error('offline'))
        apiSpies.listMessagesPage.mockReset()
            .mockResolvedValueOnce({
                messages: [makeMessage({id: 'm1', subject: 'First'}), makeMessage({id: 'm2', subject: 'Second'})],
                hasMore: true, nextCursorDateMs: 111, nextCursorId: 'c1',
            })
            .mockResolvedValueOnce({
                // m2 is a duplicate of the first page; m3 is new. hasMore false ends paging.
                messages: [makeMessage({id: 'm2', subject: 'Second'}), makeMessage({id: 'm3', subject: 'Third'})],
                hasMore: false, nextCursorDateMs: 0, nextCursorId: '',
            })
            .mockResolvedValue({messages: [], hasMore: false, nextCursorDateMs: 0, nextCursorId: ''})
        render(<App/>)

        // The appended page's new row appears, alongside the first page's rows.
        await screen.findByText('Third')
        expect(screen.getByText('First')).toBeInTheDocument()
        // The duplicate m2 is appended only once.
        expect(screen.getAllByText('Second')).toHaveLength(1)
        // Exactly two fetches: the first page and the one appended page. Paging then stops (hasMore false),
        // so no third page is ever requested; the second fetch carried the first page's cursor.
        expect(apiSpies.listMessagesPage).toHaveBeenCalledTimes(2)
        expect(apiSpies.listMessagesPage).toHaveBeenNthCalledWith(1, 'inbox', false, 0, '', 200, false)
        expect(apiSpies.listMessagesPage).toHaveBeenNthCalledWith(2, 'inbox', true, 111, 'c1', 200, false)
    })
})

describe('App: search', () => {
    // Boots the app with one account, one folder and one cached message, ready for a search.
    async function renderWithInbox() {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        await waitFor(() => expect(screen.getByText('Weekly report')).toBeInTheDocument())
    }

    it('runs the query all-mail by default and renders the highlighted match snippet', async () => {
        apiSpies.searchMessages.mockResolvedValue({
            hits: [{
                message: makeMessage({id: 's1', subject: 'Search hit'}),
                snippet: 'about the ' + SEARCH_MATCH_START + 'penguin' + SEARCH_MATCH_END + ' colony',
            }],
            degraded: false,
        })
        await renderWithInbox()
        fireEvent.change(screen.getByLabelText('Search mail'), {target: {value: 'penguin'}})
        // The debounce is 250ms; the scope defaults to all mail (empty folder and account ids).
        await waitFor(() => expect(apiSpies.searchMessages).toHaveBeenCalledWith('penguin', '', ''))
        expect(await screen.findByText('Search hit')).toBeInTheDocument()
        const marked = await screen.findByText('penguin', {selector: 'mark'})
        expect(marked.tagName).toBe('MARK')
        // The markers themselves never render.
        expect(marked.closest('.message-snippet')?.textContent).toBe('about the penguin colony')
        expect(screen.queryByText('Searched as plain text')).not.toBeInTheDocument()
    })

    it('scopes the search to the selected folder via the scope selector', async () => {
        await renderWithInbox()
        fireEvent.change(screen.getByLabelText('Search scope'), {target: {value: 'folder'}})
        fireEvent.change(screen.getByLabelText('Search mail'), {target: {value: 'report'}})
        await waitFor(() => expect(apiSpies.searchMessages).toHaveBeenCalledWith('report', 'inbox', ''))
    })

    it('hints when the query degrades to plain text', async () => {
        apiSpies.searchMessages.mockResolvedValue({hits: [], degraded: true})
        await renderWithInbox()
        fireEvent.change(screen.getByLabelText('Search mail'), {target: {value: 'broken "quote'}})
        await waitFor(() => expect(screen.getByText('Searched as plain text')).toBeInTheDocument())
    })

    it('focuses the search box from Ctrl+K (Edit > Search)', async () => {
        await renderWithInbox()
        fireEvent.keyDown(document.body, {key: 'k', ctrlKey: true})
        expect(document.activeElement).toBe(screen.getByLabelText('Search mail'))
    })
})
