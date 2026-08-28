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
        onRenameFolder: vi.fn(),
        onDeleteFolder: vi.fn(),
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

    it('reports the folder to rename, then closes', () => {
        const handlers = renderMenu()
        fireEvent.click(screen.getByRole('menuitem', {name: 'Rename folder'}))
        expect(handlers.onRenameFolder).toHaveBeenCalledWith(FOLDER)
        expect(handlers.onClose).toHaveBeenCalled()
    })

    it('leaves the rename entry out on a well-known folder, as the row does', () => {
        renderMenu({folder: {...FOLDER, kind: 'inbox'} as Folder})
        expect(screen.queryByRole('menuitem', {name: 'Rename folder'})).toBeNull()
    })

    it('marks each entry with the glyph its button carries elsewhere', () => {
        // The plus on the Folders heading, the pencil and the cross on the row's hover toolbar, so a
        // menu entry and the button doing the same thing read as one action. Paste has no button of
        // its own, so its gutter stays empty and only holds the labels in line.
        renderMenu()
        const iconOf = (name: string) =>
            screen.getByRole('menuitem', {name}).querySelector('.context-item-icon')?.textContent
        expect(iconOf('New subfolder')).toBe('+')
        expect(iconOf('Rename folder')).toBe('✎')
        expect(iconOf('Delete folder')).toBe('×')
        expect(iconOf('Paste')).toBe('')
    })

    it('reports the folder to delete, then closes', () => {
        const handlers = renderMenu()
        fireEvent.click(screen.getByRole('menuitem', {name: 'Delete folder'}))
        expect(handlers.onDeleteFolder).toHaveBeenCalledWith(FOLDER)
        expect(handlers.onClose).toHaveBeenCalled()
    })

    it('marks the delete entry as the destructive one', () => {
        renderMenu()
        expect(screen.getByRole('menuitem', {name: 'Delete folder'})).toHaveClass('danger')
    })

    it('leaves the delete entry out on a well-known folder, as the row does', () => {
        // The row's red cross appears on custom folders only, so a folder the server gave a role to
        // cannot be deleted from the menu either.
        renderMenu({folder: {...FOLDER, kind: 'inbox'} as Folder})
        expect(screen.queryByRole('menuitem', {name: 'Delete folder'})).toBeNull()
    })

    it('leaves the delete entry out when folders cannot be managed', () => {
        renderMenu({canManageFolders: false})
        expect(screen.queryByRole('menuitem', {name: 'Delete folder'})).toBeNull()
    })

    it('draws no empty group when a POP3 account leaves only Paste', () => {
        // Every folder action is absent there, so the rule that separates them from Paste would
        // otherwise sit directly under the one below the header.
        const {container} = render(
            <FolderContextMenu
                folder={FOLDER}
                x={10}
                y={20}
                canPaste
                canManageFolders={false}
                onPaste={vi.fn()}
                onNewSubfolder={vi.fn()}
                onRenameFolder={vi.fn()}
                onDeleteFolder={vi.fn()}
                onClose={vi.fn()}
            />,
        )
        expect(container.querySelectorAll('.context-sep')).toHaveLength(1)
        expect(screen.getAllByRole('menuitem')).toHaveLength(1)
    })
})
