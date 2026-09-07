// Behaviour test for the filter-rule manager at its outer interface (accounts, rules, onChanged,
// onClose). It renders the real modal and editor and drives the flows that matter: what a rule reads
// as in the list, that a destructive rule cannot be saved without a confirmation, plus what the
// confirmation for the destroy action says about the mail being gone for good.
//
// One module is stubbed: ../api (the Wails seam), so the calls the modal makes can be asserted.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {RuleManagerModal} from './RuleManagerModal'
import type {Account, Rule, RuleBackfill} from '../api'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    listFolders: vi.fn(),
    saveRule: vi.fn(),
    deleteRule: vi.fn(),
    reorderRules: vi.fn(),
    previewRuleBackfill: vi.fn(),
    runRuleBackfill: vi.fn(),
}))

// The backfill listens for its progress event, so the runtime binding it reads is stubbed too. The
// stub captures the registered listener, which is what lets a test deliver readings by hand rather
// than waiting for a real backend to emit them.
const runtimeSpies = vi.hoisted(() => ({
    listeners: new Map<string, (data: unknown) => void>(),
    EventsOn: vi.fn(),
}))
vi.mock('../../wailsjs/runtime', () => ({
    EventsOn: (name: string, cb: (data: unknown) => void) => {
        runtimeSpies.EventsOn(name)
        runtimeSpies.listeners.set(name, cb)
        return () => runtimeSpies.listeners.delete(name)
    },
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

// Both accounts share a display name on purpose: that is the ordinary case (they belong to the same
// person) and it is what made display-name labelling useless. The address is the discriminator.
const accounts = [
    {id: 'a1', displayName: 'Oliver Ernster', email: 'me@personal.example'},
    {id: 'a2', displayName: 'Oliver Ernster', email: 'me@work.example'},
] as Account[]

// buildRule is a complete, saveable rule the tests then vary.
function buildRule(overrides: Partial<Rule> = {}): Rule {
    return {
        id: 'r1',
        name: 'Newsletters',
        enabled: true,
        position: 0,
        matchMode: 'all',
        stopProcessing: false,
        accountIds: [],
        conditions: [{field: 'from', operator: 'contains', text: 'news@', caseSensitive: false}],
        actions: [{kind: 'markRead', folderId: ''}],
        ...overrides,
    } as Rule
}

// counts builds a backfill result, defaulting every kind of work to none so a test states only the
// numbers it cares about.
function counts(overrides: Partial<RuleBackfill> = {}): RuleBackfill {
    return {scanned: 0, markRead: 0, flag: 0, move: 0, destroy: 0, ...overrides} as RuleBackfill
}

// noWork is the result of a rule that matches nothing, the default for tests not about the backfill.
const noWork = counts()

function renderModal(rules: Rule[]) {
    const onChanged = vi.fn()
    const onClose = vi.fn()
    render(<RuleManagerModal accounts={accounts} rules={rules} onChanged={onChanged} onClose={onClose}/>)
    return {onChanged, onClose}
}

// cleanup() is per describe in this file, so the drain gets its own file-level hook.
afterEach(() => {
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('RuleManagerModal', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        apiSpies.listFolders.mockImplementation(async (accountId: string) =>
            accountId === 'a1'
                ? [{id: 'f2', accountId: 'a1', path: 'Receipts', name: 'Receipts', kind: 'custom', unread: 0, total: 0}]
                : [{id: 'f9', accountId: 'a2', path: 'Filed', name: 'Filed', kind: 'custom', unread: 0, total: 0}],
        )
        apiSpies.saveRule.mockResolvedValue(undefined)
        apiSpies.deleteRule.mockResolvedValue(undefined)
        apiSpies.reorderRules.mockResolvedValue(undefined)
        runtimeSpies.listeners.clear()
        apiSpies.previewRuleBackfill.mockResolvedValue(noWork)
        apiSpies.runRuleBackfill.mockResolvedValue(noWork)
    })
    afterEach(cleanup)

    it('summarises a multi-condition rule with every condition and action', async () => {
        renderModal([
            buildRule({
                matchMode: 'any',
                conditions: [
                    {field: 'senderDomain', operator: 'equals', text: 'shop.com', caseSensitive: false},
                    {field: 'subject', operator: 'contains', text: 'invoice', caseSensitive: true},
                ],
                actions: [{kind: 'markRead', folderId: ''}, {kind: 'moveTo', folderId: 'f2'}],
            }),
        ])
        await waitFor(() =>
            expect(
                screen.getByText(
                    'On any account, if Sender domain is "shop.com" or Subject contains "invoice" (match case), then mark as read, move to me@personal.example / Receipts',
                ),
            ).toBeTruthy(),
        )
    })

    it('marks a destroying rule in the list', () => {
        renderModal([buildRule({actions: [{kind: 'destroy', folderId: ''}]})])
        expect(screen.getByText('destroys')).toBeTruthy()
    })

    it('states that a rule reaches stored mail only when asked to', () => {
        renderModal([])
        expect(screen.getByText(/do not act on mail\s+already in your mailbox until you ask them to/)).toBeTruthy()
    })

    it('confirms a backfill against the counts it previewed before running it', async () => {
        const {onChanged} = renderModal([buildRule()])
        apiSpies.previewRuleBackfill.mockResolvedValue(counts({scanned: 40, markRead: 7}))
        apiSpies.runRuleBackfill.mockResolvedValue(counts({scanned: 40, markRead: 7}))

        fireEvent.click(screen.getByLabelText('Apply Newsletters to stored mail'))

        await waitFor(() => expect(screen.getByText(/7 to mark as read/)).toBeTruthy())
        // Nothing has run yet: the preview is a description; the run only happens on confirmation.
        expect(screen.getByText(/40 stored message\(s\)/)).toBeTruthy()
        expect(apiSpies.runRuleBackfill).not.toHaveBeenCalled()

        fireEvent.click(screen.getByText('Apply rule now'))
        await waitFor(() => expect(apiSpies.runRuleBackfill).toHaveBeenCalledWith('r1'))
        await waitFor(() => expect(screen.getByText('7 marked as read.')).toBeTruthy())
        expect(onChanged).toHaveBeenCalled()
    })

    it('warns that a destroying backfill cannot be undone', async () => {
        renderModal([buildRule({actions: [{kind: 'destroy', folderId: ''}]})])
        apiSpies.previewRuleBackfill.mockResolvedValue(counts({scanned: 12, destroy: 3}))

        fireEvent.click(screen.getByLabelText('Apply Newsletters to stored mail'))

        await waitFor(() => expect(screen.getByText(/3 to delete permanently/)).toBeTruthy())
        expect(screen.getByText(/does not go to Trash and no copy is kept/)).toBeTruthy()
        expect(screen.getByText('This will delete stored mail')).toBeTruthy()
    })

    it('skips the confirmation when a rule would change nothing', async () => {
        renderModal([buildRule()])

        fireEvent.click(screen.getByLabelText('Apply Newsletters to stored mail'))

        await waitFor(() => expect(screen.getByText('Nothing changed.')).toBeTruthy())
        expect(apiSpies.runRuleBackfill).not.toHaveBeenCalled()
    })

    it('shows a progress bar while a backfill scans and applies', async () => {
        renderModal([buildRule()])
        // The preview is held open so the working dialog can be inspected mid-run rather than after it.
        let finish: (c: RuleBackfill) => void = () => {}
        apiSpies.previewRuleBackfill.mockReturnValue(new Promise<RuleBackfill>((resolve) => {
            finish = resolve
        }))

        fireEvent.click(screen.getByLabelText('Apply Newsletters to stored mail'))
        await waitFor(() => expect(screen.getByRole('progressbar')).toBeTruthy())

        const emit = runtimeSpies.listeners.get('rules:backfill-progress')
        expect(emit).toBeTruthy()
        act(() => emit!({phase: 'scanning', done: 1, total: 4}))
        expect(screen.getByText('Checking folder 2 of 4...')).toBeTruthy()
        expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('25')

        act(() => emit!({phase: 'applying', done: 3, total: 6}))
        expect(screen.getByText('Applying to message 4 of 6...')).toBeTruthy()
        expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('50')

        await act(async () => {
            finish(counts({scanned: 10, markRead: 6}))
        })
        await waitFor(() => expect(screen.queryByRole('progressbar')).toBeNull())
    })

    it('fills the bar for a phase with nothing to count rather than dividing by zero', async () => {
        renderModal([buildRule()])
        let finish: (c: RuleBackfill) => void = () => {}
        apiSpies.previewRuleBackfill.mockReturnValue(new Promise<RuleBackfill>((resolve) => {
            finish = resolve
        }))

        fireEvent.click(screen.getByLabelText('Apply Newsletters to stored mail'))
        await waitFor(() => expect(screen.getByRole('progressbar')).toBeTruthy())

        const emit = runtimeSpies.listeners.get('rules:backfill-progress')
        act(() => emit!({phase: 'applying', done: 0, total: 0}))
        expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('100')
        expect(screen.getByText('Nothing to apply.')).toBeTruthy()

        await act(async () => {
            finish(noWork)
        })
    })

    it('offers no backfill for a disabled rule', () => {
        renderModal([buildRule({enabled: false})])
        const button = screen.getByLabelText('Apply Newsletters to stored mail') as HTMLButtonElement
        expect(button.disabled).toBe(true)
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

    // A rule needs at least one condition and one action, so the remove control on the last remaining
    // row could never be clicked. It is absent rather than disabled: a permanently inert cross reads as
    // broken, not as unavailable.
    it('offers no remove control while a rule has a single condition and action', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        expect(screen.queryByLabelText('Remove condition 1')).toBeNull()
        expect(screen.queryByLabelText('Remove action 1')).toBeNull()

        fireEvent.click(screen.getByText('+ Add condition'))
        expect(screen.getByLabelText('Remove condition 1')).toBeTruthy()
        expect(screen.getByLabelText('Remove condition 2')).toBeTruthy()
        // The action side is untouched by a second condition, so its lone row still offers no cross.
        expect(screen.queryByLabelText('Remove action 1')).toBeNull()
    })

    it('removes the condition the cross belongs to', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        fireEvent.click(screen.getByText('+ Add condition'))
        fireEvent.change(screen.getByLabelText('Match text 2'), {target: {value: 'digest'}})

        fireEvent.click(screen.getByLabelText('Remove condition 1'))
        expect(screen.queryByLabelText('Remove condition 1')).toBeNull()
        expect((screen.getByLabelText('Match text 1') as HTMLInputElement).value).toBe('digest')
    })

    it('adds a condition and saves both', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        fireEvent.click(screen.getByText('+ Add condition'))
        fireEvent.change(screen.getByLabelText('Match text 2'), {target: {value: 'digest'}})
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

    // The two defaults a new rule starts on: match anywhere in the message, then fire when any one
    // condition applies. Narrowing from there is easier than knowing to widen.
    it('starts a new rule on all fields and any-of-these', async () => {
        renderModal([])
        fireEvent.click(screen.getByText('New rule'))
        expect((screen.getByLabelText('Field 1') as HTMLSelectElement).value).toBe('all')
        expect((screen.getByLabelText('Match mode') as HTMLSelectElement).value).toBe('any')
    })

    it('carries the case-sensitivity toggle back on save', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))

        const toggle = screen.getByLabelText('Match case 1')
        expect(toggle.getAttribute('aria-pressed')).toBe('false')
        fireEvent.click(toggle)
        expect(screen.getByLabelText('Match case 1').getAttribute('aria-pressed')).toBe('true')

        fireEvent.click(screen.getByText('Save rule'))
        await waitFor(() => expect(apiSpies.saveRule).toHaveBeenCalledTimes(1))
        expect(apiSpies.saveRule.mock.calls[0][0].conditions[0].caseSensitive).toBe(true)
    })

    it('confirms before deleting a rule', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Delete Newsletters'))
        expect(apiSpies.deleteRule).not.toHaveBeenCalled()
        fireEvent.click(screen.getByRole('button', {name: 'Delete rule'}))
        await waitFor(() => expect(apiSpies.deleteRule).toHaveBeenCalledWith('r1'))
    })
})

