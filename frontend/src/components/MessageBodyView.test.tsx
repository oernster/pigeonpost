// The one home for rendering a message's content, shared by the reader and by each expanded message of
// the thread view. It renders from its props alone, so the tests drive it directly; ../api is mocked
// because the remote-image resolve and the external-link open both go through it.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, render, screen, waitFor} from '@testing-library/react'
import {MessageBodyView} from './MessageBodyView'
import type {MessageBody} from '../api'

const apiSpies = vi.hoisted(() => ({loadRemoteImages: vi.fn(), openExternal: vi.fn(), messageInvite: vi.fn()}))
vi.mock('../api', () => ({api: apiSpies}))

function body(overrides: Partial<MessageBody> = {}): MessageBody {
    return {plain: '', html: '', hasInvite: false, attachments: [], ...overrides} as MessageBody
}

function renderBody(overrides: Record<string, unknown> = {}) {
    return render(
        <MessageBodyView
            messageId="m1"
            body={body()}
            loading={false}
            snippet=""
            autoLoadImages={false}
            dark={false}
            {...overrides}
        />,
    )
}

beforeEach(() => {
    apiSpies.loadRemoteImages.mockReset().mockResolvedValue('<p>with images</p>')
    apiSpies.openExternal.mockReset()
})

afterEach(() => cleanup())

describe('MessageBodyView', () => {
    it('says it is loading before the body arrives', () => {
        renderBody({body: null, loading: true})
        expect(screen.getByText(/Loading message/)).toBeTruthy()
    })

    it('prefers the html body when the message has one', () => {
        const {container} = renderBody({body: body({html: '<p>the html</p>', plain: 'the plain text'})})
        expect(container.querySelector('iframe')).not.toBeNull()
        expect(container.querySelector('.reader-text')).toBeNull()
    })

    it('falls back to the plain text when there is no html', () => {
        const {container} = renderBody({body: body({plain: 'the plain text'})})
        expect(container.querySelector('.reader-text')?.textContent).toContain('the plain text')
    })

    it('falls back to the stored snippet when the message has no body text', () => {
        renderBody({body: body(), snippet: 'the snippet'})
        expect(screen.getByText('the snippet')).toBeTruthy()
    })

    it('says plainly that a message has nothing to show', () => {
        renderBody({body: body(), snippet: ''})
        expect(screen.getByText('This message has no text content.')).toBeTruthy()
    })

    it('holds remote images behind the privacy bar until asked', () => {
        renderBody({body: body({html: '<img data-pp-src="http://x/y.png">'})})
        expect(screen.getByText(/Remote images were not loaded/)).toBeTruthy()
        expect(apiSpies.loadRemoteImages).not.toHaveBeenCalled()
    })

    it('loads remote images at once when the setting says so', async () => {
        renderBody({body: body({html: '<img data-pp-src="http://x/y.png">'}), autoLoadImages: true})
        await waitFor(() => expect(apiSpies.loadRemoteImages).toHaveBeenCalled())
        expect(screen.queryByText(/Remote images were not loaded/)).toBeNull()
    })
})
