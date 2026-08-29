// Characterisation test for the account-setup modal at its stable outer interface (account, onClose,
// onSaved). It renders the real modal and drives each flow, asserting the DOM plus which api call fired with
// what. The interface it pins does not move as the modal is decomposed in Phase 2 (the provider chooser, the
// signature rich-text field with its link editor and the OAuth-vs-IMAP details form are lifted out beneath
// these same props), so this suite staying green is the proof each extraction preserved behaviour.
//
// Two modules are stubbed: ../api (the Wails seam) and @tiptap/react (real ProseMirror does not run reliably
// in jsdom). The tiptap stub returns a fixed editor so the toolbar renders and the saved signature is known.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import type {ComponentProps} from 'react'
import {AccountSetupModal} from './AccountSetupModal'
import type {Account} from '../api'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    addAccount: vi.fn(),
    updateAccount: vi.fn(),
    updateAccountProfile: vi.fn(),
    signInMicrosoft: vi.fn(),
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

// The tiptap stub: a fixed non-empty editor whose HTML is known, so a save carries a predictable signature
// and the toolbar buttons have something to call. EditorContent renders nothing.
vi.mock('@tiptap/react', () => {
    const chain = () => {
        const c: Record<string, () => unknown> = {}
        for (const m of ['focus', 'toggleBold', 'toggleItalic', 'extendMarkRange', 'setLink', 'unsetLink', 'run']) {
            c[m] = () => c
        }
        return c
    }
    const editor = {
        isActive: () => false,
        isEmpty: false,
        getHTML: () => '<p>My signature</p>',
        getAttributes: () => ({}),
        chain,
    }
    return {useEditor: () => editor, EditorContent: () => null}
})

type ModalProps = ComponentProps<typeof AccountSetupModal>

function makeAccount(overrides: Partial<Account> = {}): Account {
    return {
        id: 'acc1', displayName: 'Existing User', email: 'user@example.com', protocol: 'imap',
        inHost: 'imap.example.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.example.com', outPort: 587, outSecurity: 'starttls',
        signature: '', auth: 'password', identities: [],
        ...overrides,
    } as Account
}

function renderModal(overrides: Partial<ModalProps> = {}) {
    const onClose = vi.fn()
    const onSaved = vi.fn()
    const view = render(<AccountSetupModal account={null} onClose={onClose} onSaved={onSaved} {...overrides}/>)
    const passwordInput = () => view.container.querySelector<HTMLInputElement>('input[type="password"]')
    return {...view, onClose, onSaved, passwordInput}
}

// setValue changes a controlled input or select.
function setValue(el: Element | null, value: string) {
    fireEvent.change(el as Element, {target: {value}})
}

beforeEach(() => {
    apiSpies.addAccount.mockReset().mockResolvedValue(undefined)
    apiSpies.updateAccount.mockReset().mockResolvedValue(undefined)
    apiSpies.updateAccountProfile.mockReset().mockResolvedValue(undefined)
    apiSpies.signInMicrosoft.mockReset().mockResolvedValue('signed@outlook.com')
})

