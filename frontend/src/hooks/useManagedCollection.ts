import {useCallback, useEffect, useState, type Dispatch, type SetStateAction} from 'react'

// useManagedCollection is the shape shared by every backend-backed list the menus manage: rules,
// message templates, contacts and calendar events. Each is loaded once on mount, reloaded after its
// manager dialog changes it, each with a flag saying whether that dialog is open. The four were
// four hand-written copies of these fifteen lines, which is three chances for one of them to drift on
// the day its error handling or its reload timing needs to change.
//
// load must be stable across renders, since the mount effect depends on it. Every call site passes an
// `api` method, which is a property of a module-level object and so is stable by construction.
export interface ManagedCollection<T> {
    // items is the collection as last loaded, empty until the first load resolves.
    items: T[]
    // reload refetches it, reporting a failure through setError rather than throwing.
    reload: () => Promise<void>
    // managing is whether the collection's manager dialog is open. Its setter keeps the full state
    // setter type, so a caller may still toggle it from the previous value.
    managing: boolean
    setManaging: Dispatch<SetStateAction<boolean>>
}

export function useManagedCollection<T>(
    load: () => Promise<T[]>,
    setError: (message: string) => void,
): ManagedCollection<T> {
    const [items, setItems] = useState<T[]>([])
    const [managing, setManaging] = useState<boolean>(false)

    const reload = useCallback(async () => {
        try {
            setItems(await load())
        } catch (e) {
            setError(String(e))
        }
    }, [load, setError])

    useEffect(() => {
        void reload()
    }, [reload])

    return {items, reload, managing, setManaging}
}
