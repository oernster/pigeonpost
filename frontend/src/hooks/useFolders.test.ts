// The folder create/rename flow behind the shared prompt dialog. A create with a parent folder is a
// subfolder of it (the backend joins the path with the server's delimiter); a create with no parent
// lands at the top level of the selected account. ../api is mocked (the Wails seam) and the real
// message store is wired in the way App does.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Folder} from '../api'
import {useMessageStore} from './useMessageStore'
import {useFolders} from './useFolders'

const apiSpies = vi.hoisted(() => ({
    listFolders: vi.fn(),
    createFolder: vi.fn(),
    createSubfolder: vi.fn(),
    renameFolder: vi.fn(),
}))

vi.mock('../api', () => ({api: apiSpies}))

const PARENT = {
    id: 'f1', accountId: 'a1', path: 'Projects', name: 'Projects', kind: 'custom', unread: 0, total: 0,
} as Folder

const errors: string[] = []

function harness() {
    return renderHook(() => {
        const store = useMessageStore()
        return useFolders({selectedAccount: 'a1', store, setError: (m: string) => errors.push(m)})
    })
}

beforeEach(() => {
    errors.length = 0
    apiSpies.listFolders.mockReset().mockResolvedValue([])
    apiSpies.createFolder.mockReset().mockResolvedValue(undefined)
    apiSpies.createSubfolder.mockReset().mockResolvedValue(undefined)
    apiSpies.renameFolder.mockReset().mockResolvedValue(undefined)
})

afterEach(() => cleanup())

describe('useFolders: the folder prompt', () => {
    it('creates a top-level folder on the selected account when no parent is named', async () => {
        const {result} = harness()
        act(() => result.current.setFolderPrompt({mode: 'create'}))
        await act(async () => {
            await result.current.submitFolderPrompt('Reports')
        })
        expect(apiSpies.createFolder).toHaveBeenCalledWith('a1', 'Reports')
        expect(apiSpies.createSubfolder).not.toHaveBeenCalled()
        expect(result.current.folderPrompt).toBeNull()
    })

    it('creates a subfolder under the named parent', async () => {
        const {result} = harness()
        act(() => result.current.setFolderPrompt({mode: 'create', parent: PARENT}))
        await act(async () => {
            await result.current.submitFolderPrompt('Reports')
        })
        expect(apiSpies.createSubfolder).toHaveBeenCalledWith('f1', 'Reports')
        expect(apiSpies.createFolder).not.toHaveBeenCalled()
    })

    it('renames the prompt folder', async () => {
        const {result} = harness()
        act(() => result.current.setFolderPrompt({mode: 'rename', folder: PARENT}))
        await act(async () => {
            await result.current.submitFolderPrompt('Archive')
        })
        expect(apiSpies.renameFolder).toHaveBeenCalledWith('f1', 'Archive')
    })

    it('reports a failed create and leaves the prompt open', async () => {
        apiSpies.createSubfolder.mockRejectedValue(new Error('server said no'))
        const {result} = harness()
        act(() => result.current.setFolderPrompt({mode: 'create', parent: PARENT}))
        await act(async () => {
            await result.current.submitFolderPrompt('Reports')
        })
        expect(errors.join()).toContain('server said no')
        expect(result.current.folderPrompt).not.toBeNull()
    })

    it('does nothing when there is no prompt open', async () => {
        const {result} = harness()
        await act(async () => {
            await result.current.submitFolderPrompt('Reports')
        })
        expect(apiSpies.createFolder).not.toHaveBeenCalled()
    })
})
