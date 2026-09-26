// useMenus builds the five menu-bar definitions from App's state and actions; it also fires their
// accelerators from anywhere in the window. These tests pin both: each menu's items, labels and gating for the states the
// menus branch on, every item's action and the accelerator rules. ../api is mocked (the Wails seam).
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, renderHook} from '@testing-library/react'
import type {Folder, Message, Tag} from '../api'
import type {MenuItem} from '../components/Menu'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {snoozeChoices} from '../schedule'
import {Menus, MenusDeps, useMenus} from './useMenus'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    showDefaultAppSettings: vi.fn(),
    showDefaultMailAppSettings: vi.fn(),
}))

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy fails
// the test by name instead of throwing a TypeError into the nearest catch and passing.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

// The snooze presets are computed from the current time, so the clock is held still.
const NOW = new Date(2026, 8, 23, 10, 0)

function makeMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1', folderId: 'f1', subject: 'S', fromName: '', fromAddress: 'a@b.c',
        to: [], cc: [], date: '2026-07-15T10:00:00.000Z', size: 1, read: false, flagged: false,
        hasAttachments: false, answered: false, forwarded: false, snippet: '', tagColours: [], snoozedUntilMs: 0,
        ...overrides,
    } as Message
}

function makeFolder(id: string, name: string, kind: string): Folder {
    return {id, accountId: 'a1', path: name, name, kind, unread: 0, total: 0} as Folder
}

const FOLDERS = [makeFolder('f1', 'Inbox', 'inbox'), makeFolder('f2', 'Work', 'custom'), makeFolder('fj', 'Junk', 'junk')]
const MESSAGE = makeMessage()

// makeDeps is the state a message open in the inbox gives the menus, every action a spy.
function makeDeps(overrides: Partial<MenusDeps> = {}): MenusDeps {
    return {
        activeMessage: MESSAGE, activeOutbox: false, canMailAct: true, canReplyAll: true, canMoveCopy: true,
        selectedAccount: 'a1', accountSyncing: false, isWindows: false,
        conversationView: false, previewEnabled: true, autoLoadImages: false, unifiedMailbox: true,
        folders: FOLDERS, messageTags: [],
        saveMessageAs: vi.fn(), printMessage: vi.fn(),
        undoText: 'Undo delete', redoText: null, undoAction: vi.fn(), redoAction: vi.fn(),
        canCutNow: true, canCopyNow: true, canPasteNow: false,
        cut: vi.fn(), copy: vi.fn(), paste: vi.fn(), selectAll: vi.fn(), canSelectAll: true,
        setManagingRules: vi.fn(), setManagingTemplates: vi.fn(), focusSearch: vi.fn(),
        toggleConversationView: vi.fn(), togglePreview: vi.fn(), toggleAutoLoadImages: vi.fn(),
        toggleUnifiedMailbox: vi.fn(),
        signatureHtml: () => '<p>sig</p>', setComposeInitial: vi.fn(), setComposing: vi.fn(), setSettingUp: vi.fn(),
        sync: vi.fn(), openInNewTab: vi.fn(), openThread: vi.fn(),
        openReply: vi.fn(), openReplyAll: vi.fn(), openForward: vi.fn(), attachToNewMessage: vi.fn(),
        attachFiles: vi.fn(), setAttachPickerOpen: vi.fn(), displayMessages: [MESSAGE],
        setReadState: vi.fn(), toggleFlag: vi.fn(), toggleTag: vi.fn(),
        moveMessage: vi.fn(), copyMessage: vi.fn(), markJunk: vi.fn(), markNotJunk: vi.fn(),
        snoozeTo: vi.fn(), unsnooze: vi.fn(), setSnoozePickerFor: vi.fn(),
        setMessageToCancelSend: vi.fn(), requestDelete: vi.fn(), setMessageToPurge: vi.fn(),
        showGuide: vi.fn(), showAbout: vi.fn(), showLicence: vi.fn(), checkUpdates: vi.fn(),
        ...overrides,
    }
}

