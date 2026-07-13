// Behaviour test for the remote-calendars (CalDAV) manager at its injected-props interface. The component is
// purely presentational (all state and actions come from useCalDAVAccounts), so no api mock is needed: the
// test drives the DOM and asserts which injected callback fired. It pins the add / pull / remove flows and the
// empty state.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, within} from '@testing-library/react'
import {useState, type ComponentProps} from 'react'
import {CalDAVAccountsManager} from './CalDAVAccountsManager'
import type {CalDAVAccount} from '../api'
import {emptyCalDAVAccountForm} from '../caldavAccount'

afterEach(cleanup)

type ManagerProps = ComponentProps<typeof CalDAVAccountsManager>

function account(overrides: Partial<CalDAVAccount> = {}): CalDAVAccount {
    return {id: 'a1', displayName: 'Fastmail', baseUrl: 'https://caldav.fastmail.com', username: 'me@example.com', ...overrides} as CalDAVAccount
}

function renderManager(overrides: Partial<ManagerProps> = {}) {
    const props: ManagerProps = {
        accounts: [account()],
        adding: false,
        startAdd: vi.fn(),
        cancelAdd: vi.fn(),
        form: emptyCalDAVAccountForm(),
        setForm: vi.fn(),
        submitAdd: vi.fn(),
        pull: vi.fn(),
        pullingId: '',
        pendingDelete: null,
        setPendingDelete: vi.fn(),
        confirmRemove: vi.fn(),
        onClose: vi.fn(),
        busy: false,
        error: '',
        status: '',
        ...overrides,
    }
    const view = render(<CalDAVAccountsManager {...props}/>)
    return {...view, props}
}

describe('CalDAVAccountsManager', () => {
    it('lists accounts with their server address and offers Add account when idle', () => {
        renderManager()
        expect(screen.getByText('Fastmail')).toBeTruthy()
        expect(screen.getByText(/caldav\.fastmail\.com/)).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Done'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Add account'})).toBeTruthy()
    })

    it('shows an empty-state line when there are no accounts and no add form', () => {
        renderManager({accounts: []})
        expect(screen.getByText('No remote calendars yet.')).toBeTruthy()
    })

    it('starts the add form from the footer Add account button', () => {
        const {props} = renderManager()
        fireEvent.click(screen.getByRole('button', {name: 'Add account'}))
        expect(props.startAdd).toHaveBeenCalledOnce()
    })

    it('pulls and removes the listed account', () => {
        const {props} = renderManager()
        fireEvent.click(screen.getByRole('button', {name: 'Pull'}))
        expect(props.pull).toHaveBeenCalledWith(account())
        fireEvent.click(screen.getByRole('button', {name: 'Remove'}))
        expect(props.setPendingDelete).toHaveBeenCalledWith(account())
    })

    it('shows progress only on the row whose pull is in flight', () => {
        renderManager({
            accounts: [account({id: 'a1', displayName: 'One'}), account({id: 'a2', displayName: 'Two'})],
            pullingId: 'a1', busy: true,
        })
        // Only the pulling row reads Pulling…; the sibling still reads Pull, not a false sync label.
        expect(screen.getByRole('button', {name: 'Pulling…'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Pull'})).toBeTruthy()
    })

    it('edits the four add fields through setForm and enables submit only once complete', () => {
        // A stateful harness so the controlled inputs really update: replaying a captured synthetic event after
        // React has reset a controlled input would read a stale empty value, so drive real state instead.
        function Harness() {
            const [form, setForm] = useState(emptyCalDAVAccountForm())
            return (
                <CalDAVAccountsManager
                    accounts={[]} adding form={form} setForm={setForm}
                    startAdd={vi.fn()} cancelAdd={vi.fn()} submitAdd={vi.fn()} pull={vi.fn()} pullingId=""
                    pendingDelete={null} setPendingDelete={vi.fn()} confirmRemove={vi.fn()} onClose={vi.fn()}
                    busy={false} error="" status=""
                />
            )
        }
        const {container} = render(<Harness/>)
        const submit = () => screen.getByRole('button', {name: 'Add account'})
        // The empty form cannot be submitted.
        expect(submit()).toHaveProperty('disabled', true)

        const name = screen.getByPlaceholderText('Fastmail calendar') as HTMLInputElement
        const server = screen.getByPlaceholderText('https://caldav.fastmail.com') as HTMLInputElement
        const user = screen.getByPlaceholderText('you@example.com') as HTMLInputElement
        const password = container.querySelector('input[type="password"]') as HTMLInputElement

        fireEvent.change(name, {target: {value: 'Work'}})
        fireEvent.change(server, {target: {value: 'https://d.example.com'}})
        fireEvent.change(user, {target: {value: 'u@example.com'}})
        fireEvent.change(password, {target: {value: 'secret'}})

        expect(name.value).toBe('Work')
        expect(server.value).toBe('https://d.example.com')
        expect(user.value).toBe('u@example.com')
        expect(password.value).toBe('secret')
        // With every field filled the submit is enabled.
        expect(submit()).toHaveProperty('disabled', false)
    })

    it('submits a valid add form', () => {
        const form = {displayName: 'W', baseUrl: 'https://d.example.com', username: 'u', password: 'p'}
        const {props} = renderManager({adding: true, form})
        fireEvent.click(screen.getByRole('button', {name: 'Add account'}))
        expect(props.submitAdd).toHaveBeenCalledOnce()
    })

    it('confirms a removal through the confirmation dialog', () => {
        const {props} = renderManager({pendingDelete: account()})
        const dialog = screen.getByRole('alertdialog')
        expect(within(dialog).getByText(/Remove the account "Fastmail"/)).toBeTruthy()
        fireEvent.click(within(dialog).getByRole('button', {name: 'Remove'}))
        expect(props.confirmRemove).toHaveBeenCalledOnce()
    })

    it('surfaces an error banner', () => {
        renderManager({error: 'boom'})
        expect(screen.getByText('boom')).toBeTruthy()
    })
})
