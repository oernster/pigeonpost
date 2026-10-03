import {useCallback, useEffect, useSyncExternalStore} from 'react'

// shownMessages is the one record of which messages the reader is currently showing with their remote images
// loaded, whether by a press of Load images or by the auto-load setting. It lives outside any component so
// the reader's Load images bar and the print path read the same answer: printing restores a message's images
// only when this says the reader already chose to see them. A message is recorded only while its view is on
// screen, matching the reader's long-standing reset: moving to another message, closing the view or switching
// auto-load off re-blocks its images.
const shownMessages = new Set<string>()
const listeners = new Set<() => void>()

function subscribe(listener: () => void): () => void {
    listeners.add(listener)
    return () => {
        listeners.delete(listener)
    }
}

// setShown records or clears one message and tells every subscribed view, skipping a no-op so a view that
// re-applies the same answer does not re-render the others.
function setShown(messageId: string, shown: boolean) {
    if (shownMessages.has(messageId) === shown) {
        return
    }
    if (shown) {
        shownMessages.add(messageId)
    } else {
        shownMessages.delete(messageId)
    }
    listeners.forEach((listener) => listener())
}

// imagesShownFor reports whether the reader is showing a message with its remote images loaded. It is what
// the print path asks before restoring any image.
export function imagesShownFor(messageId: string): boolean {
    return shownMessages.has(messageId)
}

// useImagesShown is the reader's handle on the record for one message: whether its images are shown plus the
// action behind Load images. It starts from the auto-load setting (true from the first render when that is
// on, so a reader with auto-load never flashes the Load images bar) and forgets the message when the view
// moves on or closes.
export function useImagesShown(messageId: string, autoLoadImages: boolean): [boolean, () => void] {
    const recorded = useSyncExternalStore(subscribe, () => shownMessages.has(messageId))

    useEffect(() => {
        setShown(messageId, autoLoadImages)
        return () => setShown(messageId, false)
    }, [messageId, autoLoadImages])

    const show = useCallback(() => setShown(messageId, true), [messageId])
    return [autoLoadImages || recorded, show]
}