describe('RuleManagerModal account scope', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        apiSpies.listFolders.mockImplementation(async (accountId: string) =>
            accountId === 'a1'
                ? [{id: 'f2', accountId: 'a1', path: 'Receipts', name: 'Receipts', kind: 'custom', unread: 0, total: 0}]
                : [{id: 'f9', accountId: 'a2', path: 'Filed', name: 'Filed', kind: 'custom', unread: 0, total: 0}],
        )
        apiSpies.saveRule.mockResolvedValue(undefined)
    })
    afterEach(cleanup)

    it('starts a rule on every account and says so', () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        expect(screen.getByText('All accounts').getAttribute('aria-pressed')).toBe('true')
        expect(screen.getByText('Every account, including any you add later')).toBeTruthy()
    })

    it('saves the accounts a rule is limited to', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        fireEvent.click(screen.getByText('me@work.example'))
        expect(screen.getByText('All accounts').getAttribute('aria-pressed')).toBe('false')

        fireEvent.click(screen.getByText('Save rule'))
        await waitFor(() => expect(apiSpies.saveRule).toHaveBeenCalledTimes(1))
        expect(apiSpies.saveRule.mock.calls[0][0].accountIds).toEqual(['a2'])
    })

    it('widens back to every account when All accounts is picked', async () => {
        renderModal([buildRule({accountIds: ['a1']})])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        fireEvent.click(screen.getByText('All accounts'))
        fireEvent.click(screen.getByText('Save rule'))
        await waitFor(() => expect(apiSpies.saveRule).toHaveBeenCalledTimes(1))
        expect(apiSpies.saveRule.mock.calls[0][0].accountIds).toEqual([])
    })

    it('offers only folders the scoped rule can reach', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledTimes(2))
        fireEvent.change(screen.getByLabelText('Action 1'), {target: {value: 'moveTo'}})

        const options = () =>
            [...(screen.getByLabelText('Destination 1') as HTMLSelectElement).options].map((o) => o.value)
        expect(options()).toEqual(['', 'f2', 'f9'])

        fireEvent.click(screen.getByText('me@personal.example'))
        expect(options()).toEqual(['', 'f2'])
    })

    // Narrowing the scope after choosing a destination would otherwise leave a move pointing at an
    // account the rule can no longer see, which would silently do nothing on every sync.
    it('clears a destination the narrowed scope can no longer reach', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledTimes(2))
        fireEvent.change(screen.getByLabelText('Action 1'), {target: {value: 'moveTo'}})
        fireEvent.change(screen.getByLabelText('Destination 1'), {target: {value: 'f9'}})
        expect((screen.getByText('Save rule') as HTMLButtonElement).disabled).toBe(false)

        fireEvent.click(screen.getByText('me@personal.example'))
        expect((screen.getByLabelText('Destination 1') as HTMLSelectElement).value).toBe('')
        expect((screen.getByText('Save rule') as HTMLButtonElement).disabled).toBe(true)
    })

    it('names the scope in the rule summary', () => {
        renderModal([buildRule({accountIds: ['a1', 'a2']})])
        expect(
            screen.getByText(/^On me@personal.example and me@work.example, if From contains "news@"/),
        ).toBeTruthy()
    })
})

