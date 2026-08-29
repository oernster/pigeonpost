import {useCallback, useState} from 'react'

// useLoadedPanel is the shape shared by the Help menu's two read-only panels, About and the licence
// text: nothing is fetched until the menu item is chosen, the answer is held while its dialog is open
// and dropped when it closes, so the dialog's own presence is what the held value means.
//
// It is deliberately not useManagedCollection. That one loads on mount and reloads after its dialog
// edits it; this one loads on demand and is never written back. The two look alike from a distance and
// would have to grow apart the moment either changed, so they stay separate.
export interface LoadedPanel<T> {
    // value is the loaded content, null while the panel is closed or its load failed.
    value: T | null
    // open fetches the content and shows the panel, reporting a failure through setError instead.
    open: () => Promise<void>
    close: () => void
}

export function useLoadedPanel<T>(
    load: () => Promise<T>,
    setError: (message: string) => void,
): LoadedPanel<T> {
    const [value, setValue] = useState<T | null>(null)

    const open = useCallback(async () => {
        try {
            setValue(await load())
        } catch (e) {
            setError(String(e))
        }
    }, [load, setError])

    const close = useCallback(() => setValue(null), [])

    return {value, open, close}
}