function menus(overrides: Partial<MenusDeps> = {}) {
    const deps = makeDeps(overrides)
    const {result} = renderHook(() => useMenus(deps))
    return {deps, menus: result.current}
}

// lines renders a menu one line per item: the label, [shortcut], then (disabled), (checked) or (hidden),
// with a submenu's items indented beneath their parent and a separator as ---.
function lines(items: MenuItem[], indent = ''): string[] {
    return items.flatMap((item) => {
        if (item.separator) {
            return [indent + '---']
        }
        let line = indent + item.label
        line += item.shortcut ? ` [${item.shortcut}]` : ''
        line += item.disabled ? ' (disabled)' : ''
        line += item.checked ? ' (checked)' : ''
        line += item.hidden ? ' (hidden)' : ''
        return [line, ...lines(item.submenu ?? [], indent + '  ')]
    })
}

// click finds an item by its path of labels and runs its action, failing by name when it is not there.
function click(items: MenuItem[], ...path: string[]) {
    let item: MenuItem | undefined
    for (const label of path) {
        item = items.find((i) => i.label === label)
        expect(item, `no menu item ${path.join(' > ')}`).toBeDefined()
        items = item?.submenu ?? []
    }
    item?.onClick?.()
}

const palette = (mark = (_colour: string) => '') => TAG_PALETTE.map((c) => `  ${c.name}${mark(c.colour)}`)

beforeEach(() => {
    vi.useFakeTimers({toFake: ['Date']})
    vi.setSystemTime(NOW)
    apiSpies.showDefaultAppSettings.mockReset().mockResolvedValue(undefined)
    apiSpies.showDefaultMailAppSettings.mockReset().mockResolvedValue(undefined)
})
afterEach(() => {
    cleanup()
    vi.useRealTimers()
    document.body.innerHTML = ''
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies').toEqual([])
})

describe('useMenus: the menus with a message open', () => {
    it('builds the File, Edit, View and Help menus', () => {
        const {menus: m} = menus()
        expect(lines(m.fileMenu)).toEqual(['Save as... [Ctrl+S]', 'Print... [Ctrl+P]'])
        expect(lines(m.editMenu)).toEqual([
            'Undo delete [Ctrl+Z]', 'Redo [Ctrl+Shift+Z] (disabled)', '---',
            'Cut [Ctrl+X]', 'Copy [Ctrl+C]', 'Paste [Ctrl+V] (disabled)', 'Delete [Del]',
            'Delete permanently [Shift+Del]', '---', 'Select all [Ctrl+A]', '---',
            'Search [Ctrl+K]', 'Rules', 'Templates',
        ])
        expect(lines(m.viewMenu)).toEqual([
            'Conversation view', 'Unified mailbox (checked)', 'Reading pane [F8] (checked)', 'Load images by default',
        ])
        expect(lines(m.helpMenu)).toEqual(['Guide', '---', 'About PigeonPost', 'Licence', 'Check for Updates'])
    })

    it('builds the Mail menu', () => {
        expect(lines(menus().menus.mailMenu)).toEqual([
            'Compose [Ctrl+N] (hidden)', 'Attach', '  Attach email...', '  Attach file(s)...',
            'Add account', 'Sync [F9]', '---',
            'Open in new tab [Ctrl+T]', 'Open conversation [Ctrl+Shift+T] (disabled)', '---',
            'Respond', '  Reply [Ctrl+R]', '  Reply all [Ctrl+Shift+R]', '  Forward [Ctrl+L]', '  Attach to new message',
            '---', 'Mark as read', 'Add star', 'Tag with colour', ...palette(),
            'Snooze', '  In 3 hours', '  Tomorrow morning (09:00)', '  Next Monday (09:00)', '  Pick a time...',
            '---', 'Move to', '  Work', '  Junk', 'Copy to', '  Work', '  Junk', 'Mark as junk',
            '---', 'Cancel send (disabled)',
        ])
    })
})

