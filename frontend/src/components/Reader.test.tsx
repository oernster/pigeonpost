// Characterization test for the reader at its stable outer interface: its props and its observable
// behaviour. It renders the real Reader with the Wails seam (../api) mocked, drives each interaction and
// asserts the resulting DOM plus which api calls fired. The interface it pins does not move as the reader
// is decomposed in Phase 2 (the tag-colour menu, the toolbar and the attachments block are lifted out
// beneath these same props), so this suite staying green is the proof each extraction preserved behaviour.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor, within} from '@testing-library/react'
import type {ComponentProps} from 'react'
import userEvent from '@testing-library/user-event'
import {Reader} from './Reader'
import type {Folder, Message, MessageBody, Tag} from '../api'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {formatBytes} from '../readerFormat'

// The reader reaches the backend only through ../api, so mocking that one module isolates it fully. vi.hoisted
// lifts the spy set above the hoisted vi.mock factory so the factory can close over it.
const apiSpies = vi.hoisted(() => ({
    openExternal: vi.fn(),
    saveAttachment: vi.fn(),
    openAttachment: vi.fn(),
    openEmailAttachment: vi.fn(),
    saveAllAttachments: vi.fn(),
    loadRemoteImages: vi.fn(),
}))

vi.mock('../api', () => ({
    api: {
        openExternal: apiSpies.openExternal,
        saveAttachment: apiSpies.saveAttachment,
        openAttachment: apiSpies.openAttachment,
        openEmailAttachment: apiSpies.openEmailAttachment,
        saveAllAttachments: apiSpies.saveAllAttachments,
        loadRemoteImages: apiSpies.loadRemoteImages,
    },
}))

type ReaderProps = ComponentProps<typeof Reader>

const RED = '#e05252'
const ORANGE = '#e8833a'

function makeMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1',
        folderId: 'inbox',
        accountId: '',
        subject: 'Hello there',
        fromName: 'Alice Example',
        fromAddress: 'alice@example.com',
        to: [{name: 'Bob Jones', address: 'bob@example.com'}],
        cc: [],
        date: '2026-07-11T10:00:00.000Z',
        size: 1024,
        read: false,
        flagged: false,
        hasAttachments: false,
        answered: false,
        forwarded: false,
        snippet: 'A short snippet',
        tagColours: [],
        ...overrides,
    }
}

function makeBody(overrides: Partial<MessageBody> = {}): MessageBody {
    return {
        plain: '',
        html: '<p>Message body</p>',
        hasInvite: false,
        attachments: [],
        ...overrides,
    }
}

function makeFolder(overrides: Partial<Folder> = {}): Folder {
    return {id: 'f', accountId: 'a1', path: '', name: 'Folder', kind: '', unread: 0, total: 0, ...overrides}
}

function makeTag(colour: string, name: string): Tag {
    return {id: colourTagId(colour), name, colour}
}

// renderReader fills every required prop with a spy or a sensible default, so each test overrides only what
// it exercises. The returned handlers are the same spies passed in, for call assertions, alongside the
// effective message object so a test can assert a callback fired with it.
function renderReader(overrides: Partial<ReaderProps> = {}) {
    const handlers = {
        onToggleRead: vi.fn(),
        onReply: vi.fn(),
        onReplyAll: vi.fn(),
        onForward: vi.fn(),
        onDelete: vi.fn(),
        onCancelSend: vi.fn(),
        onMove: vi.fn(),
        onCopy: vi.fn(),
        onToggleTag: vi.fn(),
        onSelectTab: vi.fn(),
        onCloseTab: vi.fn(),
    }
    const props: ReaderProps = {
        message: makeMessage(),
        folders: [],
        canMoveCopy: false,
        autoLoadImages: false,
        dark: false,
        tags: [],
        messageTags: [],
        body: makeBody(),
        bodyLoading: false,
        tabs: [],
        ...handlers,
        ...overrides,
    }
    const view = render(<Reader {...props}/>)
    // message is returned for call assertions; it is only read by tests that pass a concrete message, so
    // it is narrowed to Message (the message: null case never touches it).
    return {...view, ...handlers, message: props.message as Message}
}

// openColourMenu clicks the Colour trigger and returns the swatch group once it is open.
async function openColourMenu(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole('button', {name: /Colour/}))
    return screen.getByRole('group', {name: 'Tag colour'})
}

