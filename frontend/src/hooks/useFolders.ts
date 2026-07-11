import {Dispatch, MutableRefObject, SetStateAction, useCallback, useEffect, useRef, useState} from 'react'
import {Folder, api} from '../api'
import type {MessageStore} from './useMessageStore'

// FolderPrompt is the create-or-rename dialog state: which mode, and the folder being renamed.
export interface FolderPrompt {
    mode: 'create' | 'rename'
    folder?: Folder
}

// FoldersDeps is what the folder list needs from the rest of App: the selected account (whose folders are
// listed and under which a new folder is created), the message store (a folder delete clears the open
// folder's messages) and the error sink.
export interface FoldersDeps {
    selectedAccount: string
    store: MessageStore
    setError: (message: string) => void
}

export interface Folders {
    folders: Folder[]
    setFolders: Dispatch<SetStateAction<Folder[]>>
    selectedFolder: string
    setSelectedFolder: Dispatch<SetStateAction<string>>
    // selectedFolderRef mirrors selectedFolder for the async loaders, so a background refresh only replaces
    // the list when the user is still on the folder it was started for.
    selectedFolderRef: MutableRefObject<string>
    folderPrompt: FolderPrompt | null
    setFolderPrompt: Dispatch<SetStateAction<FolderPrompt | null>>
    folderToDelete: Folder | null
    setFolderToDelete: Dispatch<SetStateAction<Folder | null>>
    folderBusy: boolean
    refreshFolders: () => Promise<void>
    submitFolderPrompt: (value: string) => Promise<void>
    confirmDeleteFolder: () => Promise<void>
    reparentFolder: (folderId: string, newParentId: string) => Promise<void>
}

// useFolders owns the account's folder list, the selected folder and the folder create/rename/delete/reparent
// flow. loadFolderMessages and selectFolder stay in App: they coordinate folder navigation with the outbox
// view (selectFolder's synthetic-Outbox branch reads the queue), which would otherwise couple this hook to
// useOutbox in a cycle, since useOutbox already reads the folder list to build its synthetic Outbox folder.
export function useFolders(deps: FoldersDeps): Folders {
    const {selectedAccount, store, setError} = deps
    const {setMessages, setSelectedMessage} = store

    const [folders, setFolders] = useState<Folder[]>([])
    const [selectedFolder, setSelectedFolder] = useState<string>('')
    const [folderPrompt, setFolderPrompt] = useState<FolderPrompt | null>(null)
    const [folderToDelete, setFolderToDelete] = useState<Folder | null>(null)
    const [folderBusy, setFolderBusy] = useState<boolean>(false)
    // Tracks the folder currently on screen, so a background refresh only replaces the list when the
    // user has not navigated away since it started.
    const selectedFolderRef = useRef<string>('')

    useEffect(() => {
        selectedFolderRef.current = selectedFolder
    }, [selectedFolder])

    const refreshFolders = useCallback(async () => {
        if (selectedAccount) {
            setFolders(await api.listFolders(selectedAccount))
        }
    }, [selectedAccount])

    // submitFolderPrompt handles both create and rename from the shared PromptDialog.
    const submitFolderPrompt = useCallback(async (value: string) => {
        if (!folderPrompt) {
            return
        }
        setFolderBusy(true)
        setError('')
        try {
            if (folderPrompt.mode === 'create') {
                await api.createFolder(selectedAccount, value)
            } else if (folderPrompt.folder) {
                await api.renameFolder(folderPrompt.folder.id, value)
            }
            await refreshFolders()
            setFolderPrompt(null)
        } catch (e) {
            setError(String(e))
        } finally {
            setFolderBusy(false)
        }
    }, [folderPrompt, selectedAccount, refreshFolders])

    const confirmDeleteFolder = useCallback(async () => {
        if (!folderToDelete) {
            return
        }
        setFolderBusy(true)
        setError('')
        try {
            await api.deleteFolder(folderToDelete.id)
            if (folderToDelete.id === selectedFolder) {
                setSelectedFolder('')
                setMessages([])
                setSelectedMessage(null)
            }
            await refreshFolders()
            setFolderToDelete(null)
        } catch (e) {
            setError(String(e))
        } finally {
            setFolderBusy(false)
        }
    }, [folderToDelete, selectedFolder, refreshFolders])

    // reparentFolder moves a folder under newParentId (empty for the top level) on the server, then
    // refreshes the folder list. Like a rename the folder's server path changes while the open folder and
    // its messages are left as they are. It backs the folder drag-and-drop; a same-level reorder is a local
    // display concern handled in the sidebar and never reaches here.
    const reparentFolder = useCallback(async (folderId: string, newParentId: string) => {
        setFolderBusy(true)
        setError('')
        try {
            await api.moveFolder(folderId, newParentId)
            await refreshFolders()
        } catch (e) {
            setError(String(e))
        } finally {
            setFolderBusy(false)
        }
    }, [refreshFolders])

    return {
        folders, setFolders, selectedFolder, setSelectedFolder, selectedFolderRef,
        folderPrompt, setFolderPrompt, folderToDelete, setFolderToDelete, folderBusy,
        refreshFolders, submitFolderPrompt, confirmDeleteFolder, reparentFolder,
    }
}