describe('useMenus: the Mail menu with nothing open', () => {
    it('disables every message action and offers no move targets', () => {
        const {menus: m} = menus({
            activeMessage: null, canMailAct: false, canReplyAll: false, selectedAccount: '', displayMessages: [],
        })
        expect(lines(m.mailMenu)).toEqual([
            'Compose [Ctrl+N] (disabled) (hidden)', 'Attach', '  Attach email... (disabled)',
            '  Attach file(s)... (disabled)', 'Add account', 'Sync [F9] (disabled)', '---',
            'Open in new tab [Ctrl+T] (disabled)', 'Open conversation [Ctrl+Shift+T] (disabled)', '---',
            'Respond (disabled)', '  Reply [Ctrl+R] (disabled)', '  Reply all [Ctrl+Shift+R] (disabled)',
            '  Forward [Ctrl+L] (disabled)', '  Attach to new message (disabled)',
            '---', 'Mark as read (disabled)', 'Add star (disabled)', 'Tag with colour (disabled)', ...palette(),
            'Snooze (disabled)', '  In 3 hours', '  Tomorrow morning (09:00)', '  Next Monday (09:00)', '  Pick a time...',
            '---', 'Move to (disabled)', 'Copy to (disabled)', 'Mark as junk (disabled)',
            '---', 'Cancel send (disabled)',
        ])
    })
})

describe('useMenus: the Mail menu follows the state', () => {
    it('names the opposite of the read and star states', () => {
        const mail = lines(menus({activeMessage: makeMessage({read: true, flagged: true})}).menus.mailMenu)
        expect(mail).toContain('Mark as unread')
        expect(mail).toContain('Remove star')
    })

    it('offers the Windows default-app items only on Windows, after Sync', () => {
        const mail = lines(menus({isWindows: true}).menus.mailMenu)
        const sync = mail.indexOf('Sync [F9]')
        expect(mail.slice(sync + 1, sync + 3)).toEqual(['Set as default for .eml...', 'Set as default email client...'])
        expect(lines(menus().menus.mailMenu)).not.toContain('Set as default for .eml...')
    })

    it('relabels and disables Sync while the account syncs', () => {
        expect(lines(menus({accountSyncing: true}).menus.mailMenu)).toContain('Synchronising… [F9] (disabled)')
    })

    it('offers Not junk for a message in Junk and moves it anywhere but there', () => {
        const mail = lines(menus({activeMessage: makeMessage({folderId: 'fj'})}).menus.mailMenu)
        expect(mail).toContain('Not junk')
        expect(mail).not.toContain('Mark as junk')
        expect(mail.slice(mail.indexOf('Move to') + 1, mail.indexOf('Copy to'))).toEqual(['  Inbox', '  Work'])
    })

    it('offers Unsnooze in place of the Snooze submenu for a snoozed message', () => {
        const mail = lines(menus({activeMessage: makeMessage({snoozedUntilMs: NOW.getTime() + 1})}).menus.mailMenu)
        expect(mail).toContain('Unsnooze')
        expect(mail.some((line) => line.startsWith('Snooze'))).toBe(false)
    })

    it('ticks the colour tags the message carries', () => {
        const applied = TAG_PALETTE[1].colour
        const tags = [{id: colourTagId(applied)} as Tag]
        const mail = lines(menus({messageTags: tags}).menus.mailMenu)
        const first = mail.indexOf('Tag with colour') + 1
        expect(mail.slice(first, first + TAG_PALETTE.length))
            .toEqual(palette((colour) => (colour === applied ? ' (checked)' : '')))
    })

    it('gates the other items on their own flags', () => {
        const mail = lines(menus({
            canMoveCopy: false, canReplyAll: false, activeOutbox: true, conversationView: true,
        }).menus.mailMenu)
        expect(mail).toEqual(expect.arrayContaining([
            'Move to (disabled)', 'Copy to (disabled)', 'Mark as junk (disabled)',
            '  Reply all [Ctrl+Shift+R] (disabled)', 'Cancel send', 'Open conversation [Ctrl+Shift+T]',
        ]))
    })
})

// Each row is a path of labels and what running that item must have called. Items are run whatever their
// gating, since the gate is the menu's to enforce and the shape tests above pin it.
type Wiring = [keyof Menus, string[], (d: MenusDeps) => void]

