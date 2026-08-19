// Behaviour test for the filter-rule manager at its outer interface (accounts, rules, onChanged,
// onClose). It renders the real modal and editor and drives the flows that matter: what a rule reads
// as in the list, that a destructive rule cannot be saved without a confirmation, plus what the
// confirmation for the destroy action says about the mail being gone for good.
//
// One module is stubbed: ../api (the Wails seam), so the calls the modal makes can be asserted.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {RuleManagerModal} from './RuleManagerModal'
import type {Account, Rule} from '../api'

const apiSpies = vi.hoisted(() => ({
    listFolders: vi.fn(),
    saveRule: vi.fn(),
    deleteRule: vi.fn(),
    reorderRules: vi.fn(),
}))

vi.mock('../api', () => ({
    api: {
        listFolders: apiSpies.listFolders,
        saveRule: apiSpies.saveRule,
        deleteRule: apiSpies.deleteRule,
        reorderRules: apiSpies.reorderRules,
    },
}))

const accounts = [{id: 'a1', displayName: 'Personal'}] as Account[]

// buildRule is a complete, saveable rule the tests then vary.
function buildRule(overrides: Partial<Rule> = {}): Rule {
    return {
        id: 'r1',
        name: 'Newsletters',
        enabled: true,
        position: 0,
        matchMode: 'all',
        stopProcessing: false,
        conditions: [{field: 'from', operator: 'contains', text: 'news@'}],
        actions: [{kind: 'markRead', folderId: ''}],
        ...overrides,
    } as Rule
}

function renderModal(rules: Rule[]) {
    const onChanged = vi.fn()
    const onClose = vi.fn()
    render(<RuleManagerModal accounts={accounts} rules={rules} onChanged={onChanged} onClose={onClose}/>)
    return {onChanged, onClose}
}

describe('RuleManagerModal', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        apiSpies.listFolders.mockResolvedValue([
            {id: 'f2', accountId: 'a1', path: 'Receipts', name: 'Receipts', kind: 'custom', unread: 0, total: 0},
        ])
        apiSpies.saveRule.mockResolvedValue(undefined)
        apiSpies.deleteRule.mockResolvedValue(undefined)
        apiSpies.reorderRules.mockResolvedValue(undefined)
    })
    afterEach(cleanup)

    it('summarises a multi-condition rule with every condition and action', async () => {
        renderModal([
            buildRule({
                matchMode: 'any',
                conditions: [
                    {field: 'senderDomain', operator: 'equals', text: 'shop.com'},
                    {field: 'subject', operator: 'contains', text: 'invoice'},
                ],
                actions: [{kind: 'markRead', folderId: ''}, {kind: 'moveTo', folderId: 'f2'}],
            }),
        ])
        await waitFor(() =>
            expect(
                screen.getByText(
                    'If Sender domain is "shop.com" or Subject contains "invoice", then mark as read, move to Personal / Receipts',
                ),
            ).toBeTruthy(),
        )
    })

    it('marks a destroying rule in the list', () => {
        renderModal([buildRule({actions: [{kind: 'destroy', folderId: ''}]})])
        expect(screen.getByText('destroys')).toBeTruthy()
    })

    it('states that rules never act on mail already in the mailbox', () => {
        renderModal([])
        expect(screen.getByText(/never act on mail\s+already in your mailbox/)).toBeTruthy()
    })

    it('saves a flag-only rule without a confirmation', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        fireEvent.click(screen.getByText('Save rule'))
        await waitFor(() => expect(apiSpies.saveRule).toHaveBeenCalledTimes(1))
    })

    it('will not save a destroying rule until the warning is confirmed', async () => {
        renderModal([buildRule({actions: [{kind: 'destroy', folderId: ''}]})])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        fireEvent.click(screen.getByText('Save rule'))

        expect(apiSpies.saveRule).not.toHaveBeenCalled()
        expect(screen.getByText('This rule destroys mail')).toBeTruthy()
        // The warning must be explicit that nothing is recoverable, because a rule runs unattended.
        expect(screen.getByText(/does not go to Trash, no copy is kept/)).toBeTruthy()

        fireEvent.click(screen.getByText('Save destroying rule'))
        await waitFor(() => expect(apiSpies.saveRule).toHaveBeenCalledTimes(1))
        expect(apiSpies.saveRule.mock.calls[0][0].actions[0].kind).toBe('destroy')
    })

    it('blocks a move rule that names no destination', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalled())
        fireEvent.change(screen.getByLabelText('Action 1'), {target: {value: 'moveTo'}})

        const save = screen.getByText('Save rule') as HTMLButtonElement
        expect(save.disabled).toBe(true)

        fireEvent.change(screen.getByLabelText('Destination 1'), {target: {value: 'f2'}})
        expect((screen.getByText('Save rule') as HTMLButtonElement).disabled).toBe(false)
    })

    it('adds a condition and saves both', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        fireEvent.click(screen.getByText('Add condition'))
        fireEvent.change(screen.getAllByLabelText('Match text')[1], {target: {value: 'digest'}})
        fireEvent.click(screen.getByText('Save rule'))
        await waitFor(() => expect(apiSpies.saveRule).toHaveBeenCalledTimes(1))
        expect(apiSpies.saveRule.mock.calls[0][0].conditions).toHaveLength(2)
    })

    it('toggles a rule off without opening the editor', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Disable Newsletters'))
        await waitFor(() => expect(apiSpies.saveRule).toHaveBeenCalledTimes(1))
        expect(apiSpies.saveRule.mock.calls[0][0].enabled).toBe(false)
    })

    it('reorders rules by swapping their ids', async () => {
        renderModal([buildRule(), buildRule({id: 'r2', name: 'Receipts'})])
        fireEvent.click(screen.getByLabelText('Move Receipts up'))
        await waitFor(() => expect(apiSpies.reorderRules).toHaveBeenCalledWith(['r2', 'r1']))
    })

    it('confirms before deleting a rule', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Delete Newsletters'))
        expect(apiSpies.deleteRule).not.toHaveBeenCalled()
        fireEvent.click(screen.getByRole('button', {name: 'Delete rule'}))
        await waitFor(() => expect(apiSpies.deleteRule).toHaveBeenCalledWith('r1'))
    })
})
