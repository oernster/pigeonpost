// Both right-click menus (the message's and the folder's) close on a click outside them or on Escape and
// keep themselves inside the window when opened near an edge. The rule is one, so it is checked once here
// against each menu. jsdom has no layout, so a menu's size is stubbed on getBoundingClientRect and the
// window is jsdom's own 1024 by 768. ../api is mocked (the Wails seam): the message menu reads the tags
// already on its message when it opens.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render} from '@testing-library/react'
import type {ReactElement} from 'react'
import type {Folder, Message} from '../api'
import {FolderContextMenu} from './FolderContextMenu'
import {MessageContextMenu} from './MessageContextMenu'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    messageTags: vi.fn(),
}))

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy fails
// the test by name instead of throwing a TypeError into the nearest catch and passing.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

// The margin both menus keep from the window's edges.
const MARGIN = 8

const FOLDER = {id: 'f1', accountId: 'a1', path: 'Work', name: 'Work', kind: 'custom', unread: 0, total: 0} as Folder
const MESSAGE = {
    id: 'm1', folderId: 'f1', subject: 'S', fromName: '', fromAddress: 'a@b.c', to: [], cc: [],
    date: '2026-07-15T10:00:00.000Z', size: 1, read: false, flagged: false, hasAttachments: false,
    answered: false, forwarded: false, snippet: '', tagColours: [], snoozedUntilMs: 0,
} as unknown as Message

// Each entry makes its menu opened at (x, y), so a test can render it and then re-render it elsewhere.
type Menu = (x: number, y: number, onClose: () => void) => ReactElement

const noop = () => {}

const MENUS: [string, Menu][] = [
    ['the folder menu', (x, y, onClose) => (
        <FolderContextMenu
            folder={FOLDER} x={x} y={y} canPaste canManageFolders onClose={onClose}
            onPaste={noop} onNewSubfolder={noop} onRenameFolder={noop} onDeleteFolder={noop}
        />
    )],
    ['the message menu', (x, y, onClose) => (
        <MessageContextMenu
            message={MESSAGE} x={x} y={y} folders={[FOLDER]} tags={[]} selection={[MESSAGE]} canMoveCopy
            onClose={onClose} onReply={noop} onReplyAll={noop} onForward={noop} onSetRead={noop}
            onToggleFlag={noop} onMove={noop} onCopy={noop} onSetTag={noop}
            onCutMessages={noop} onCopyMessages={noop} onPaste={noop} canPaste={false}
            onOpenInNewTab={noop} onSaveAs={noop} onPrint={noop} onAttachToNew={noop}
            onMarkJunk={noop} onMarkNotJunk={noop} isJunk={() => false}
            onSnooze={noop} onSnoozeCustom={noop} onUnsnooze={noop}
            onDelete={noop} onDeletePermanent={noop} onCancelSend={noop}
            onBulkSetRead={noop} onBulkSetFlag={noop} onBulkMove={noop} onBulkDelete={noop}
            onBulkDeletePermanent={noop}
        />
    )],
]

// sized makes every element report this size, which is what the menus measure themselves by.
function sized(width: number, height: number) {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
        {x: 0, y: 0, top: 0, left: 0, right: width, bottom: height, width, height, toJSON: () => ({})},
    )
}

function menuOf(view: ReturnType<typeof render>): HTMLElement {
    const menu = view.container.querySelector<HTMLElement>('.context-menu')
    expect(menu, 'no .context-menu rendered').not.toBeNull()
    return menu as HTMLElement
}

beforeEach(() => {
    apiSpies.messageTags.mockReset().mockResolvedValue([])
})
afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies').toEqual([])
})

describe.each(MENUS)('%s', (_name, menuAt) => {
    const open = (x: number, y: number, onClose: () => void) => render(menuAt(x, y, onClose))

    it('closes on a click outside it but not on one inside it', () => {
        const onClose = vi.fn()
        const view = open(10, 20, onClose)
        fireEvent.mouseDown(menuOf(view))
        expect(onClose).not.toHaveBeenCalled()
        fireEvent.mouseDown(document.body)
        expect(onClose).toHaveBeenCalled()
    })

    it('closes on Escape and on no other key', () => {
        const onClose = vi.fn()
        open(10, 20, onClose)
        fireEvent.keyDown(document, {key: 'Enter'})
        expect(onClose).not.toHaveBeenCalled()
        fireEvent.keyDown(document, {key: 'Escape'})
        expect(onClose).toHaveBeenCalled()
    })

    it('opens where it was asked to when it fits', () => {
        sized(200, 300)
        const menu = menuOf(open(10, 20, vi.fn()))
        expect([menu.style.left, menu.style.top]).toEqual(['10px', '20px'])
    })

    it('moves back inside the window when opened near the right and bottom edges', () => {
        sized(200, 300)
        const menu = menuOf(open(1000, 700, vi.fn()))
        expect([menu.style.left, menu.style.top])
            .toEqual([`${window.innerWidth - 200 - MARGIN}px`, `${window.innerHeight - 300 - MARGIN}px`])
    })

    it('keeps its margin from the top left when it is bigger than the window', () => {
        sized(window.innerWidth * 2, window.innerHeight * 2)
        const menu = menuOf(open(500, 500, vi.fn()))
        expect([menu.style.left, menu.style.top]).toEqual([`${MARGIN}px`, `${MARGIN}px`])
    })

    it('places itself again when it is moved somewhere else', () => {
        sized(200, 300)
        const view = open(1000, 700, noop)
        view.rerender(menuAt(10, 20, noop))
        const menu = menuOf(view)
        expect([menu.style.left, menu.style.top]).toEqual(['10px', '20px'])
    })
})

describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