afterEach(() => {
    cleanup()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('AccountSetupModal: provider chooser', () => {
    it('lists the providers when adding', () => {
        renderModal()
        expect(screen.getByRole('dialog', {name: 'Add account'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Microsoft'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Gmail'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: /Set up manually/})).toBeInTheDocument()
    })

    it('goes to the details form with servers prefilled when a provider is chosen', () => {
        renderModal()
        fireEvent.click(screen.getByRole('button', {name: 'Gmail'}))
        expect(screen.getByRole('heading', {name: 'Add Gmail'})).toBeInTheDocument()
        expect(screen.getByDisplayValue('imap.gmail.com')).toBeInTheDocument()
        expect(screen.getByDisplayValue('smtp.gmail.com')).toBeInTheDocument()
    })

    it('collects only the name for a Microsoft account', () => {
        renderModal()
        fireEvent.click(screen.getByRole('button', {name: 'Microsoft'}))
        expect(screen.getByRole('heading', {name: 'Add Microsoft'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Continue with Microsoft'})).toBeInTheDocument()
        expect(screen.queryByText('Email')).toBeNull()
        expect(screen.queryByText('Password')).toBeNull()
    })

    // The prerequisite has to be stated before the sign-in, not after it fails: Microsoft ships a new
    // mailbox with IMAP off, so without this the first thing a new user meets is a sign-in that succeeds
    // and an account that will not add.
    it('warns that IMAP must be switched on before a Microsoft sign-in', () => {
        renderModal()
        fireEvent.click(screen.getByRole('button', {name: 'Microsoft'}))
        expect(screen.getByText(/IMAP is switched off by default/)).toBeInTheDocument()
        expect(screen.getByRole('link', {name: 'How to turn on IMAP'})).toBeInTheDocument()
    })

    it('goes to an empty manual form', () => {
        renderModal()
        fireEvent.click(screen.getByRole('button', {name: /Set up manually/}))
        expect(screen.getByText('Incoming (IMAP)')).toBeInTheDocument()
        expect(screen.getByPlaceholderText('imap.example.com')).toHaveValue('')
    })

    it('cancels from the provider step', () => {
        const {onClose} = renderModal()
        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
        expect(onClose).toHaveBeenCalled()
    })

    it('returns to the provider chooser with Back', () => {
        renderModal()
        fireEvent.click(screen.getByRole('button', {name: /Set up manually/}))
        fireEvent.click(screen.getByRole('button', {name: 'Back'}))
        expect(screen.getByRole('button', {name: 'Microsoft'})).toBeInTheDocument()
    })
})

describe('AccountSetupModal: manual add', () => {
    function startManual() {
        const view = renderModal()
        fireEvent.click(screen.getByRole('button', {name: /Set up manually/}))
        return view
    }

    it('guesses the server hosts from the email domain', () => {
        startManual()
        setValue(screen.getByPlaceholderText('jane@example.com'), 'jane@acme.com')
        expect(screen.getByPlaceholderText('imap.example.com')).toHaveValue('imap.acme.com')
        expect(screen.getByPlaceholderText('smtp.example.com')).toHaveValue('smtp.acme.com')
    })

    it('tracks the POP3 port when the protocol changes', () => {
        startManual()
        expect(screen.getByDisplayValue('993')).toBeInTheDocument()
        setValue(screen.getByDisplayValue('IMAP (keeps mail on the server)'), 'pop3')
        expect(screen.getByDisplayValue('995')).toBeInTheDocument()
    })

    it('keeps Add account disabled until the form is valid', () => {
        startManual()
        expect(screen.getByRole('button', {name: 'Add account'})).toBeDisabled()
    })

    it('adds the account and reports the saved email', async () => {
        const {onSaved, passwordInput} = startManual()
        setValue(screen.getByPlaceholderText('Jane Doe'), 'Jane Doe')
        setValue(screen.getByPlaceholderText('jane@example.com'), 'jane@acme.com')
        setValue(passwordInput(), 'secret')
        fireEvent.click(screen.getByRole('button', {name: 'Add account'}))
        await waitFor(() => expect(onSaved).toHaveBeenCalledWith('jane@acme.com'))
        expect(apiSpies.addAccount).toHaveBeenCalledWith(expect.objectContaining({
            displayName: 'Jane Doe', email: 'jane@acme.com', password: 'secret', protocol: 'imap',
            inHost: 'imap.acme.com', outHost: 'smtp.acme.com', signature: '<p>My signature</p>', identities: [],
        }))
    })

    it('surfaces an error when adding fails', async () => {
        const {onSaved, passwordInput} = startManual()
        apiSpies.addAccount.mockRejectedValueOnce('server unreachable')
        setValue(screen.getByPlaceholderText('Jane Doe'), 'Jane Doe')
        setValue(screen.getByPlaceholderText('jane@example.com'), 'jane@acme.com')
        setValue(passwordInput(), 'secret')
        fireEvent.click(screen.getByRole('button', {name: 'Add account'}))
        expect(await screen.findByText('server unreachable')).toBeInTheDocument()
        expect(onSaved).not.toHaveBeenCalled()
    })

    it('adds a send-as identity and includes it in the save', async () => {
        const {onSaved, passwordInput} = startManual()
        setValue(screen.getByPlaceholderText('Jane Doe'), 'Jane Doe')
        setValue(screen.getByPlaceholderText('jane@example.com'), 'jane@acme.com')
        setValue(passwordInput(), 'secret')
        fireEvent.click(screen.getByRole('button', {name: 'Add address'}))
        setValue(screen.getByPlaceholderText('alias@example.com'), 'alias@acme.com')
        fireEvent.click(screen.getByRole('button', {name: 'Add account'}))
        await waitFor(() => expect(onSaved).toHaveBeenCalled())
        expect(apiSpies.addAccount).toHaveBeenCalledWith(expect.objectContaining({
            identities: [{name: '', address: 'alias@acme.com'}],
        }))
    })

    it('removes a send-as identity row', () => {
        startManual()
        fireEvent.click(screen.getByRole('button', {name: 'Add address'}))
        expect(screen.getByPlaceholderText('alias@example.com')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Remove'}))
        expect(screen.queryByPlaceholderText('alias@example.com')).toBeNull()
    })

    it('renders the signature editor toolbar', () => {
        startManual()
        expect(screen.getByText('Signature')).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Bold'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Italic'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Link'})).toBeInTheDocument()
    })

    it('opens the signature link row and closes it on apply', () => {
        startManual()
        expect(screen.queryByPlaceholderText('https://example.com')).toBeNull()
        fireEvent.click(screen.getByRole('button', {name: 'Link'}))
        setValue(screen.getByPlaceholderText('https://example.com'), 'example.com')
        fireEvent.click(screen.getByRole('button', {name: 'Apply'}))
        expect(screen.queryByPlaceholderText('https://example.com')).toBeNull()
    })
})

describe('AccountSetupModal: editing', () => {
    it('opens straight to the details form with a locked email', () => {
        renderModal({account: makeAccount()})
        expect(screen.getByRole('dialog', {name: 'Edit account'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Save changes'})).toBeInTheDocument()
        expect(screen.getByDisplayValue('user@example.com')).toHaveClass('locked')
        expect(screen.getByText(/Leave the password blank/)).toBeInTheDocument()
    })

    it('saves changes to an existing account', async () => {
        const {onSaved} = renderModal({account: makeAccount()})
        setValue(screen.getByDisplayValue('Existing User'), 'New Name')
        fireEvent.click(screen.getByRole('button', {name: 'Save changes'}))
        await waitFor(() => expect(onSaved).toHaveBeenCalledWith('user@example.com'))
        expect(apiSpies.updateAccount).toHaveBeenCalledWith(expect.objectContaining({
            email: 'user@example.com', displayName: 'New Name',
        }))
    })
})

describe('AccountSetupModal: oauth account', () => {
    const oauth = () => makeAccount({auth: 'oauth2', email: 'ms@outlook.com', displayName: 'MS User'})

    it('edits only the profile, hiding the password and server fields', async () => {
        const {onSaved} = renderModal({account: oauth()})
        expect(screen.queryByText('Password')).toBeNull()
        expect(screen.queryByText('Incoming (IMAP)')).toBeNull()
        setValue(screen.getByDisplayValue('MS User'), 'MS Renamed')
        fireEvent.click(screen.getByRole('button', {name: 'Save changes'}))
        await waitFor(() => expect(onSaved).toHaveBeenCalledWith('ms@outlook.com'))
        expect(apiSpies.updateAccountProfile).toHaveBeenCalledWith(expect.objectContaining({
            email: 'ms@outlook.com', displayName: 'MS Renamed', signature: '<p>My signature</p>',
        }))
        expect(apiSpies.updateAccount).not.toHaveBeenCalled()
    })
})

describe('AccountSetupModal: microsoft add', () => {
    function startMicrosoft() {
        const view = renderModal()
        fireEvent.click(screen.getByRole('button', {name: 'Microsoft'}))
        setValue(screen.getByPlaceholderText('Jane Doe'), 'MS Add')
        return view
    }

    it('signs in through OAuth and reports the signed-in address', async () => {
        const {onSaved} = startMicrosoft()
        fireEvent.click(screen.getByRole('button', {name: 'Continue with Microsoft'}))
        expect(apiSpies.signInMicrosoft).toHaveBeenCalledWith({
            displayName: 'MS Add', signature: '<p>My signature</p>', identities: [],
        })
        await waitFor(() => expect(onSaved).toHaveBeenCalledWith('signed@outlook.com'))
    })

    // The signature used to be reachable only by editing the account after it was added, which made
    // Microsoft the one provider whose add form was incomplete.
    it('offers the signature and send-as fields before the sign-in, without the email field', () => {
        startMicrosoft()
        expect(screen.getByText('Signature')).toBeInTheDocument()
        expect(screen.getByText('Send-as addresses')).toBeInTheDocument()
        expect(screen.queryByPlaceholderText('jane@example.com')).toBeNull()
    })

    it('surfaces a sign-in error and re-enables the button', async () => {
        apiSpies.signInMicrosoft.mockRejectedValueOnce('consent denied')
        const {onSaved} = startMicrosoft()
        fireEvent.click(screen.getByRole('button', {name: 'Continue with Microsoft'}))
        expect(await screen.findByText('consent denied')).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Continue with Microsoft'})).toBeEnabled()
        expect(onSaved).not.toHaveBeenCalled()
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
