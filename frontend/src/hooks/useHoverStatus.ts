import {createContext, useContext, useMemo, useSyncExternalStore, type MouseEvent} from 'react'
import {createHoverChannel, type HoverChannel, type HoverStatus} from '../hoverStatus'
import {hoverStatusAt} from '../hoverProbe'

// HoverChannelContext hands the app's one hover channel to every surface that can report what is under
// the pointer. App creates the channel; outside it (a component rendered on its own in a test) there is
// none and reports go nowhere.
export const HoverChannelContext = createContext<HoverChannel | null>(null)

// useHoverChannelOwner creates the channel for the lifetime of the component that owns it (App).
export function useHoverChannelOwner(): HoverChannel {
    return useMemo(createHoverChannel, [])
}

const noReport = () => {}

// useHoverReport answers the function a surface calls with its reading; null clears the tray.
export function useHoverReport(): (status: HoverStatus | null) => void {
    return useContext(HoverChannelContext)?.report ?? noReport
}

const noStatus = () => null
const noSubscribe = () => noReport

// useHoverStatus answers the current reading, re-rendering the caller only when it changes. The tray is
// its one reader.
export function useHoverStatus(): HoverStatus | null {
    const channel = useContext(HoverChannelContext)
    return useSyncExternalStore(channel?.subscribe ?? noSubscribe, channel?.current ?? noStatus)
}

// useHoverProbe answers pointer handlers for an element in the app's own document (a plain-text body,
// the compose editor) that report the link or emoji under the pointer and clear the tray on leaving.
export function useHoverProbe() {
    const report = useHoverReport()
    return {
        onMouseMove: (event: MouseEvent) => report(hoverStatusAt(document, event.target, event.clientX, event.clientY)),
        onMouseLeave: () => report(null),
    }
}
