// The reworked contacts dialog: the card grid without per-row delete, Delete contact living at the
// end of an open contact's editor (still confirmed) and the auto-collect toggle persisting its
// setting.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {ContactsModal} from './ContactsModal'
import {AUTO_COLLECT_KEY} from '../autoCollect'
import type {Contact} from '../api'

const apiSpies = vi.hoisted(() => ({
    listContactGroups: vi.fn(),
    saveContact: vi.fn(),
    deleteContact: vi.fn(),
    saveContactGroup: vi.fn(),
    deleteContactGroup: vi.fn(),
    importContactsFromFile: vi.fn(),
    exportContactsToFile: vi.fn(),
}))

vi.mock('../api', () => ({
    api: {
        listContactGroups: apiSpies.listContactGroups,
        saveContact: apiSpies.saveContact,
        deleteContact: apiSpies.deleteContact,
        saveContactGroup: apiSpies.saveContactGroup,
        deleteContactGroup: apiSpies.deleteContactGroup,
        importContactsFromFile: apiSpies.importContactsFromFile,
        exportContactsToFile: apiSpies.exportContactsToFile,
    },
}))

function contact(id: string, name: string, address: string): Contact {
    return {
        id, uid: '', formattedName: name, givenName: name.split(' ')[0] ?? '',
        familyName: name.split(' ')[1] ?? '', organization: '', title: '', note: '', birthday: '',
        emails: [{label: '', address}], phones: [], addresses: [],
    } as unknown as Contact
}

const jane = contact('c1', 'Jane Doe', 'jane@example.com')

function renderContacts(contacts: Contact[] = [jane]) {
    const onChanged = vi.fn()
    render(<ContactsModal contacts={contacts} onChanged={onChanged} onClose={vi.fn()}/>)
    return {onChanged}
}

beforeEach(() => {
    apiSpies.listContactGroups.mockReset().mockResolvedValue([])
    apiSpies.saveContact.mockReset().mockResolvedValue(undefined)
    apiSpies.deleteContact.mockReset().mockResolvedValue(undefined)
    window.localStorage.removeItem(AUTO_COLLECT_KEY)
})

afterEach(() => {
    window.localStorage.removeItem(AUTO_COLLECT_KEY)
    cleanup()
})

describe('ContactsModal: layout and delete placement', () => {
    it('lists contacts without a per-row delete control', () => {
        renderContacts()
        expect(screen.getByText('Jane Doe')).toBeInTheDocument()
        expect(screen.queryByRole('button', {name: 'Delete Jane Doe'})).toBeNull()
    })

    it('opens the editor from the pencil button on a card', () => {
        renderContacts()
        fireEvent.click(screen.getByRole('button', {name: 'Edit Jane Doe'}))
        expect(screen.getByRole('button', {name: 'Save changes'})).toBeInTheDocument()
        expect(screen.getByDisplayValue('Jane')).toBeInTheDocument()
    })

    it('offers Delete contact at the end of an open contact and deletes after confirming', async () => {
        const {onChanged} = renderContacts()
        fireEvent.click(screen.getByText('Jane Doe'))
        fireEvent.click(screen.getByRole('button', {name: 'Delete contact'}))
        expect(screen.getByText(/Delete "Jane Doe"\?/)).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Delete'}))
        await waitFor(() => expect(apiSpies.deleteContact).toHaveBeenCalledWith('c1'))
        expect(onChanged).toHaveBeenCalled()
    })

    it('shows no Delete contact button on a new, unsaved contact', () => {
        renderContacts()
        fireEvent.click(screen.getByRole('button', {name: 'New contact'}))
        expect(screen.queryByRole('button', {name: 'Delete contact'})).toBeNull()
    })

    it('every added address carries a visible remove control that removes it', () => {
        renderContacts()
        fireEvent.click(screen.getByText('Jane Doe'))
        fireEvent.click(screen.getByRole('button', {name: 'Add address'}))
        fireEvent.click(screen.getByRole('button', {name: 'Add address'}))
        fireEvent.change(screen.getAllByPlaceholderText('street')[0], {target: {value: '1 First Lane'}})
        expect(screen.getAllByRole('button', {name: 'Remove address'})).toHaveLength(2)

        fireEvent.click(screen.getAllByRole('button', {name: 'Remove address'})[0])
        expect(screen.getAllByRole('button', {name: 'Remove address'})).toHaveLength(1)
        // The first address went; the surviving row is the empty second one.
        expect(screen.getByPlaceholderText('street')).toHaveValue('')
    })
})

describe('ContactsModal: the auto-collect toggle', () => {
    it('is on by default and persists turning it off and on', () => {
        renderContacts()
        const toggle = screen.getByRole('checkbox', {name: 'Add people you email to contacts automatically'})
        expect(toggle).toBeChecked()
        fireEvent.click(toggle)
        expect(window.localStorage.getItem(AUTO_COLLECT_KEY)).toBe('0')
        fireEvent.click(toggle)
        expect(window.localStorage.getItem(AUTO_COLLECT_KEY)).toBe('1')
    })

    it('reads a stored off setting', () => {
        window.localStorage.setItem(AUTO_COLLECT_KEY, '0')
        renderContacts()
        expect(screen.getByRole('checkbox', {name: 'Add people you email to contacts automatically'}))
            .not.toBeChecked()
    })
})