const WIRING: Wiring[] = [
    ['fileMenu', ['Save as...'], (d) => expect(d.saveMessageAs).toHaveBeenCalledWith(MESSAGE)],
    ['fileMenu', ['Print...'], (d) => expect(d.printMessage).toHaveBeenCalledWith(MESSAGE)],
    ['editMenu', ['Undo delete'], (d) => expect(d.undoAction).toHaveBeenCalled()],
    ['editMenu', ['Redo'], (d) => expect(d.redoAction).toHaveBeenCalled()],
    ['editMenu', ['Cut'], (d) => expect(d.cut).toHaveBeenCalled()],
    ['editMenu', ['Copy'], (d) => expect(d.copy).toHaveBeenCalled()],
    ['editMenu', ['Paste'], (d) => expect(d.paste).toHaveBeenCalled()],
    ['editMenu', ['Delete'], (d) => expect(d.requestDelete).toHaveBeenCalledWith(MESSAGE)],
    ['editMenu', ['Delete permanently'], (d) => expect(d.setMessageToPurge).toHaveBeenCalledWith(MESSAGE)],
    ['editMenu', ['Select all'], (d) => expect(d.selectAll).toHaveBeenCalled()],
    ['editMenu', ['Search'], (d) => expect(d.focusSearch).toHaveBeenCalled()],
    ['editMenu', ['Rules'], (d) => expect(d.setManagingRules).toHaveBeenCalledWith(true)],
    ['editMenu', ['Templates'], (d) => expect(d.setManagingTemplates).toHaveBeenCalledWith(true)],
    ['viewMenu', ['Conversation view'], (d) => expect(d.toggleConversationView).toHaveBeenCalled()],
    ['viewMenu', ['Unified mailbox'], (d) => expect(d.toggleUnifiedMailbox).toHaveBeenCalled()],
    ['viewMenu', ['Reading pane'], (d) => expect(d.togglePreview).toHaveBeenCalled()],
    ['viewMenu', ['Load images by default'], (d) => expect(d.toggleAutoLoadImages).toHaveBeenCalled()],
    ['mailMenu', ['Compose'], (d) => {
        expect(d.setComposeInitial).toHaveBeenCalledWith({bodyHtml: '<p></p><p>sig</p>'})
        expect(d.setComposing).toHaveBeenCalledWith(true)
    }],
    ['mailMenu', ['Attach', 'Attach email...'], (d) => expect(d.setAttachPickerOpen).toHaveBeenCalledWith(true)],
    ['mailMenu', ['Attach', 'Attach file(s)...'], (d) => expect(d.attachFiles).toHaveBeenCalled()],
    ['mailMenu', ['Add account'], (d) => expect(d.setSettingUp).toHaveBeenCalledWith(true)],
    ['mailMenu', ['Sync'], (d) => expect(d.sync).toHaveBeenCalled()],
    ['mailMenu', ['Open in new tab'], (d) => expect(d.openInNewTab).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Open conversation'], (d) => expect(d.openThread).toHaveBeenCalledWith(MESSAGE.id)],
    ['mailMenu', ['Respond', 'Reply'], (d) => expect(d.openReply).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Respond', 'Reply all'], (d) => expect(d.openReplyAll).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Respond', 'Forward'], (d) => expect(d.openForward).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Respond', 'Attach to new message'], (d) => expect(d.attachToNewMessage).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Mark as read'], (d) => expect(d.setReadState).toHaveBeenCalledWith(MESSAGE, true)],
    ['mailMenu', ['Add star'], (d) => expect(d.toggleFlag).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Tag with colour', TAG_PALETTE[0].name],
        (d) => expect(d.toggleTag).toHaveBeenCalledWith(colourTagId(TAG_PALETTE[0].colour), true)],
    ['mailMenu', ['Snooze', 'In 3 hours'],
        (d) => expect(d.snoozeTo).toHaveBeenCalledWith(MESSAGE, snoozeChoices(NOW)[0].at)],
    ['mailMenu', ['Snooze', 'Pick a time...'], (d) => expect(d.setSnoozePickerFor).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Move to', 'Work'], (d) => expect(d.moveMessage).toHaveBeenCalledWith(MESSAGE, 'f2')],
    ['mailMenu', ['Copy to', 'Work'], (d) => expect(d.copyMessage).toHaveBeenCalledWith(MESSAGE, 'f2')],
    ['mailMenu', ['Mark as junk'], (d) => expect(d.markJunk).toHaveBeenCalledWith(MESSAGE)],
    ['mailMenu', ['Cancel send'], (d) => expect(d.setMessageToCancelSend).toHaveBeenCalledWith(MESSAGE)],
    ['helpMenu', ['Guide'], (d) => expect(d.showGuide).toHaveBeenCalled()],
    ['helpMenu', ['About PigeonPost'], (d) => expect(d.showAbout).toHaveBeenCalled()],
    ['helpMenu', ['Licence'], (d) => expect(d.showLicence).toHaveBeenCalled()],
    ['helpMenu', ['Check for Updates'], (d) => expect(d.checkUpdates).toHaveBeenCalled()],
]

