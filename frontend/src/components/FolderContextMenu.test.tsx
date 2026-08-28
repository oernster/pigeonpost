// The folder row's right-click menu: New subfolder creates a folder under the one clicked and Paste
// files the message clipboard into it. Both entries answer to the props alone, so this renders the real
// component and drives each one. The menu clamps itself into the viewport on first paint; jsdom reports
// a zero-sized rect, so no clamping happens here and the position is not what is under test.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import type {ComponentProps} from 'react'
import {FolderContextMenu} from './FolderContextMenu'
import type {Folder} from '../api'

type MenuProps = ComponentProps<typeof FolderContextMenu>

const FOLDER = {
    id: 'f1', accountId: 'a1', path: 'Projects', name: 'Projects', kind: 'custom', unread: 0, total: 0,
} as Folder

function renderMenu(overrides: Partial<MenuProps> = {}) {
    const handlers = {
        onPaste: vi.fn(),
        onNewSubfolder: vi.fn(),
        onClose: vi.fn(),
    }
    const props: MenuProps = {
        folder: FOLDER,
        x: 10,
        y: 20,
        canPaste: true,
        canManageFolders: true,
        ...handlers,
        ...overrides,
    }
    render(<FolderContextMenu {...props}/>)
    return handlers
}

afterEach(() => cleanup())

describe('FolderContextMenu', () => {
    it('names the folder it was opened on', () => {
        renderMenu()
        expect(screen.getByText('Projects')).toBeInTheDocument()
    })

    it('reports the folder a new subfolder goes under, then closes', () => {
        const handlers = renderMenu()
        fireEvent.click(screen.getByRole('menuitem', {name: 'New subfolder'}))
        expect(handlers.onNewSubfolder).toHaveBeenCalledWith(FOLDER)
        expect(handlers.onClose).toHaveBeenCalled()
    })

    it('leaves the subfolder entry out when folders cannot be managed', () => {
        renderMenu({canManageFolders: false})
        expect(screen.queryByRole('menuitem', {name: 'New subfolder'})).toBeNull()
        expect(screen.getByRole('menuitem', {name: 'Paste'})).toBeInTheDocument()
    })

    it('pastes into the folder, then closes', () => {
        const handlers = renderMenu()
        fireEvent.click(screen.getByRole('menuitem', {name: 'Paste'}))
        expect(handlers.onPaste).toHaveBeenCalledWith(FOLDER)
        expect(handlers.onClose).toHaveBeenCalled()
    })

    it('disables Paste while the clipboard is empty', () => {
        renderMenu({canPaste: false})
        expect(screen.getByRole('menuitem', {name: 'Paste'})).toBeDisabled()
    })
})
