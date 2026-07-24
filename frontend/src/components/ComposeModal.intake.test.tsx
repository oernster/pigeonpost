// Paste and drop intake for the compose window: images embed in the body, every other file becomes an
// in-memory attachment sent as attachmentData. The editor is stubbed (ProseMirror does not run in
// jsdom); the intake handlers are driven through the editorProps the stub captures, exactly as
// ProseMirror would call them.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {ComposeModal} from './ComposeModal'

const apiSpies = vi.hoisted(() => ({
    send: vi.fn(),
    saveDraft: vi.fn(),
    clearDraftRecovery: vi.fn(),
    saveDraftRecovery: vi.fn(),
    pickAttachments: vi.fn(),
    listContacts: vi.fn(),
    collectContacts: vi.fn(),
}))

vi.mock('../api', () => ({
    api: {
        send: apiSpies.send,
        saveDraft: apiSpies.saveDraft,
        clearDraftRecovery: apiSpies.clearDraftRecovery,
        saveDraftRecovery: apiSpies.saveDraftRecovery,
        pickAttachments: apiSpies.pickAttachments,
        listContacts: apiSpies.listContacts,
        collectContacts: apiSpies.collectContacts,
    },
}))

// The editor stub captures the useEditor options (to drive handlePaste / handleDrop) and records
// setImage calls (the embed path).
const editorSpies = vi.hoisted(() => ({
    // options is the whole useEditor argument; the tests reach into editorProps, so it stays untyped.
    options: undefined as any,
    setImageCalls: [] as {src: string}[],
}))

vi.mock('@tiptap/react', () => {
    const chain = () => {
        const c: Record<string, (...args: unknown[]) => unknown> = {}
        for (const m of ['focus', 'run']) {
            c[m] = () => c
        }
        c.setImage = (arg: unknown) => {
            editorSpies.setImageCalls.push(arg as {src: string})
            return c
        }
        return c
    }
    const editor = {
        isActive: () => false,
        getText: () => 'hello',
        getHTML: () => '<p>hello</p>',
        getAttributes: () => ({}),
        chain,
    }
    return {
        useEditor: (options: any) => {
            editorSpies.options = options
            return editor
        },
        EditorContent: () => null,
    }
})

const TO_PLACEHOLDER = 'name@example.com, other@example.com'

function renderCompose() {
    render(<ComposeModal
        accountId="acc1"
        senders={[{name: 'Me', address: 'me@x.com'}]}
        canSaveDraft={true}
        holdSeconds={0}
        onHeld={vi.fn()}
        onClose={vi.fn()}
        onMarkReplied={vi.fn()}
        onMarkForwarded={vi.fn()}
    />)
}

// paste wraps a file list the way ProseMirror hands a paste event to handlePaste.
function paste(files: File[]): boolean {
    return editorSpies.options.editorProps.handlePaste(null, {clipboardData: {files}})
}

beforeEach(() => {
    apiSpies.send.mockReset().mockResolvedValue('')
    apiSpies.saveDraft.mockReset().mockResolvedValue(undefined)
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.saveDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.pickAttachments.mockReset().mockResolvedValue([])
    apiSpies.listContacts.mockReset().mockResolvedValue([])
    apiSpies.collectContacts.mockReset().mockResolvedValue(0)
    editorSpies.setImageCalls = []
})

afterEach(() => cleanup())