describe('useMenus: every item reaches its action', () => {
    it.each(WIRING)('%s > %j', (menu, path, called) => {
        const {deps, menus: m} = menus()
        click(m[menu], ...path)
        called(deps)
    })

    it('reaches the actions of the items only some states offer', () => {
        const junk = menus({activeMessage: makeMessage({folderId: 'fj'})})
        click(junk.menus.mailMenu, 'Not junk')
        expect(junk.deps.markNotJunk).toHaveBeenCalledWith(junk.deps.activeMessage)

        const snoozed = menus({activeMessage: makeMessage({snoozedUntilMs: NOW.getTime() + 1})})
        click(snoozed.menus.mailMenu, 'Unsnooze')
        expect(snoozed.deps.unsnooze).toHaveBeenCalledWith(snoozed.deps.activeMessage)

        const windows = menus({isWindows: true})
        click(windows.menus.mailMenu, 'Set as default for .eml...')
        click(windows.menus.mailMenu, 'Set as default email client...')
        expect(apiSpies.showDefaultAppSettings).toHaveBeenCalled()
        expect(apiSpies.showDefaultMailAppSettings).toHaveBeenCalled()
    })
})

// press sends a key to the window the way the webview does, from `target` when one is given.
function press(key: string, modifiers: {ctrlKey?: boolean; shiftKey?: boolean} = {}, target: EventTarget = window) {
    target.dispatchEvent(new KeyboardEvent('keydown', {key, bubbles: true, ...modifiers}))
}

describe('useMenus: the accelerators', () => {
    it('fire an item inside a submenu', () => {
        const {deps} = menus()
        press('r', {ctrlKey: true})
        expect(deps.openReply).toHaveBeenCalledWith(MESSAGE)
    })

    it('fire an item hidden from the menu', () => {
        const {deps} = menus()
        press('n', {ctrlKey: true})
        expect(deps.setComposing).toHaveBeenCalledWith(true)
    })

    it('skip a disabled item and a display-only one', () => {
        const {deps} = menus()
        press('t', {ctrlKey: true, shiftKey: true})
        press('a', {ctrlKey: true})
        expect(deps.openThread).not.toHaveBeenCalled()
        expect(deps.selectAll).not.toHaveBeenCalled()
    })

    it('leave Ctrl+Z to a focused text field', () => {
        const {deps} = menus()
        const field = document.body.appendChild(document.createElement('input'))
        press('z', {ctrlKey: true}, field)
        expect(deps.undoAction).not.toHaveBeenCalled()
        press('z', {ctrlKey: true})
        expect(deps.undoAction).toHaveBeenCalled()
    })

    it('do nothing while a dialog is open', () => {
        const {deps} = menus()
        document.body.appendChild(document.createElement('div')).className = 'modal'
        press('r', {ctrlKey: true})
        expect(deps.openReply).not.toHaveBeenCalled()
    })
})

// The mock covers the api in both directions: the afterEach above catches a method reached with no spy;
// this catches a spy declared under a name the api does not have.
describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