beforeEach(() => {
    apiSpies.openExternal.mockReset().mockResolvedValue(undefined)
    apiSpies.saveAttachment.mockReset().mockResolvedValue(undefined)
    apiSpies.openAttachment.mockReset().mockResolvedValue(undefined)
    apiSpies.saveAllAttachments.mockReset().mockResolvedValue(undefined)
    apiSpies.loadRemoteImages.mockReset().mockResolvedValue('')
    apiSpies.openEmailAttachment.mockReset().mockResolvedValue({
        subject: 'Nested subject', from: 'x@y.com', to: 'me@z.com', date: '', html: '<p>eml body</p>', plain: '',
    })
})

afterEach(() => cleanup())

describe('Reader: empty and basic render', () => {
    it('shows the prompt and no toolbar when no message is selected', () => {
        renderReader({message: null})
        expect(screen.getByText('Select a message to read.')).toBeInTheDocument()
        expect(screen.queryByRole('button', {name: 'Reply'})).toBeNull()
    })

    it('renders the subject, sender and body of the selected message', () => {
        const {container} = renderReader()
        expect(screen.getByRole('heading', {name: 'Hello there', level: 2})).toBeInTheDocument()
        expect(screen.getByText('From')).toBeInTheDocument()
        expect(screen.getByText('Alice Example <alice@example.com>')).toBeInTheDocument()
        // The body renders inside the sandboxed iframe, so its HTML lives in the frame's srcdoc.
        const frame = container.querySelector('iframe.reader-html-frame') as HTMLIFrameElement
        expect(frame).not.toBeNull()
        expect(frame.getAttribute('srcdoc')).toContain('<p>Message body</p>')
    })

    it('falls back to placeholders for an empty subject and unknown sender', () => {
        renderReader({message: makeMessage({subject: '', fromName: '', fromAddress: ''})})
        expect(screen.getByRole('heading', {name: '(no subject)', level: 2})).toBeInTheDocument()
        expect(screen.getByText('(unknown sender)')).toBeInTheDocument()
    })
})

describe('Reader: toolbar actions', () => {
    it('fires the matching callback for each toolbar button', async () => {
        const user = userEvent.setup()
        const {message, onReply, onReplyAll, onForward, onToggleRead, onDelete} = renderReader()
        await user.click(screen.getByRole('button', {name: 'Reply'}))
        await user.click(screen.getByRole('button', {name: 'Reply all'}))
        await user.click(screen.getByRole('button', {name: 'Forward'}))
        await user.click(screen.getByRole('button', {name: 'Mark as read'}))
        await user.click(screen.getByRole('button', {name: 'Delete'}))
        expect(onReply).toHaveBeenCalledWith(message)
        expect(onReplyAll).toHaveBeenCalledWith(message)
        expect(onForward).toHaveBeenCalledWith(message)
        expect(onToggleRead).toHaveBeenCalledWith(message)
        expect(onDelete).toHaveBeenCalledWith(message)
    })

    it('hides Reply all when the message has no recipients', () => {
        renderReader({message: makeMessage({to: [], cc: []})})
        expect(screen.getByRole('button', {name: 'Reply'})).toBeInTheDocument()
        expect(screen.queryByRole('button', {name: 'Reply all'})).toBeNull()
    })

    it('labels the read toggle by the message read state', () => {
        renderReader({message: makeMessage({read: true})})
        expect(screen.getByRole('button', {name: 'Mark as unread'})).toBeInTheDocument()
        expect(screen.queryByRole('button', {name: 'Mark as read'})).toBeNull()
    })
})

describe('Reader: move and copy', () => {
    const folders = [
        makeFolder({id: 'inbox', name: 'Inbox'}),
        makeFolder({id: 'archive', name: 'Archive'}),
        makeFolder({id: 'work', name: 'Work'}),
    ]

    it('offers move and copy of the current message to other folders', async () => {
        const user = userEvent.setup()
        const {message, onMove, onCopy} = renderReader({folders, canMoveCopy: true})
        await user.selectOptions(screen.getByLabelText('Move to folder'), 'archive')
        await user.selectOptions(screen.getByLabelText('Copy to folder'), 'work')
        expect(onMove).toHaveBeenCalledWith(message, 'archive')
        expect(onCopy).toHaveBeenCalledWith(message, 'work')
    })

    it('omits the current folder from the destinations', () => {
        renderReader({folders, canMoveCopy: true})
        const move = screen.getByLabelText('Move to folder')
        expect(within(move).queryByRole('option', {name: 'Inbox'})).toBeNull()
        expect(within(move).getByRole('option', {name: 'Archive'})).toBeInTheDocument()
    })

    it('hides move and copy when the account cannot move or copy', () => {
        renderReader({folders, canMoveCopy: false})
        expect(screen.queryByLabelText('Move to folder')).toBeNull()
        expect(screen.queryByLabelText('Copy to folder')).toBeNull()
    })

    it('hides move and copy when there is nowhere else to put the message', () => {
        renderReader({folders: [makeFolder({id: 'inbox', name: 'Inbox'})], canMoveCopy: true})
        expect(screen.queryByLabelText('Move to folder')).toBeNull()
    })
})

