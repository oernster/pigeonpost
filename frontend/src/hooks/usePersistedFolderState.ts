import {useEffect, useRef, useState} from 'react'
import {api} from '../api'

// collapseKey and folderOrderKey name the per-account localStorage entries: the collapsed folder paths and
// the custom folders' local display order (a list of folder paths). IMAP has no folder order of its own, so
// a same-level reorder is a purely local, persisted display concern.
function collapseKey(accountId: string): string {
    return `pigeonpost.collapsed.${accountId}`
}

function folderOrderKey(accountId: string): string {
    return `pigeonpost.folderorder.${accountId}`
}

// readCache and writeCache wrap the localStorage copy of the state. A storage failure is swallowed:
// the cache is just cold and the backend copy still persists.
function readCache(key: string): string[] {
    try {
        const raw = localStorage.getItem(key)
        return raw ? (JSON.parse(raw) as string[]) : []
    } catch {
        return []
    }
}

function writeCache(key: string, value: string[]): void {
    try {
        localStorage.setItem(key, JSON.stringify(value))
    } catch {
        // A storage failure just means the cache is not warm; the backend copy still persists.
    }
}

// loadBackend and saveBackend route the backend calls through async wrappers, so a missing Wails
// runtime (jsdom tests) or a backend failure surfaces as a rejection the callers swallow, never a
// synchronous throw out of an effect.
async function loadBackend(accountId: string): Promise<{order?: string[]; collapsed?: string[]}> {
    return api.folderUIState(accountId)
}

async function saveBackend(accountId: string, order: string[], collapsed: string[]): Promise<void> {
    return api.saveFolderUIState(accountId, order, collapsed)
}

// usePersistedFolderState owns the folder tree's per-account display state: the set of collapsed folder
// paths and the custom folders' local order. The durable home is the backend database (which survives an
// application update); localStorage is a warm cache of the same state, read synchronously so the tree
// renders its remembered shape on the first paint. When the cache is empty but the backend has state
// (the first launch after an update wiped the WebView profile) the backend copy restores it; when the
// backend is empty but the cache has state (the first launch after this feature shipped) the cached copy
// migrates up. Every change writes both. A failure on either side is swallowed: the state is just
// remembered by the surviving copy and the UI still works.
export function usePersistedFolderState(accountId: string) {
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
    const [order, setOrder] = useState<string[]>([])
    // The latest values, kept in refs so a change to one half can save the full state without reading
    // the other half through a stale closure (and so rapid successive changes compose before React
    // re-renders).
    const orderRef = useRef<string[]>(order)
    const collapsedRef = useRef<Set<string>>(collapsed)

    useEffect(() => {
        const cachedOrder = readCache(folderOrderKey(accountId))
        const cachedCollapsed = readCache(collapseKey(accountId))
        orderRef.current = cachedOrder
        collapsedRef.current = new Set(cachedCollapsed)
        setOrder(orderRef.current)
        setCollapsed(collapsedRef.current)
        let stale = false
        loadBackend(accountId)
            .then((state) => {
                if (stale) {
                    return
                }
                const backendOrder = state.order ?? []
                const backendCollapsed = state.collapsed ?? []
                if (backendOrder.length > 0 || backendCollapsed.length > 0) {
                    // The backend copy is the durable one: it wins and re-warms the cache.
                    orderRef.current = backendOrder
                    collapsedRef.current = new Set(backendCollapsed)
                    setOrder(backendOrder)
                    setCollapsed(collapsedRef.current)
                    writeCache(folderOrderKey(accountId), backendOrder)
                    writeCache(collapseKey(accountId), backendCollapsed)
                } else if (cachedOrder.length > 0 || cachedCollapsed.length > 0) {
                    // One-time migration: state saved by a version that kept it only in localStorage is
                    // pushed up so the next application update cannot lose it.
                    void saveBackend(accountId, cachedOrder, cachedCollapsed).catch(() => undefined)
                }
            })
            .catch(() => {
                // Backend unavailable: the cached copy already applied remains in force.
            })
        return () => {
            stale = true
        }
    }, [accountId])

    // persist writes the full state to both homes. The backend write is fire-and-forget: a failure just
    // means the change is remembered by the cache alone until the next successful save.
    const persist = (nextOrder: string[], nextCollapsed: Set<string>) => {
        writeCache(folderOrderKey(accountId), nextOrder)
        writeCache(collapseKey(accountId), [...nextCollapsed])
        void saveBackend(accountId, nextOrder, [...nextCollapsed]).catch(() => undefined)
    }

    const toggle = (path: string) => {
        const next = new Set(collapsedRef.current)
        if (next.has(path)) {
            next.delete(path)
        } else {
            next.add(path)
        }
        collapsedRef.current = next
        setCollapsed(next)
        persist(orderRef.current, next)
    }

    const persistOrder = (next: string[]) => {
        orderRef.current = next
        setOrder(next)
        persist(next, collapsedRef.current)
    }

    // expand removes a folder path from the collapsed set (idempotent: expanding an already-open folder is a
    // no-op). It backs the drag-to-open spring-loading, which must open a folder without ever collapsing one.
    const expand = (path: string) => {
        if (!collapsedRef.current.has(path)) {
            return
        }
        const next = new Set(collapsedRef.current)
        next.delete(path)
        collapsedRef.current = next
        setCollapsed(next)
        persist(orderRef.current, next)
    }

    return {collapsed, order, toggle, persistOrder, expand}
}
