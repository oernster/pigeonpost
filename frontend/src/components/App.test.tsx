// Characterisation test for App at its stable outer interface. App is the root component and takes no
// props, so its interface is purely its observable behaviour: what it renders and which api calls fire in
// response to mount and the core user gestures. This suite pins that behaviour BEFORE Phase 3 decomposes App
// into hooks and sub-components (useMessageStore, useSelection, useFolders, useAccounts, TitleBar, AppModals
// and the rest). None of those extractions change what App does on screen, so this suite staying green is the
// proof each one preserved behaviour, exactly as the modal characterisation tests were the proof in Phase 2.
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
import {IDLE_REFOCUS_MS} from '../hooks/useIdleRefocus'
import {printFrameId, printReadyMarkerId} from '../print'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    version: vi.fn(), author: vi.fn(),
    listAccounts: vi.fn(),
    draftRecovery: vi.fn(), clearDraftRecovery: vi.fn(),
    listRules: vi.fn(), listTemplates: vi.fn(), listContacts: vi.fn(), listEvents: vi.fn(),
    send: vi.fn(), markReplied: vi.fn(), markForwarded: vi.fn(), collectContacts: vi.fn(),
    snoozedCount: vi.fn(), outboxCount: vi.fn(),
    listContactGroups: vi.fn(), listCalendars: vi.fn(), listEventInstances: vi.fn(),
    listCalDAVAccounts: vi.fn(),
    unreadCounts: vi.fn(), listTags: vi.fn(), saveTag: vi.fn(),
    messageTags: vi.fn(), messageBody: vi.fn(), loadRemoteImages: vi.fn(), searchMessages: vi.fn(),
    setMessageTag: vi.fn(), listMessages: vi.fn(), listMessagesPage: vi.fn(), syncFolder: vi.fn(),
    listFolders: vi.fn(), listOutbox: vi.fn(), cancelOutboxItem: vi.fn(),
    conversation: vi.fn(),
    syncAccount: vi.fn(), replayOutbox: vi.fn(), removeAccount: vi.fn(),
    deleteMessage: vi.fn(), deleteMessagePermanent: vi.fn(), saveMessageAs: vi.fn(),
    markFlagged: vi.fn(), moveMessage: vi.fn(), markJunk: vi.fn(), markNotJunk: vi.fn(), copyMessage: vi.fn(),
    createFolder: vi.fn(), renameFolder: vi.fn(), deleteFolder: vi.fn(), moveFolder: vi.fn(),
    folderUIState: vi.fn(), saveFolderUIState: vi.fn(),
    pickAttachments: vi.fn(), about: vi.fn(), licence: vi.fn(), openReleases: vi.fn(),
    checkForUpdates: vi.fn(async () => ({current: '', latest: '', updateAvailable: false, downloadUrl: '', pageUrl: ''})),
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
// The api mock is built from the real module rather than hand-listed beside it, so a method the app
// calls that no spy declares fails the test by name instead of throwing a TypeError into the nearest
// catch and passing. See src/test/apiMock.ts for why that mattered and what it found. Spreading the
// real module also carries its genuine value exports (the page size, the search-match markers, the
// EventScope enum), so those cannot drift from the real ones either.
const unstubbedCalls = vi.hoisted(() => new Set<string>())

vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})
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