describe('Reader: outbox message', () => {
    it('shows only Cancel send and fires it', async () => {
        const user = userEvent.setup()
        const {message, onCancelSend} = renderReader({message: makeMessage({folderId: '__outbox__'})})
        expect(screen.queryByRole('button', {name: 'Reply'})).toBeNull()
        expect(screen.queryByRole('button', {name: 'Delete'})).toBeNull()
        await user.click(screen.getByRole('button', {name: 'Cancel send'}))
        expect(onCancelSend).toHaveBeenCalledWith(message)
    })
})

describe('Reader: tag colour menu', () => {
    it('toggles the menu and renders one swatch per palette colour', async () => {
        const user = userEvent.setup()
        renderReader()
        expect(screen.queryByRole('group', {name: 'Tag colour'})).toBeNull()
        const group = await openColourMenu(user)
        expect(within(group).getAllByRole('menuitemcheckbox')).toHaveLength(TAG_PALETTE.length)
    })

    it('applies a colour by firing onToggleTag with the derived id', async () => {
        const user = userEvent.setup()
        const {onToggleTag} = renderReader()
        await openColourMenu(user)
        await user.click(screen.getByTitle('Red'))
        expect(onToggleTag).toHaveBeenCalledWith(colourTagId(RED), true)
    })

    it('marks an assigned colour and toggles it off from the swatch', async () => {
        const user = userEvent.setup()
        const {onToggleTag} = renderReader({messageTags: [makeTag(RED, 'Red')]})
        await openColourMenu(user)
        const red = screen.getByTitle('Red')
        expect(red).toHaveAttribute('aria-checked', 'true')
        await user.click(red)
        expect(onToggleTag).toHaveBeenCalledWith(colourTagId(RED), false)
    })

    it('removes a colour from the reader tag row', async () => {
        const user = userEvent.setup()
        const {onToggleTag} = renderReader({messageTags: [makeTag(RED, 'Red')]})
        await user.click(screen.getByRole('button', {name: 'Remove Red colour'}))
        expect(onToggleTag).toHaveBeenCalledWith(colourTagId(RED), false)
    })

    it('opens by keyboard onto the first swatch and rolls focus with the arrows', async () => {
        const user = userEvent.setup()
        renderReader()
        screen.getByRole('button', {name: /Colour/}).focus()
        await user.keyboard('{ArrowDown}')
        await waitFor(() => expect(screen.getByTitle('Red')).toHaveFocus())
        await user.keyboard('{ArrowRight}')
        expect(screen.getByTitle('Orange')).toHaveFocus()
        await user.keyboard('{ArrowLeft}')
        expect(screen.getByTitle('Red')).toHaveFocus()
        await user.keyboard('{Escape}')
        expect(screen.queryByRole('group', {name: 'Tag colour'})).toBeNull()
        expect(screen.getByRole('button', {name: /Colour/})).toHaveFocus()
    })

    it('wraps focus from the last swatch back to the first', async () => {
        const user = userEvent.setup()
        renderReader()
        screen.getByRole('button', {name: /Colour/}).focus()
        await user.keyboard('{ArrowDown}')
        await waitFor(() => expect(screen.getByTitle('Red')).toHaveFocus())
        await user.keyboard('{ArrowLeft}')
        expect(screen.getByTitle('Slate')).toHaveFocus()
    })

    it('closes the menu on an outside click', async () => {
        const user = userEvent.setup()
        renderReader()
        await openColourMenu(user)
        fireEvent.mouseDown(document.body)
        expect(screen.queryByRole('group', {name: 'Tag colour'})).toBeNull()
    })
})

