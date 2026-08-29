import {useCallback, useEffect, useRef, useState} from 'react'
import {Message, api} from '../api'
import type {SearchScope} from '../components/MessageList'
import {isUnifiedFolder} from '../unified'
import {isSnoozedFolder} from '../snooze'

// useSearch owns the local full-text search over the cached mail: the query, the scope it runs in, the
// per-hit matched-text snippets and whether the last query degraded to plain text because its operators
// could not be parsed. Results replace the folder listing while a query is active, so the results
// themselves are written back into the shared message store rather than held here.
//
// SEARCH_DEBOUNCE_MS is how long typing settles before a query runs. Long enough that a typed word does
// not issue a query per keystroke; short enough that the results feel live.
const SEARCH_DEBOUNCE_MS = 250

export interface SearchOptions {
    // The anchors the narrower scopes run against. A scope whose anchor disappears falls back to all mail.
    selectedFolder: string
    selectedAccount: string
    // setResults writes the hits into the shared message store, where the list reads them.
    setResults: (messages: Message[]) => void
    setError: (message: string) => void
}

export function useSearch({selectedFolder, selectedAccount, setResults, setError}: SearchOptions) {
    const [query, setQuery] = useState<string>('')
    const [scope, setScope] = useState<SearchScope>('all')
    const [snippets, setSnippets] = useState<Map<string, string>>(new Map())
    const [degraded, setDegraded] = useState<boolean>(false)
    // inputRef lets Edit > Search (Ctrl+K) focus the search box from anywhere.
    const inputRef = useRef<HTMLInputElement>(null)
    const focusSearch = useCallback(() => inputRef.current?.focus(), [])
    const active = query.trim() !== ''

    // A scope whose anchor disappears (the folder deselected or synthetic, the account removed) falls
    // back to all mail visibly, so the selector never claims a narrower scope than the search actually
    // runs with. The unified mailbox is not a real folder, so it cannot anchor a folder scope.
    useEffect(() => {
        if ((scope === 'folder' && (!selectedFolder || isUnifiedFolder(selectedFolder) || isSnoozedFolder(selectedFolder)))
            || (scope === 'account' && !selectedAccount)) {
            setScope('all')
        }
    }, [scope, selectedFolder, selectedAccount])

    // Debounced full-text search: results replace the folder listing while a query is active. The scope
    // selector narrows it to the selected folder or account; changing either re-runs the live query. The
    // stale flag discards a slow response that lands after the query has changed, so an older search can
    // never overwrite a newer one's results.
    useEffect(() => {
        const q = query.trim()
        if (q === '') {
            setResults([])
            setSnippets(new Map())
            setDegraded(false)
            return
        }
        let stale = false
        const folderId = scope === 'folder' ? selectedFolder : ''
        const accountId = scope === 'account' ? selectedAccount : ''
        const handle = window.setTimeout(() => {
            void api.searchMessages(q, folderId, accountId).then((result) => {
                if (stale) {
                    return
                }
                setResults(result.hits.map((hit) => hit.message))
                setSnippets(new Map(result.hits.map((hit) => [hit.message.id, hit.snippet])))
                setDegraded(result.degraded)
            }).catch((e) => setError(String(e)))
        }, SEARCH_DEBOUNCE_MS)
        return () => {
            stale = true
            window.clearTimeout(handle)
        }
        // setResults and setError are stable for the life of the app; listing them would re-run the
        // query on every render that rebuilds them.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [query, scope, selectedFolder, selectedAccount])

    return {query, setQuery, scope, setScope, snippets, degraded, active, inputRef, focusSearch}
}
