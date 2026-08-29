// Behaviour test for the contacts modal's import flow at its outer interface. It pins what the user is
// told after an import, which is the part that failed silently before: an import that stored nothing
// produced no message and no error, so a mis-picked file was indistinguishable from the import not
// having run. The status line now names its source file and separates new contacts from updated ones,
// and these assertions are what keep it honest.
//
// Only ../api is stubbed, since that is the Wails seam; the real modal renders.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {ContactsModal} from './ContactsModal'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    importContactsFromFile: vi.fn(),
    exportContactsToFile: vi.fn(),
    listContactGroups: vi.fn(),
    saveContactGroup: vi.fn(),
    deleteContactGroup: vi.fn(),
    saveContact: vi.fn(),
    deleteContact: vi.fn(),
}))

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy
// fails the test by name instead of throwing a TypeError into the nearest catch and passing. The
// afterEach below reports any that were reached. See src/test/apiMock.ts.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

const onChanged = vi.fn()

beforeEach(() => {
    vi.clearAllMocks()
    apiSpies.listContactGroups.mockResolvedValue([])
})

afterEach(() => {
    cleanup()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

// startImport renders the modal and clicks Import, resolving the api call with the given result.
async function startImport(result: unknown) {
    apiSpies.importContactsFromFile.mockResolvedValue(result)
    render(<ContactsModal contacts={[]} onChanged={onChanged} onClose={() => {}}/>)
    fireEvent.click(screen.getByRole('button', {name: 'Import…'}))
    await waitFor(() => expect(apiSpies.importContactsFromFile).toHaveBeenCalled())
}

it('names the file it imported, so a small count cannot be mistaken for the wrong file', async () => {
    await startImport({added: 137, updated: 0, cancelled: false, file: 'Collected Addresses.csv'})
    await screen.findByText('Imported 137 contacts from Collected Addresses.csv.')
    expect(onChanged).toHaveBeenCalled()
})

it('reports new and updated contacts separately', async () => {
    await startImport({added: 12, updated: 125, cancelled: false, file: 'Collected Addresses.csv'})
    await screen.findByText(
        'Imported 12 contacts and updated 125 existing contacts from Collected Addresses.csv.')
})

it('says when a re-import changed nothing new, rather than implying every row was added', async () => {
    await startImport({added: 0, updated: 5, cancelled: false, file: 'Personal Address Book.csv'})
    await screen.findByText('Updated 5 existing contacts from Personal Address Book.csv. None were new.')
})

it('singularises a single contact', async () => {
    await startImport({added: 1, updated: 0, cancelled: false, file: 'one.csv'})
    await screen.findByText('Imported 1 contact from one.csv.')
})

it('reports a file that yielded nothing instead of passing in silence', async () => {
    await startImport({added: 0, updated: 0, cancelled: false, file: 'empty.csv'})
    await screen.findByText('No contacts found from empty.csv.')
    expect(onChanged).not.toHaveBeenCalled()
})

it('stays silent when the file dialog is cancelled', async () => {
    await startImport({added: 0, updated: 0, cancelled: true, file: ''})
    // Nothing to wait for, so assert the absence after the promise has settled.
    await waitFor(() => expect(apiSpies.importContactsFromFile).toHaveBeenCalled())
    expect(screen.queryByText(/No contacts found/)).toBeNull()
    expect(screen.queryByText(/Imported/)).toBeNull()
    expect(onChanged).not.toHaveBeenCalled()
})

it('surfaces an import failure as an error', async () => {
    apiSpies.importContactsFromFile.mockRejectedValue(new Error('read contacts file: permission denied'))
    render(<ContactsModal contacts={[]} onChanged={onChanged} onClose={() => {}}/>)
    fireEvent.click(screen.getByRole('button', {name: 'Import…'}))
    await screen.findByText(/permission denied/)
    expect(onChanged).not.toHaveBeenCalled()
})

// The mock covers the api in both directions: the afterEach above catches a method reached with no
// spy; this catches the opposite, a spy declared under a name the api does not have, which binds to
// nothing, so every test configuring it would be configuring a stub the code can never call.
describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