describe('Reader: attachments', () => {
    const withAttachments = (...attachments: MessageBody['attachments']) =>
        renderReader({body: makeBody({attachments})})

    it('lists an attachment with its name and size', () => {
        withAttachments({index: 0, filename: 'report.pdf', contentType: 'application/pdf', size: 2048})
        expect(screen.getByText('report.pdf')).toBeInTheDocument()
        expect(screen.getByText(formatBytes(2048))).toBeInTheDocument()
        expect(screen.getByText('1 attachment')).toBeInTheDocument()
    })

    it('opens a non-eml attachment through the OS handler', async () => {
        const user = userEvent.setup()
        const {message} = withAttachments({index: 2, filename: 'report.pdf', contentType: '', size: 10})
        await user.click(screen.getByRole('button', {name: 'Open'}))
        expect(apiSpies.openAttachment).toHaveBeenCalledWith(message.id, 2)
        expect(apiSpies.openEmailAttachment).not.toHaveBeenCalled()
    })

    it('opens an eml attachment in the in-app viewer', async () => {
        const user = userEvent.setup()
        const {message} = withAttachments({index: 0, filename: 'nested.eml', contentType: '', size: 10})
        await user.click(screen.getByRole('button', {name: 'Open'}))
        expect(apiSpies.openEmailAttachment).toHaveBeenCalledWith(message.id, 0)
        const dialog = await screen.findByRole('dialog', {name: 'Attached email'})
        expect(within(dialog).getByText('Nested subject')).toBeInTheDocument()
    })

    it('saves one attachment', async () => {
        const user = userEvent.setup()
        const {message} = withAttachments({index: 3, filename: 'a.txt', contentType: '', size: 1})
        await user.click(screen.getByRole('button', {name: 'Save'}))
        expect(apiSpies.saveAttachment).toHaveBeenCalledWith(message.id, 3)
    })

    it('saves all attachments only when there is more than one', async () => {
        const user = userEvent.setup()
        const {message} = withAttachments(
            {index: 0, filename: 'a.txt', contentType: '', size: 1},
            {index: 1, filename: 'b.txt', contentType: '', size: 2},
        )
        expect(screen.getByText('2 attachments')).toBeInTheDocument()
        await user.click(screen.getByRole('button', {name: 'Save all'}))
        expect(apiSpies.saveAllAttachments).toHaveBeenCalledWith(message.id)
    })

    it('shows no Save all for a single attachment', () => {
        withAttachments({index: 0, filename: 'a.txt', contentType: '', size: 1})
        expect(screen.queryByRole('button', {name: 'Save all'})).toBeNull()
    })

    it('surfaces an error raised while saving', async () => {
        const user = userEvent.setup()
        apiSpies.saveAttachment.mockRejectedValueOnce('disk full')
        withAttachments({index: 0, filename: 'a.txt', contentType: '', size: 1})
        await user.click(screen.getByRole('button', {name: 'Save'}))
        expect(await screen.findByText('disk full')).toBeInTheDocument()
    })
})

