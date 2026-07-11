import {useEffect, useState} from 'react'

// collapseKey and folderOrderKey name the per-account localStorage entries: the collapsed folder paths and
// the custom folders' local display order (a list of folder paths). IMAP has no folder order of its own, so
// a same-level reorder is a purely local, persisted display concern.
function collapseKey(accountId: string): string {
    return `pigeonpost.collapsed.${accountId}`
}

function folderOrderKey(accountId: string): string {
    return `pigeonpost.folderorder.${accountId}`
}

// usePersistedFolderState owns the folder tree's per-account, localStorage-backed display state: the set of
// collapsed folder paths and the custom folders' local order. Both are loaded when the account changes and
// written back on every change, so they survive restarts. A storage failure is swallowed: the state is just
// not remembered and the UI still works.
export function usePersistedFolderState(accountId: string) {
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
    const [order, setOrder] = useState<string[]>([])

    useEffect(() => {
        try {
            const raw = localStorage.getItem(collapseKey(accountId))
            setCollapsed(new Set(raw ? (JSON.parse(raw) as string[]) : []))
        } catch {
            setCollapsed(new Set())
        }
    }, [accountId])

    useEffect(() => {
        try {
            const raw = localStorage.getItem(folderOrderKey(accountId))
            setOrder(raw ? (JSON.parse(raw) as string[]) : [])
        } catch {
            setOrder([])
        }
    }, [accountId])

    const toggle = (path: string) => {
        setCollapsed((prev) => {
            const next = new Set(prev)
            if (next.has(path)) {
                next.delete(path)
            } else {
                next.add(path)
            }
            try {
                localStorage.setItem(collapseKey(accountId), JSON.stringify([...next]))
            } catch {
                // A storage failure just means the state is not remembered; the UI still works.
            }
            return next
        })
    }

    const persistOrder = (next: string[]) => {
        setOrder(next)
        try {
            localStorage.setItem(folderOrderKey(accountId), JSON.stringify(next))
        } catch {
            // A storage failure just means the order is not remembered; the UI still works.
        }
    }

    return {collapsed, order, toggle, persistOrder}
}
