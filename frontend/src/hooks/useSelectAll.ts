import {Dispatch, MutableRefObject, SetStateAction, useCallback} from 'react'
import {Message} from '../api'
import {sortByDate} from '../threads'
import type {FolderPagination} from './useFolderPagination'

// SelectAllDeps is what select all reads: the view on screen, the folder it shows and how that folder
// is paged, plus the setters it marks the selection and fills the list with.
export interface SelectAllDeps {
    visibleList: Message[]
    searchActive: boolean
    sortAscending: boolean
    pagination: FolderPagination
    selectedFolderRef: MutableRefObject<string>
    setMessages: Dispatch<SetStateAction<Message[]>>
    setMarkedIds: Dispatch<SetStateAction<Set<string>>>
    setAnchorId: Dispatch<SetStateAction<string | null>>
    setError: Dispatch<SetStateAction<string>>
}

// useSelectAll is the one select all, behind both Ctrl+A on the list and Edit > Select all. It marks
// every message in the view. The flat folder view loads a page at a time as it scrolls, so marking only
// the rows loaded so far would select part of the folder, however much had been scrolled into view;
// when pages remain it loads the whole folder first. A search already holds its whole result set. A
// folder switch while the folder loads abandons the selection rather than marking the wrong folder.
export function useSelectAll(deps: SelectAllDeps): () => Promise<void> {
    const {
        visibleList, searchActive, sortAscending, pagination, selectedFolderRef,
        setMessages, setMarkedIds, setAnchorId, setError,
    } = deps
    return useCallback(async () => {
        const folderId = selectedFolderRef.current
        let rows = visibleList
        if (!searchActive && folderId && pagination.hasMore()) {
            try {
                const all = await pagination.loadAll(folderId)
                if (selectedFolderRef.current !== folderId) {
                    return
                }
                setMessages(all)
                rows = sortByDate(all, sortAscending)
            } catch (e) {
                setError(String(e))
                return
            }
        }
        if (rows.length === 0) {
            return
        }
        setMarkedIds(new Set(rows.map((m) => m.id)))
        setAnchorId(rows[0].id)
    }, [visibleList, searchActive, sortAscending, pagination, selectedFolderRef, setMessages, setMarkedIds, setAnchorId, setError])
}