describe('Reader: message body', () => {
    it('shows a loading placeholder while the body loads', () => {
        renderReader({body: null, bodyLoading: true})
        expect(screen.getByText('Loading message…')).toBeInTheDocument()
    })

    it('blocks remote images until asked, then loads them through the server-side proxy', async () => {
        const user = userEvent.setup()
        apiSpies.loadRemoteImages.mockResolvedValue('<img src="data:image/png;base64,AAAA" alt="pic">shown')
        const rawHtml = '<img data-pp-src="https://x.test/i.png" alt="pic">shown'
        const {container} = renderReader({body: makeBody({html: rawHtml})})
        const srcdoc = () => (container.querySelector('iframe.reader-html-frame') as HTMLIFrameElement).getAttribute('srcdoc') ?? ''
        expect(screen.getByText('Remote images were not loaded to protect your privacy.')).toBeInTheDocument()
        // While blocked the remote source stays parked, the proxy is not called and the CSP forbids remote images.
        expect(srcdoc()).toContain('data-pp-src="https://x.test/i.png"')
        expect(srcdoc()).toContain('img-src data:;')
        expect(apiSpies.loadRemoteImages).not.toHaveBeenCalled()
        await user.click(screen.getByRole('button', {name: 'Load images'}))
        expect(screen.queryByRole('button', {name: 'Load images'})).toBeNull()
        // The proxy resolves the parked body; its inlined-data: result is what the frame then renders.
        expect(apiSpies.loadRemoteImages).toHaveBeenCalledWith(rawHtml)
        await waitFor(() => expect(srcdoc()).toContain('data:image/png;base64,AAAA'))
        expect(srcdoc()).not.toContain('data-pp-src=')
        // The CSP never widens to remote images; it always permits only data:.
        expect(srcdoc()).not.toContain('https:')
    })

    it('loads remote images at once through the proxy when auto-load is on, with no Load images bar', async () => {
        apiSpies.loadRemoteImages.mockResolvedValue('<img src="data:image/png;base64,BBBB" alt="pic">shown')
        const rawHtml = '<img data-pp-src="https://x.test/i.png" alt="pic">shown'
        const {container} = renderReader({autoLoadImages: true, body: makeBody({html: rawHtml})})
        const srcdoc = () => (container.querySelector('iframe.reader-html-frame') as HTMLIFrameElement).getAttribute('srcdoc') ?? ''
        // The images are shown from the start, so there is no blocked-images bar and the proxy resolves at once.
        expect(screen.queryByText('Remote images were not loaded to protect your privacy.')).toBeNull()
        expect(apiSpies.loadRemoteImages).toHaveBeenCalledWith(rawHtml)
        await waitFor(() => expect(srcdoc()).toContain('data:image/png;base64,BBBB'))
        expect(srcdoc()).not.toContain('data-pp-src=')
    })

    it('opens a link in the body through the external browser', () => {
        const {container} = renderReader({body: makeBody({html: '<a href="https://example.com/x">go</a>'})})
        const frame = container.querySelector('iframe.reader-html-frame') as HTMLIFrameElement
        // jsdom does not render the srcdoc, so exercise the frame's delegated link handler on its same-origin
        // document by injecting a link and dispatching a bubbling click, as a real in-email click would.
        const cdoc = frame.contentDocument as Document
        const anchor = cdoc.createElement('a')
        anchor.setAttribute('href', 'https://example.com/x')
        cdoc.body.appendChild(anchor)
        anchor.dispatchEvent(new Event('click', {bubbles: true, cancelable: true}))
        expect(apiSpies.openExternal).toHaveBeenCalledWith('https://example.com/x')
    })

    it('renders plain text when there is no html', () => {
        renderReader({body: makeBody({html: '', plain: 'just text'})})
        expect(screen.getByText('just text')).toBeInTheDocument()
    })

    it('renders the email dark, inverted to match the app, when the dark theme is on', () => {
        const {container} = renderReader({dark: true, body: makeBody({html: '<p>Message body</p>'})})
        const srcdoc = (container.querySelector('iframe.reader-html-frame') as HTMLIFrameElement).getAttribute('srcdoc') ?? ''
        expect(srcdoc).toContain('html{filter:invert(1) hue-rotate(180deg);}')
    })
})

describe('Reader: tabs and back', () => {
    it('renders open tabs and wires select and close', async () => {
        const user = userEvent.setup()
        const other = makeMessage({id: 'm2', subject: 'Second'})
        const {message, onSelectTab, onCloseTab} = renderReader({tabs: [makeMessage(), other]})
        expect(screen.getAllByRole('tab')).toHaveLength(2)
        await user.click(screen.getByText('Second'))
        expect(onSelectTab).toHaveBeenCalledWith(other)
        await user.click(screen.getByRole('button', {name: 'Close Hello there'}))
        expect(onCloseTab).toHaveBeenCalledWith(message.id)
    })

    it('shows a Back button in full-width mode and fires onBack', async () => {
        const user = userEvent.setup()
        const onBack = vi.fn()
        renderReader({onBack})
        await user.click(screen.getByRole('button', {name: /Back/}))
        expect(onBack).toHaveBeenCalled()
    })
})

describe('Reader: reset on message change', () => {
    it('closes the colour menu when the message changes', async () => {
        const user = userEvent.setup()
        const handlers = {
            onToggleRead: vi.fn(), onReply: vi.fn(), onReplyAll: vi.fn(), onForward: vi.fn(),
            onDelete: vi.fn(), onCancelSend: vi.fn(), onMove: vi.fn(), onCopy: vi.fn(),
            onToggleTag: vi.fn(), onSelectTab: vi.fn(), onCloseTab: vi.fn(),
        }
        const base: ReaderProps = {
            message: makeMessage({id: 'first'}), folders: [], canMoveCopy: false, autoLoadImages: false,
            dark: false, tags: [], messageTags: [], body: makeBody(), bodyLoading: false, tabs: [], ...handlers,
        }
        const {rerender} = render(<Reader {...base}/>)
        await openColourMenu(user)
        expect(screen.getByRole('group', {name: 'Tag colour'})).toBeInTheDocument()
        rerender(<Reader {...base} message={makeMessage({id: 'second'})}/>)
        expect(screen.queryByRole('group', {name: 'Tag colour'})).toBeNull()
    })
})