describe('ComposeModal: paste and drop intake', () => {
    it('attaches a pasted non-image file and sends it as attachmentData', async () => {
        renderCompose()
        const pdf = new File([new Uint8Array([37, 80, 68, 70])], 'report.pdf', {type: 'application/pdf'})
        let handled = false
        await act(async () => {
            handled = paste([pdf])
        })
        expect(handled).toBe(true)
        expect(await screen.findByTitle('report.pdf')).toBeInTheDocument()

        fireEvent.change(screen.getByPlaceholderText(TO_PLACEHOLDER), {target: {value: 'a@b.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(apiSpies.send).toHaveBeenCalled())
        const req = apiSpies.send.mock.calls[0][0]
        expect(req.attachmentData).toHaveLength(1)
        expect(req.attachmentData[0].name).toBe('report.pdf')
        expect(req.attachmentData[0].contentType).toBe('application/pdf')
        expect(atob(req.attachmentData[0].content)).toBe('%PDF')
    })

    it('embeds a pasted image at the cursor with its own bytes', async () => {
        renderCompose()
        const png = new File([new Uint8Array([137, 80, 78, 71])], 'shot.png', {type: 'image/png'})
        await act(async () => {
            expect(paste([png])).toBe(true)
        })
        await waitFor(() => expect(editorSpies.setImageCalls).toHaveLength(1))
        expect(editorSpies.setImageCalls[0].src.startsWith('data:image/png;base64,')).toBe(true)
        // An embedded image is body content, not an attachment chip.
        expect(screen.queryByTitle('shot.png')).toBeNull()
    })

    it('splits a mixed paste: the image embeds, the file attaches', async () => {
        renderCompose()
        const png = new File([new Uint8Array([1])], 'shot.png', {type: 'image/png'})
        const pdf = new File([new Uint8Array([2])], 'report.pdf', {type: 'application/pdf'})
        await act(async () => {
            paste([png, pdf])
        })
        expect(await screen.findByTitle('report.pdf')).toBeInTheDocument()
        await waitFor(() => expect(editorSpies.setImageCalls).toHaveLength(1))
    })

    it('lets a file-free paste fall through to the editor', () => {
        renderCompose()
        expect(paste([])).toBe(false)
        expect(editorSpies.setImageCalls).toHaveLength(0)
    })

    it('refuses a paste that would break the attachment size cap', async () => {
        renderCompose()
        // Size is read before any bytes are, so a plain object stands in for an oversized File.
        const huge = {name: 'big.iso', type: 'application/octet-stream', size: 26 * 1024 * 1024} as File
        await act(async () => {
            expect(paste([huge])).toBe(true)
        })
        expect(screen.getByText(/25 MB attachment limit/)).toBeInTheDocument()
        expect(screen.queryByTitle('big.iso')).toBeNull()
    })

    it('removes a pasted attachment from its chip', async () => {
        renderCompose()
        const pdf = new File([new Uint8Array([2])], 'report.pdf', {type: 'application/pdf'})
        await act(async () => {
            paste([pdf])
        })
        expect(await screen.findByTitle('report.pdf')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Remove report.pdf'}))
        expect(screen.queryByTitle('report.pdf')).toBeNull()
    })

    it('embeds an image arriving only through clipboard items (WebKit paste shape)', async () => {
        renderCompose()
        const png = new File([new Uint8Array([137, 80])], 'shot.png', {type: 'image/png'})
        await act(async () => {
            const handled = editorSpies.options.editorProps.handlePaste(null, {
                clipboardData: {files: [], items: [{kind: 'file', getAsFile: () => png}]},
            })
            expect(handled).toBe(true)
        })
        await waitFor(() => expect(editorSpies.setImageCalls).toHaveLength(1))
    })

    it('attaches by path when the paste carries file URIs instead of File objects', async () => {
        renderCompose()
        await act(async () => {
            const handled = editorSpies.options.editorProps.handlePaste(null, {
                clipboardData: {
                    files: [],
                    getData: (format: string) =>
                        format === 'text/uri-list' ? 'file:///Users/oliver/My%20Report.pdf' : '',
                },
            })
            expect(handled).toBe(true)
        })
        expect(await screen.findByTitle('/Users/oliver/My Report.pdf')).toBeInTheDocument()

        fireEvent.change(screen.getByPlaceholderText(TO_PLACEHOLDER), {target: {value: 'a@b.com'}})
        fireEvent.click(screen.getByRole('button', {name: 'Send'}))
        await waitFor(() => expect(apiSpies.send).toHaveBeenCalled())
        expect(apiSpies.send.mock.calls[0][0].attachmentPaths).toEqual(['/Users/oliver/My Report.pdf'])
    })

    it('takes a drop on the modal outside the editor', async () => {
        renderCompose()
        const pdf = new File([new Uint8Array([3])], 'dropped.pdf', {type: 'application/pdf'})
        const dialog = screen.getByRole('dialog', {name: 'New message'})
        // fireEvent.drop drops dataTransfer unless it is an own property of the event (see
        // Sidebar.test.tsx for the same jsdom gotcha).
        const event = new Event('drop', {bubbles: true, cancelable: true})
        Object.defineProperty(event, 'dataTransfer', {value: {files: [pdf]}})
        await act(async () => {
            dialog.dispatchEvent(event)
        })
        expect(await screen.findByTitle('dropped.pdf')).toBeInTheDocument()
    })
})