// pickAccount drives the sidebar's account picker the way a user does: open the listbox, then press the
// option for the account wanted. The picker is a listbox rather than a native select precisely so that a
// pick always reports, including a pick of the account already showing.
function pickAccount(accountId: string) {
    fireEvent.click(screen.getByLabelText('Active account'))
    fireEvent.mouseDown(document.getElementById(`account-option-${accountId}`)!)
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
    apiSpies.draftRecovery.mockReset().mockResolvedValue({
        present: false, accountId: '', to: '', cc: '', bcc: '', subject: '', bodyHtml: '', savedMs: 0,
    })
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.listRules.mockReset().mockResolvedValue([])
    apiSpies.listTemplates.mockReset().mockResolvedValue([])
    apiSpies.send.mockReset().mockResolvedValue('')
    apiSpies.collectContacts.mockReset().mockResolvedValue(undefined)
    apiSpies.snoozedCount.mockReset().mockResolvedValue(0)
    apiSpies.listContactGroups.mockReset().mockResolvedValue([])
    apiSpies.listCalDAVAccounts.mockReset().mockResolvedValue([])
    apiSpies.listCalendars.mockReset().mockResolvedValue([])
    apiSpies.listEventInstances.mockReset().mockResolvedValue([])
    apiSpies.outboxCount.mockReset().mockResolvedValue(0)
    apiSpies.markReplied.mockReset().mockResolvedValue(undefined)
    apiSpies.markForwarded.mockReset().mockResolvedValue(undefined)
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
    apiSpies.folderUIState.mockReset().mockResolvedValue({order: [], collapsed: []})
    apiSpies.saveFolderUIState.mockReset().mockResolvedValue(undefined)
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
    apiSpies.markNotJunk.mockReset().mockResolvedValue(undefined)
    apiSpies.copyMessage.mockReset().mockResolvedValue(undefined)
    apiSpies.conversation.mockReset().mockResolvedValue([])
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

afterEach(() => {
    cleanup()
    // The print frame is appended to document.body, which cleanup() does not own.
    document.getElementById(printFrameId)?.remove()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

// The mock covers the api in both directions. The afterEach above catches a method the app reaches
// that no spy declares; this catches the opposite, a spy declared under a name the api does not have,
// which binds to nothing: every test configuring it would then be configuring a stub the code can
// never call, so it would pass for the wrong reason.
// About and Licence are each a text loaded on demand from the Help menu, held while its dialog is open
// and dropped when it closes. Neither had any coverage.
// The four manager dialogs (contacts, calendar, filter rules and message templates) are the surfaces
// over the four managed collections. Opening and closing them had no coverage, nor did the calendar's
// one extra state, the event a clicked reminder lands it on.
// The destructive confirmations App itself owns. Four were already pinned by the tests above (delete a
// message, delete a selection, delete a folder, remove an account); these are the three that were not.
describe('App: the remaining confirmations', () => {
    async function renderWithMessage() {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
    }

    it('confirms before permanently deleting one message', async () => {
        await renderWithMessage()
        fireEvent.click(screen.getByRole('button', {name: 'Edit'}))
        fireEvent.click(await screen.findByRole('menuitem', {name: 'Delete permanently'}))
        const dialog = await screen.findByRole('alertdialog', {name: 'Delete permanently'})
        expect(within(dialog).getByText(/cannot be recovered/)).toBeInTheDocument()
        fireEvent.click(within(dialog).getByRole('button', {name: 'Delete permanently'}))
        await waitFor(() => expect(apiSpies.deleteMessagePermanent).toHaveBeenCalledWith('m1'))
    })

    it('cancels out of the permanent delete without touching the message', async () => {
        await renderWithMessage()
        fireEvent.click(screen.getByRole('button', {name: 'Edit'}))
        fireEvent.click(await screen.findByRole('menuitem', {name: 'Delete permanently'}))
        const dialog = await screen.findByRole('alertdialog', {name: 'Delete permanently'})
        fireEvent.click(within(dialog).getByRole('button', {name: 'Cancel'}))
        await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
        expect(apiSpies.deleteMessagePermanent).not.toHaveBeenCalled()
    })

    // Two of App's seven confirmations stay unpinned here: the bulk permanent delete, whose Shift+Delete
    // gesture needs list focus this harness does not give it, plus Cancel send, which needs a queued
    // outbox message. Both share their handler with a sibling that is pinned (runBulkDelete with the
    // bulk delete above, cancelSend with the outbox tests), so neither is unexercised code.
})

describe('App: the manager dialogs', () => {
    it('opens contacts from the title bar and closes it', async () => {
        render(<App/>)
        fireEvent.click(screen.getByRole('button', {name: 'Contacts'}))
        const dialog = await screen.findByRole('dialog', {name: 'Contacts'})
        fireEvent.click(within(dialog).getAllByRole('button', {name: 'Close'}).slice(-1)[0])
        await waitFor(() => expect(screen.queryByRole('dialog', {name: 'Contacts'})).toBeNull())
    })

    it('opens the calendar from the title bar and closes it', async () => {
        render(<App/>)
        fireEvent.click(screen.getByRole('button', {name: 'Calendar'}))
        const dialog = await screen.findByRole('dialog', {name: 'Calendar'})
        fireEvent.click(within(dialog).getAllByRole('button', {name: 'Close'}).slice(-1)[0])
        await waitFor(() => expect(screen.queryByRole('dialog', {name: 'Calendar'})).toBeNull())
    })

    it('opens the filter rules from the Edit menu', async () => {
        render(<App/>)
        fireEvent.click(screen.getByRole('button', {name: 'Edit'}))
        fireEvent.click(await screen.findByRole('menuitem', {name: 'Rules'}))
        expect(await screen.findByRole('dialog', {name: 'Filter rules'})).toBeInTheDocument()
    })

    it('opens the message templates from the Edit menu', async () => {
        render(<App/>)
        fireEvent.click(screen.getByRole('button', {name: 'Edit'}))
        fireEvent.click(await screen.findByRole('menuitem', {name: 'Templates'}))
        expect(await screen.findByRole('dialog', {name: 'Message templates'})).toBeInTheDocument()
    })

    // A clicked reminder opens the calendar on the event it is about. What is pinned here is App's half,
    // the wiring from the reminder toast to the calendar being open. The other half, the calendar landing
    // on that event and forgetting it on close, is CalendarModal's own and is not pinned at this level:
    // reaching it needs a fully-formed event and instance whose shapes are the calendar's business.
    it('opens the calendar on a clicked reminder', async () => {
        const handlers: Record<string, (arg: unknown) => void> = {}
        runtimeSpies.EventsOn.mockImplementation((event: string, cb: (arg: unknown) => void) => {
            handlers[event] = cb
            return () => undefined
        })
        render(<App/>)
        await waitFor(() => expect(handlers['calendar:reminder']).toBeInstanceOf(Function))
        const loadsBefore = apiSpies.listEvents.mock.calls.length
        act(() => handlers['calendar:reminder']({
            eventId: 'e1', summary: 'Standup', start: '2026-08-29T09:00:00.000Z',
        }))
        fireEvent.click(await screen.findByText('Standup'))
        expect(await screen.findByRole('dialog', {name: 'Calendar'})).toBeInTheDocument()
        // The events are refreshed first, so the calendar can find the one the reminder is about.
        await waitFor(() => expect(apiSpies.listEvents.mock.calls.length).toBeGreaterThan(loadsBefore))
    })
})

describe('App: about and licence', () => {
    const about = {
        name: 'PigeonPost', tagline: 'Calm mail', version: '9.9.9', author: 'Oliver Ernster',
        copyright: '', licence: 'GPL-3.0', attribution: '', credits: [],
    }

    async function openHelp(item: string) {
        render(<App/>)
        fireEvent.click(screen.getByRole('button', {name: 'Help'}))
        fireEvent.click(await screen.findByRole('menuitem', {name: item}))
    }

    it('opens About from the Help menu and drops it on close', async () => {
        apiSpies.about.mockResolvedValue(about)
        await openHelp('About PigeonPost')
        const dialog = await screen.findByRole('dialog', {name: 'About PigeonPost'})
        expect(within(dialog).getByText('Calm mail')).toBeInTheDocument()
        // The dialog carries both a corner cross and a footer Close, so pick the footer one.
        fireEvent.click(within(dialog).getAllByRole('button', {name: 'Close'})[1])
        await waitFor(() => expect(screen.queryByRole('dialog', {name: 'About PigeonPost'})).toBeNull())
    })

    it('opens the licence text from the Help menu', async () => {
        apiSpies.licence.mockResolvedValue('GNU GENERAL PUBLIC LICENSE')
        await openHelp('Licence')
        const dialog = await screen.findByRole('dialog', {name: 'Licence'})
        expect(within(dialog).getByText(/GNU GENERAL PUBLIC LICENSE/)).toBeInTheDocument()
    })

    it('reports a failed About read through the error bar', async () => {
        apiSpies.about.mockRejectedValue('about unavailable')
        await openHelp('About PigeonPost')
        expect(await screen.findByText(/about unavailable/)).toBeInTheDocument()
        expect(screen.queryByRole('dialog', {name: 'About PigeonPost'})).toBeNull()
    })

    it('reports a failed licence read through the error bar', async () => {
        apiSpies.licence.mockRejectedValue('licence unavailable')
        await openHelp('Licence')
        expect(await screen.findByText(/licence unavailable/)).toBeInTheDocument()
    })
})

describe('App: the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })

    it('covers enough of the api to be worth having', async () => {
        // Guards against a rename or a moved import turning the checks above into vacuous passes.
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(Object.keys(actual.api).length).toBeGreaterThan(50)
    })
})

describe('App: mount and splash', () => {
    it('renders the titlebar and shows the splash on launch', () => {
        const {container} = render(<App/>)
        expect(container.querySelector('.splash')).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Mail'})).toBeInTheDocument()
        expect(apiSpies.listAccounts).toHaveBeenCalled()
    })

    it('names the version and author on the splash once they load', async () => {
        apiSpies.version.mockResolvedValue('9.9.9')
        apiSpies.author.mockResolvedValue('Oliver Ernster')
        const {container} = render(<App/>)
        expect(await screen.findByText('v9.9.9')).toBeInTheDocument()
        expect(screen.getByText('by Oliver Ernster')).toBeInTheDocument()
        expect(container.querySelector('.splash')).toBeInTheDocument()
    })

    it('still shows the splash when the version and author cannot be read', async () => {
        apiSpies.version.mockRejectedValue('no version')
        apiSpies.author.mockRejectedValue('no author')
        const {container} = render(<App/>)
        await waitFor(() => expect(apiSpies.author).toHaveBeenCalled())
        // Neither failure is worth an error bar on launch; the splash simply carries no line for them.
        expect(container.querySelector('.splash')).toBeInTheDocument()
        expect(container.querySelector('.splash-version')).toBeNull()
        expect(container.querySelector('.splash-author')).toBeNull()
        expect(container.querySelector('.error-bar')).toBeNull()
    })

    it('fades the splash before it goes', async () => {
        const {container} = render(<App/>)
        expect(container.querySelector('.splash.fading')).toBeNull()
        await waitFor(() => expect(container.querySelector('.splash.fading')).not.toBeNull(), {timeout: 3000})
        await waitFor(() => expect(container.querySelector('.splash')).toBeNull(), {timeout: 3000})
    })

    it('groups the title bar into the mark, the working set and the app pair', () => {
        // Which group a control sits in is a layout decision rather than a cosmetic one: the mark and
        // the controls read left to right from the corner while the app pair is held at the far end. A
        // control added to the wrong group lands on the wrong side of the bar.
        const {container} = render(<App/>)
        const inCentre = container.querySelectorAll('.titlebar-actions [aria-label], .titlebar-actions button')
        const centreLabels = [...inCentre].map((el) => el.getAttribute('aria-label') ?? el.textContent?.trim())
        expect(centreLabels).toContain('Compose')
        expect(centreLabels).toContain('Add account')
        expect(centreLabels).toContain('Sync')
        expect(centreLabels.some((l) => l?.includes('Contacts'))).toBe(true)
        expect(centreLabels.some((l) => l?.includes('Calendar'))).toBe(true)
        // The menus travel with the controls. Moving the controls alone was tried once and split one
        // sequence into two with a gap between them, so the two belonging to the same group is the point
        // of the arrangement rather than an incidental detail of it.
        for (const menu of ['File', 'Edit', 'View', 'Mail']) {
            expect(centreLabels).toContain(menu)
        }
        // The left group holds the mark alone. A menu appearing here is the old arrangement returning.
        const left = container.querySelector('.titlebar-left')
        expect(left?.querySelectorAll('.menu-title')).toHaveLength(0)
        const right = container.querySelector('.titlebar-right')
        expect(right?.querySelectorAll('button')).toHaveLength(2)
        // The app mark stands where the wordmark did. It is a picture and not a control, so the two
        // properties worth pinning are that it is not a button and that nothing can focus it: a span
        // with no tabindex takes no click and no place in the tab order.
        const mark = container.querySelector('.titlebar-left .titlebar-mark')
        expect(mark).toBeInTheDocument()
        expect(mark!.tagName).toBe('SPAN')
        expect(mark!.hasAttribute('tabindex')).toBe(false)
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

    // Picking the account already showing takes you back to its inbox: it reloads the folder list and
    // reopens the Inbox, so a picker used to check where mail landed lands you in it. A native select was
    // silent on a re-pick, which is why the picker is a listbox of the app's own.
    it('reopens the inbox when the account already showing is picked again', async () => {
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
        // Move off the inbox, so the re-pick has somewhere to bring the view back from.
        fireEvent.click(container.querySelector('[data-folder-id="archive"]')!)
        expect(await screen.findByText('Archived item')).toBeInTheDocument()
        pickAccount('acc1')
        expect(await screen.findByText('Weekly report')).toBeInTheDocument()
        const inbox = container.querySelector('[data-folder-id="inbox"]')!
        expect(inbox.className).toContain('selected')
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

    // After the idle interval with no user activity, the app returns to its resting view: the active
    // account's Inbox becomes the selected folder again. The timer arms on activity, so the test
    // switches to fake timers, taps a key and advances past the interval.
    it('reselects the inbox after the idle interval', async () => {
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
        expect(await screen.findByText('Archived item')).toBeInTheDocument()
        vi.useFakeTimers()
        try {
            act(() => {
                window.dispatchEvent(new Event('keydown'))
                vi.advanceTimersByTime(IDLE_REFOCUS_MS)
            })
        } finally {
            vi.useRealTimers()
        }
        expect(await screen.findByText('Weekly report')).toBeInTheDocument()
        const inbox = container.querySelector('[data-folder-id="inbox"]')!
        expect(inbox.className).toContain('selected')
    })

    // A message open in the reader keeps the view alive: the idle return would close it mid-read, so
    // it is skipped entirely while a message is selected.
    it('stays on the open message instead of returning to the inbox when idle', async () => {
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
        fireEvent.click(await screen.findByText('Archived item'))
        await waitFor(() => expect(apiSpies.messageBody).toHaveBeenCalledWith('a1'))
        vi.useFakeTimers()
        try {
            act(() => {
                window.dispatchEvent(new Event('keydown'))
                vi.advanceTimersByTime(IDLE_REFOCUS_MS)
            })
        } finally {
            vi.useRealTimers()
        }
        const archive = container.querySelector('[data-folder-id="archive"]')!
        expect(archive.className).toContain('selected')
        const reader = container.querySelector('.reader') as HTMLElement
        expect(within(reader).getByRole('button', {name: 'Reply'})).toBeInTheDocument()
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

    // Double-clicking a row pops the email out into its own dialog over the app (the
    // Thunderbird-style open); closing it returns to the list with the layout untouched.
    it('pops a message out into its own dialog on double-click', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        const {container} = render(<App/>)
        const row = await screen.findByText('Weekly report')
        fireEvent.doubleClick(row)
        const dialog = await screen.findByRole('dialog', {name: 'Weekly report'})
        // The dialog hosts the ordinary reader, so its actions are all present.
        expect(within(dialog).getByRole('button', {name: 'Reply'})).toBeInTheDocument()
        // The list-plus-pane layout is untouched behind it.
        expect(container.querySelector('.pane.message-list')).toBeInTheDocument()
        // The dialog's close cross takes focus so one key shuts it.
        expect(document.activeElement).toBe(within(dialog).getByRole('button', {name: 'Close'}))
        fireEvent.mouseDown(within(dialog).getByRole('button', {name: 'Close'}))
        await waitFor(() => expect(screen.queryByRole('dialog', {name: 'Weekly report'})).not.toBeInTheDocument())
    })
})

// The panes region decides which of the three panes are on screen and what the splitters span. It is one
// block of App's JSX with a three-way choice in it: both panes with the reading pane on, the reader alone
// when a message is open full-width with the pane off, otherwise the list alone. Nothing pinned that choice
// directly, so these tests state it before the block moves.
describe('App: the panes layout', () => {
    // Standing arrangement for every case here: one account, one folder, one message.
    const arrange = () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
    }

    // turnReadingPaneOff drives the real gesture rather than reaching into state, so the test exercises the
    // same path a user does.
    const turnReadingPaneOff = async (container: HTMLElement) => {
        fireEvent.click(screen.getByRole('button', {name: 'View'}))
        fireEvent.click(screen.getByRole('menuitemcheckbox', {name: 'Reading pane'}))
        await waitFor(() => expect(container.querySelector('.panes.no-preview')).toBeInTheDocument())
    }

    it('shows both panes and the list splitter with the reading pane on', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        await waitFor(() => expect(container.querySelector('.pane.reader')).toBeInTheDocument())
        expect(container.querySelector('.pane.message-list')).toBeInTheDocument()
        // Two panes means a list|reader boundary, so that splitter is drawn alongside the sidebar one.
        expect(screen.getByRole('separator', {name: 'Resize message list'})).toBeInTheDocument()
        expect(screen.getByRole('separator', {name: 'Resize sidebar'})).toBeInTheDocument()
    })

    it('shows the list alone and drops the list splitter with the reading pane off', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        await turnReadingPaneOff(container)
        expect(container.querySelector('.pane.message-list')).toBeInTheDocument()
        expect(container.querySelector('.pane.reader')).not.toBeInTheDocument()
        // No list|reader boundary with one pane, so only the sidebar handle remains.
        expect(screen.queryByRole('separator', {name: 'Resize message list'})).not.toBeInTheDocument()
        expect(screen.getByRole('separator', {name: 'Resize sidebar'})).toBeInTheDocument()
    })

    it('shows the reader alone when a message is opened with the reading pane off', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        await turnReadingPaneOff(container)
        // Open in new tab is the gesture that opens a message full-width while the pane is off.
        fireEvent.click(screen.getByRole('button', {name: 'Mail'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Open in new tab'}))
        await waitFor(() => expect(container.querySelector('.pane.reader')).toBeInTheDocument())
        // The reader takes the whole width, so the list is not rendered beside it.
        expect(container.querySelector('.pane.message-list')).not.toBeInTheDocument()
    })
})

// The two right-click menus are App's last conditional JSX blocks. Both components have their own tests
// covering what they render; what nothing covered is App's wiring of them, which is what these pin: the
// gesture that opens each menu, one representative entry reaching the right handler and the dismissal
// that puts the menu away.
describe('App: the right-click menus', () => {
    const arrange = () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([
            makeFolder('inbox', 'Inbox', 'inbox'),
            makeFolder('f2', 'Projects', 'custom'),
        ])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
    }

    // The message menu nests submenus, each its own role=menu, so the outer menu is found by its class
    // rather than by role. Both menus render as .context-menu, so one helper serves them both.
    const findContextMenu = async (container: HTMLElement): Promise<HTMLElement> => {
        await waitFor(() => expect(container.querySelector('.context-menu')).toBeInTheDocument())
        return container.querySelector('.context-menu') as HTMLElement
    }

    it('opens the message menu on a right-click and routes Save as to the api', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.contextMenu(await screen.findByText('Weekly report'))
        const menu = await findContextMenu(container)
        fireEvent.click(within(menu).getByRole('menuitem', {name: /Save as/}))
        await waitFor(() => expect(apiSpies.saveMessageAs).toHaveBeenCalledWith('m1', 'Weekly report.eml'))
    })

    it('routes the message menu Delete to the delete confirmation rather than deleting', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.contextMenu(await screen.findByText('Weekly report'))
        const menu = await findContextMenu(container)
        fireEvent.click(within(menu).getByRole('menuitem', {name: 'Delete'}))
        // onDelete is wired to requestDelete, so the menu asks rather than acting.
        expect(await screen.findByRole('alertdialog', {name: 'Delete message'})).toBeInTheDocument()
        expect(apiSpies.deleteMessage).not.toHaveBeenCalled()
    })

    it('dismisses the message menu on Escape', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.contextMenu(await screen.findByText('Weekly report'))
        await findContextMenu(container)
        fireEvent.keyDown(document, {key: 'Escape'})
        await waitFor(() => expect(container.querySelector('.context-menu')).not.toBeInTheDocument())
    })

    it('opens the folder menu on a right-click and routes Rename to the folder prompt', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.contextMenu(await screen.findByText('Projects'))
        const menu = await findContextMenu(container)
        fireEvent.click(within(menu).getByRole('menuitem', {name: /Rename folder/}))
        // onRenameFolder opens the same prompt the row's pencil does, seeded with the folder's name.
        const prompt = await screen.findByRole('dialog')
        expect(within(prompt).getByDisplayValue('Projects')).toBeInTheDocument()
    })

    it('dismisses the folder menu on Escape', async () => {
        arrange()
        const {container} = render(<App/>)
        fireEvent.contextMenu(await screen.findByText('Projects'))
        await findContextMenu(container)
        fireEvent.keyDown(document, {key: 'Escape'})
        await waitFor(() => expect(container.querySelector('.context-menu')).not.toBeInTheDocument())
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

    it('leaves a Shift-click mousedown default intact so the range can still be dragged', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([
            makeMessage({id: 'm1', subject: 'First', date: '2026-07-11T10:03:00.000Z'}),
            makeMessage({id: 'm2', subject: 'Second', date: '2026-07-11T10:02:00.000Z'}),
        ])
        const {container} = render(<App/>)
        fireEvent.click(await screen.findByText('First'))
        const row = container.querySelector('[data-mid="m2"]')!
        // The browser starts a native drag from the mousedown that begins the gesture. Cancelling that
        // default cancels the drag with it, so a Shift-click must leave it alone (the text smear it would
        // otherwise cause is handled by user-select: none in CSS).
        const down = new MouseEvent('mousedown', {bubbles: true, cancelable: true, shiftKey: true})
        fireEvent(row, down)
        expect(down.defaultPrevented).toBe(false)
        // The range gesture itself still works; the dragged row carries its id to the folder tree.
        fireEvent.click(screen.getByText('Second'), {shiftKey: true})
        expect(await screen.findByText('2 messages selected')).toBeInTheDocument()
        expect(row).toHaveAttribute('draggable', 'true')
    })

    it('keeps the Shift-drag escape hatch on the rows in the stylesheet', async () => {
        // Blink refuses to start a native drag from a Shift-held mousedown unless the pressed
        // node's computed -webkit-user-drag is element; the draggable attribute is not consulted
        // and the check runs on the deepest hit-tested node, where the property does not inherit.
        // Without this rule a Shift-selected range can only be dragged after releasing Shift.
        // jsdom applies no stylesheets and Vitest serves CSS imports as empty modules (raw query
        // included), so the rule is pinned by reading the stylesheet from disk. The pieced-together
        // specifier keeps the untyped node built-in out of tsc's module resolution.
        const fsModule = 'node:' + 'fs'
        // Vitest rewrites import.meta.url to a non-file scheme, so the path is anchored to the
        // runner's working directory (the frontend root) instead.
        const {readFileSync} = (await import(fsModule)) as {readFileSync: (path: string, encoding: string) => string}
        const css = readFileSync('src/styles/list-rows.css', 'utf8')
        expect(css).toMatch(/\.message-row,\s*\.message-row \*\s*\{\s*-webkit-user-drag: element;\s*\}/)
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

    // A message sitting in the Junk folder offers the rescue instead: Not junk moves it back to the
    // inbox on the server and drops it from the Junk view at once.
    it('rescues a junked message from the Mail menu (markNotJunk)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([
            makeFolder('inbox', 'Inbox', 'inbox'),
            makeFolder('junk', 'Junk', 'junk'),
        ])
        apiSpies.listMessages.mockImplementation(async (folderId: string) =>
            folderId === 'junk' ? [makeMessage({id: 'm1', subject: 'Not actually spam', folderId: 'junk'})] : [])
        render(<App/>)
        fireEvent.click(await screen.findByText('Junk'))
        fireEvent.click(await screen.findByText('Not actually spam'))
        fireEvent.click(screen.getByRole('button', {name: 'Mail'}))
        // The junk action reads Not junk here; the re-junk item is not offered.
        expect(screen.queryByRole('menuitem', {name: 'Mark as junk'})).not.toBeInTheDocument()
        fireEvent.click(screen.getByRole('menuitem', {name: 'Not junk'}))
        await waitFor(() => expect(apiSpies.markNotJunk).toHaveBeenCalledWith('m1'))
        await waitFor(() => expect(screen.queryByText('Not actually spam')).not.toBeInTheDocument())
        // The inbox is synced at once so the rescued message (and its unread badge) appears
        // immediately rather than on the next background sync.
        await waitFor(() => expect(apiSpies.syncFolder).toHaveBeenCalledWith('inbox'))
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

// The outbox that Phase 3.6 moves into useOutbox: the queue is loaded on mount; while the selected
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

// The account list and the load/remove operations that Phase 3.8 moves into useAccounts. The account
// picker carries a Remove button; confirming it calls the remove api.
describe('App: account management', () => {
    it('removes an account through the confirm dialog (useAccounts)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        render(<App/>)
        // The picker carries a Remove <email> button for the account showing; it asks first.
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

    // A sync talks to the mail server, so it easily outlives an account switch. Its folder list is claimed
    // by the account it was fetched for: landing it under the account since selected left the sidebar showing
    // the previous account's folders, so no row matched the newly opened Inbox and neither its highlight nor
    // its unread badges appeared.
    it('discards a folder list that lands after the account has been switched (useSync)', async () => {
        apiSpies.listAccounts.mockResolvedValue([
            makeAccount(),
            makeAccount({id: 'acc2', email: 'other@example.com', displayName: 'Other'}),
        ])
        apiSpies.listFolders.mockImplementation((accountId: string) =>
            Promise.resolve(accountId === 'acc2'
                ? [{...makeFolder('inbox2', 'Inbox', 'inbox', {unread: 2}), accountId: 'acc2'}]
                : [makeFolder('inbox', 'Inbox', 'inbox')]))
        apiSpies.listMessages.mockImplementation((id: string) =>
            Promise.resolve(id === 'inbox2'
                ? [makeMessage({id: 'm2', folderId: 'inbox2', subject: 'New arrival'})]
                : [makeMessage({subject: 'Weekly report'})]))
        // The first account's sync is held open, so it finishes only after the switch to the second.
        let finishSync = () => undefined as void
        apiSpies.syncAccount.mockImplementation(() => new Promise<void>((resolve) => {
            finishSync = () => resolve()
        }))
        const {container} = render(<App/>)
        expect(await screen.findByText('Weekly report')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Sync'}))
        await waitFor(() => expect(apiSpies.syncAccount).toHaveBeenCalledWith('acc1'))
        pickAccount('acc2')
        expect(await screen.findByText('New arrival')).toBeInTheDocument()
        await act(async () => {
            finishSync()
        })
        const row = await waitFor(() => {
            const found = container.querySelector('[data-folder-id="inbox2"]')
            expect(found).not.toBeNull()
            return found as HTMLElement
        })
        expect(container.querySelector('[data-folder-id="inbox"]')).toBeNull()
        expect(row.className).toContain('selected')
        expect(within(row).getByText('2')).toBeInTheDocument()
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

    it('opens a pre-filled composer on mailto:open (useComposeLauncher)', async () => {
        const handlers = captureEvents()
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        render(<App/>)
        await waitFor(() => expect(handlers['mailto:open']).toBeInstanceOf(Function))
        act(() => handlers['mailto:open']({
            to: ['jane@example.org'], cc: null, bcc: null,
            subject: 'Chess move', body: 'Knight takes.',
        }))
        const dialog = await screen.findByRole('dialog', {name: 'New message'})
        expect(within(dialog).getByDisplayValue('jane@example.org')).toBeInTheDocument()
        expect(within(dialog).getByDisplayValue('Chess move')).toBeInTheDocument()
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

    // The per-folder unread badge rides on the folder list, not on the unread counts, so an arrival that
    // refreshed only the counts badged the account and the titlebar while leaving the Inbox row bare until
    // the next sync or account switch. mail:new refetches the folder list for that badge.
    it('badges the folder an arrival landed in on mail:new (useAppEvents)', async () => {
        const handlers = captureEvents()
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        const {container} = render(<App/>)
        expect(await screen.findByText('Weekly report')).toBeInTheDocument()
        const row = () => container.querySelector('[data-folder-id="inbox"]') as HTMLElement
        expect(within(row()).queryByText('2')).toBeNull()
        // The arrival is already in the local cache by the time the poller announces it, so the refetched
        // folder list is what carries the new count.
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox', {unread: 2})])
        act(() => handlers['mail:new'](undefined))
        await waitFor(() => expect(within(row()).getByText('2')).toBeInTheDocument())
    })
})

// The menu definitions and the accelerator effect that Phase 3.13 moves into useMenus. The menu-item onClick
// paths are already characterised (Move to, Mark as junk, Tag with colour, Reading pane); these pin the two
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

    it('keeps Compose out of the Mail menu while its accelerator still fires', async () => {
        // Composing has a button of its own in the title bar, so the menu entry was a duplicate. The
        // item stays in the definitions, hidden, because the accelerators are wired from them: the
        // Ctrl+N test above is what proves the key survived its removal from the dropdown.
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        render(<App/>)
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledWith('acc1'))
        fireEvent.click(screen.getByRole('button', {name: 'Mail'}))
        expect(await screen.findByRole('menuitem', {name: /Add account/})).toBeInTheDocument()
        expect(screen.queryByRole('menuitem', {name: 'Compose'})).toBeNull()
    })

    it('opens the reply composer via the Ctrl+R submenu accelerator (useMenus)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        // Reply is gated on a selected message. Its accelerator lives on a Respond submenu item,
        // so this also pins the flattening of submenu children into the global shortcut scan.
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.keyDown(document.body, {key: 'r', ctrlKey: true})
        expect(await screen.findByRole('dialog', {name: 'New message'})).toBeInTheDocument()
    })

    // Printing had no coverage above the pure print.ts document builder: the frame lifecycle, the
    // load guard and the remote-image restore all lived unpinned in App. These characterise them at
    // App's outer interface, so the behaviour is held whichever module ends up owning it.
    it('prints the selected message into an off-screen frame (print)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        apiSpies.messageBody.mockResolvedValue({
            plain: '', html: '<p><img data-pp-src="https://example.com/a.png"/>Body</p>',
            hasInvite: false, attachments: [],
        })
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'File'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Print...'}))
        await waitFor(() => expect(document.getElementById(printFrameId)).not.toBeNull())
        const frame = document.getElementById(printFrameId) as HTMLIFrameElement
        expect(frame.getAttribute('aria-hidden')).toBe('true')
        // The parked remote images are restored for the printed copy; the document is written into the
        // frame rather than set through srcdoc.
        await waitFor(() => expect(frame.contentDocument?.documentElement.innerHTML ?? '').toContain('Weekly report'))
        const written = frame.contentDocument?.documentElement.innerHTML ?? ''
        expect(written).toContain('src="https://example.com/a.png"')
        expect(written).not.toContain('data-pp-src=')
        expect(frame.getAttribute('srcdoc')).toBeNull()
    })

    it('does not print a frame that has not loaded the print document (print)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'File'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Print...'}))
        await waitFor(() => expect(document.getElementById(printFrameId)).not.toBeNull())
        const frame = document.getElementById(printFrameId) as HTMLIFrameElement
        // The empty about:blank document a fresh frame momentarily holds carries no print-ready marker,
        // so a load fired against it must not reach print and take a blank page.
        const printed = vi.fn()
        Object.defineProperty(frame.contentWindow, 'print', {value: printed, configurable: true})
        frame.contentDocument?.getElementById(printReadyMarkerId)?.remove()
        frame.onload?.(new Event('load'))
        expect(printed).not.toHaveBeenCalled()
    })

    it('reports a failed print through the error bar (print)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        apiSpies.messageBody.mockRejectedValueOnce('printer on fire')
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'File'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Print...'}))
        expect(await screen.findByText(/printer on fire/)).toBeInTheDocument()
    })

    it('reports a failed save-as through the error bar (useMenus)', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        apiSpies.saveMessageAs.mockRejectedValueOnce('disk full')
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.click(screen.getByRole('button', {name: 'File'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Save as...'}))
        expect(await screen.findByText(/disk full/)).toBeInTheDocument()
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

// Sending a reply flags its original answered. ComposeModal reports which original was acted on; this
// pins App's half of it, that the report reaches the message action and so the server flag.
describe('App: reply marking', () => {
    it('marks a reply answered as it is sent', async () => {
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([makeMessage({subject: 'Weekly report'})])
        apiSpies.send.mockResolvedValue('')
        render(<App/>)
        fireEvent.click(await screen.findByText('Weekly report'))
        fireEvent.keyDown(document.body, {key: 'r', ctrlKey: true})
        const dialog = await screen.findByRole('dialog', {name: 'New message'})
        fireEvent.click(within(dialog).getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(apiSpies.markReplied).toHaveBeenCalledWith('m1'))
        expect(apiSpies.markForwarded).not.toHaveBeenCalled()
    })
})

// The View-menu preferences are each a boolean read from localStorage on mount and written back when
// toggled. Only the reading half of one of them was pinned, so these characterise both halves of each
// before they are collapsed onto one hook.
describe('App: persisted view preferences', () => {
    const flags = [
        {label: 'Conversation view', key: 'conversationView'},
        {label: 'Unified mailbox', key: 'unifiedMailbox'},
        {label: 'Load images by default', key: 'autoLoadImages'},
    ] as const

    afterEach(() => flags.forEach((f) => localStorage.removeItem(f.key)))

    for (const {label, key} of flags) {
        it(`persists ${label} when it is toggled on`, async () => {
            apiSpies.listAccounts.mockResolvedValue([makeAccount()])
            apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
            render(<App/>)
            await screen.findByText('Inbox')
            fireEvent.click(screen.getByRole('button', {name: 'View'}))
            fireEvent.click(await screen.findByRole('menuitemcheckbox', {name: label}))
            await waitFor(() => expect(localStorage.getItem(key)).toBe('1'))
        })

        it(`reads ${label} back from storage on mount`, async () => {
            localStorage.setItem(key, '1')
            apiSpies.listAccounts.mockResolvedValue([makeAccount()])
            apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
            render(<App/>)
            await screen.findByText('Inbox')
            fireEvent.click(screen.getByRole('button', {name: 'View'}))
            // A ticked preference renders its item checked; that tick is how the stored value shows.
            const item = await screen.findByRole('menuitemcheckbox', {name: label})
            expect(item).toHaveAttribute('aria-checked', 'true')
        })
    }
})

// The four backend-backed collections the menus manage (rules, templates, contacts and calendar events)
// were loaded by four hand-written copies of one shape with nothing pinning any of them. These
// characterise the shape at App's outer interface before it is collapsed into one hook.
describe('App: managed collections', () => {
    it('loads every managed collection on mount', async () => {
        render(<App/>)
        await waitFor(() => expect(apiSpies.listRules).toHaveBeenCalled())
        expect(apiSpies.listTemplates).toHaveBeenCalled()
        expect(apiSpies.listContacts).toHaveBeenCalled()
        expect(apiSpies.listEvents).toHaveBeenCalled()
    })

    for (const [name, spy] of [
        ['rules', 'listRules'], ['templates', 'listTemplates'],
        ['contacts', 'listContacts'], ['events', 'listEvents'],
    ] as const) {
        it(`reports a failed ${name} load through the error bar`, async () => {
            apiSpies[spy].mockRejectedValueOnce(`${name} unavailable`)
            render(<App/>)
            expect(await screen.findByText(new RegExp(`${name} unavailable`))).toBeInTheDocument()
        })
    }
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
// loadMoreMessages and toggleSort). The list loads one page, appends the next as it nears the end, then
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

    it('clears the results, the snippets and the hint when the query is emptied', async () => {
        apiSpies.searchMessages.mockResolvedValue({
            hits: [{message: makeMessage({id: 's1', subject: 'Search hit'}), snippet: 'hit'}],
            degraded: true,
        })
        await renderWithInbox()
        const box = screen.getByLabelText('Search mail')
        fireEvent.change(box, {target: {value: 'broken "quote'}})
        expect(await screen.findByText('Search hit')).toBeInTheDocument()
        expect(screen.getByText('Searched as plain text')).toBeInTheDocument()

        fireEvent.change(box, {target: {value: ''}})
        await waitFor(() => expect(screen.getByText('Weekly report')).toBeInTheDocument())

        // Emptying the box resets the results, the snippets and the degraded hint rather than leaving
        // them for the next query to inherit. Typing again shows the hint only once a response says so,
        // which is observable in the window before the debounced query has run at all.
        apiSpies.searchMessages.mockResolvedValue({
            hits: [{message: makeMessage({id: 's2', subject: 'Clean hit'}), snippet: 'clean'}],
            degraded: false,
        })
        fireEvent.change(box, {target: {value: 'penguin'}})
        expect(screen.queryByText('Searched as plain text')).toBeNull()
        expect(screen.queryByText('Search hit')).toBeNull()
        expect(await screen.findByText('Clean hit')).toBeInTheDocument()
    })

    it('falls the scope back to all mail when its folder stops being a real one', async () => {
        await renderWithInbox()
        const scope = screen.getByLabelText('Search scope') as HTMLSelectElement
        fireEvent.change(scope, {target: {value: 'folder'}})
        expect(scope.value).toBe('folder')
        // The unified mailbox is not a real folder, so it cannot anchor a folder scope. The selector
        // must not claim a narrower scope than the search actually runs with.
        fireEvent.click(screen.getByRole('button', {name: 'View'}))
        fireEvent.click(await screen.findByRole('menuitemcheckbox', {name: 'Unified mailbox'}))
        await waitFor(() => expect((screen.getByLabelText('Search scope') as HTMLSelectElement).value).toBe('all'))
        localStorage.removeItem('unifiedMailbox')
    })

    it('discards a slow response that lands after the query has moved on', async () => {
        await renderWithInbox()
        let resolveFirst: (value: {hits: unknown[]; degraded: boolean}) => void = () => undefined
        apiSpies.searchMessages.mockImplementationOnce(() => new Promise((resolve) => {
            resolveFirst = resolve as typeof resolveFirst
        }))
        const box = screen.getByLabelText('Search mail')
        fireEvent.change(box, {target: {value: 'slow'}})
        await waitFor(() => expect(apiSpies.searchMessages).toHaveBeenCalledWith('slow', '', ''))

        apiSpies.searchMessages.mockResolvedValue({
            hits: [{message: makeMessage({id: 's2', subject: 'Newer hit'}), snippet: 'newer'}],
            degraded: false,
        })
        fireEvent.change(box, {target: {value: 'quick'}})
        expect(await screen.findByText('Newer hit')).toBeInTheDocument()

        // The first query answers late. It is stale, so it must not overwrite the newer results.
        await act(async () => {
            resolveFirst({
                hits: [{message: makeMessage({id: 's1', subject: 'Stale hit'}), snippet: 'stale'}],
                degraded: false,
            })
            await Promise.resolve()
        })
        expect(screen.queryByText('Stale hit')).toBeNull()
        expect(screen.getByText('Newer hit')).toBeInTheDocument()
    })

    it('reports a failed search through the error bar', async () => {
        apiSpies.searchMessages.mockRejectedValue('index unavailable')
        await renderWithInbox()
        fireEvent.change(screen.getByLabelText('Search mail'), {target: {value: 'penguin'}})
        expect(await screen.findByText(/index unavailable/)).toBeInTheDocument()
    })

    it('focuses the search box from Ctrl+K (Edit > Search)', async () => {
        await renderWithInbox()
        fireEvent.keyDown(document.body, {key: 'k', ctrlKey: true})
        expect(document.activeElement).toBe(screen.getByLabelText('Search mail'))
    })
})

describe('App: opening a conversation', () => {
    // Two messages of one exchange in the open folder, so the conversation view puts a header row above
    // them. The lookup behind the thread view returns the half the list can never show: the reply that
    // lives in Sent.
    const INBOUND = {subject: 'Lunch on Friday', id: 'm1'}
    const REPLY = {subject: 'Re: Lunch on Friday', id: 'm2'}

    async function renderWithConversation() {
        localStorage.setItem('conversationView', '1')
        apiSpies.listAccounts.mockResolvedValue([makeAccount()])
        apiSpies.listFolders.mockResolvedValue([makeFolder('inbox', 'Inbox', 'inbox')])
        apiSpies.listMessages.mockResolvedValue([
            makeMessage(INBOUND),
            makeMessage({...REPLY, date: '2026-08-28T12:00:00.000Z'}),
        ])
        apiSpies.conversation.mockResolvedValue([
            {message: makeMessage(INBOUND), folderName: 'Inbox', folderKind: 'inbox'},
            {message: makeMessage({...REPLY, id: 'm3'}), folderName: 'Sent', folderKind: 'sent'},
        ])
        const rendered = render(<App/>)
        await waitFor(() => expect(screen.getByText('2 messages')).toBeInTheDocument())
        return rendered
    }

    afterEach(() => localStorage.removeItem('conversationView'))

    it('makes the conversation header a way in rather than a label', async () => {
        await renderWithConversation()
        // The header row was decoration for a long time: it announced a conversation and offered no way to
        // reach it. It is a control now; it names what it opens.
        const header = screen.getByRole('button', {name: /Open the conversation/})
        expect(header).toBeInTheDocument()
    })

    it('opens the whole conversation, including the half that lives in another folder', async () => {
        const {container} = await renderWithConversation()
        fireEvent.click(screen.getByRole('button', {name: /Open the conversation/}))
        await waitFor(() => expect(apiSpies.conversation).toHaveBeenCalledWith('m1'))
        const thread = await waitFor(() => {
            const el = container.querySelector('.thread-view')
            expect(el).not.toBeNull()
            return el as HTMLElement
        })
        expect(within(thread).getByText('Sent')).toBeInTheDocument()
    })

    it('shows the conversation strip under an open message only while the view is on', async () => {
        const {container} = await renderWithConversation()
        fireEvent.click(screen.getAllByText('Lunch on Friday')[0])
        await waitFor(() => expect(container.querySelector('.conversation-strip')).not.toBeNull())
        // The tick governs the whole feature: with conversations off, nothing about them is on screen.
        fireEvent.click(screen.getByRole('button', {name: 'View'}))
        fireEvent.click(screen.getByText('Conversation view'))
        await waitFor(() => expect(container.querySelector('.conversation-strip')).toBeNull())
        expect(container.querySelector('.conversation-header')).toBeNull()
    })

    it('closes an open thread when the view is switched off', async () => {
        const {container} = await renderWithConversation()
        fireEvent.click(screen.getByRole('button', {name: /Open the conversation/}))
        await waitFor(() => expect(container.querySelector('.thread-view')).not.toBeNull())
        fireEvent.click(screen.getByRole('button', {name: 'View'}))
        fireEvent.click(screen.getByText('Conversation view'))
        await waitFor(() => expect(container.querySelector('.thread-view')).toBeNull())
    })

    it('leaves the thread when another message is picked', async () => {
        const {container} = await renderWithConversation()
        fireEvent.click(screen.getByRole('button', {name: /Open the conversation/}))
        await waitFor(() => expect(container.querySelector('.thread-view')).not.toBeNull())
        fireEvent.click(screen.getAllByText('Lunch on Friday')[0])
        await waitFor(() => expect(container.querySelector('.thread-view')).toBeNull())
    })
})