// The chips, the move destinations and the summary must all name an account by something that tells
// two accounts apart. Labelling by display name left four chips reading "Oliver Ernster".
describe('RuleManagerModal account labelling', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        apiSpies.listFolders.mockImplementation(async (accountId: string) =>
            accountId === 'a1'
                ? [{id: 'f2', accountId: 'a1', path: 'Receipts', name: 'Receipts', kind: 'custom', unread: 0, total: 0}]
                : [{id: 'f9', accountId: 'a2', path: 'Filed', name: 'Filed', kind: 'custom', unread: 0, total: 0}],
        )
    })
    afterEach(cleanup)

    it('names each account chip by its address, not the shared display name', () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        expect(screen.getByText('me@personal.example')).toBeTruthy()
        expect(screen.getByText('me@work.example')).toBeTruthy()
        expect(screen.queryByText('Oliver Ernster')).toBeNull()
    })

    it('names each move destination by its account address', async () => {
        renderModal([buildRule()])
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        await waitFor(() => expect(apiSpies.listFolders).toHaveBeenCalledTimes(2))
        fireEvent.change(screen.getByLabelText('Action 1'), {target: {value: 'moveTo'}})
        const labels = [...(screen.getByLabelText('Destination 1') as HTMLSelectElement).options].map((o) => o.text)
        expect(labels).toEqual(['Choose a folder', 'me@personal.example / Receipts', 'me@work.example / Filed'])
    })

    it('falls back to the display name when an account has no address', () => {
        const named = [{id: 'a1', displayName: 'Oliver Ernster', email: ''}] as Account[]
        render(
            <RuleManagerModal accounts={named} rules={[buildRule()]} onChanged={() => {}} onClose={() => {}}/>,
        )
        fireEvent.click(screen.getByLabelText('Edit Newsletters'))
        expect(screen.getByText('Oliver Ernster')).toBeTruthy()
    })
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
