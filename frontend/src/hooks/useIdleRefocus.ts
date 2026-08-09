import {useEffect, useRef} from 'react'

// IDLE_REFOCUS_MS is how long the app must see no user activity (keys, pointer, wheel) before the
// keyboard focus is returned to its resting place: the active account's Inbox folder row. Long enough
// that it never interrupts someone mid-thought, short enough that an abandoned focus position (left on
// a button after a click; lost to a closed dialog) is tidied away promptly.
export const IDLE_REFOCUS_MS = 10_000

// ACTIVITY_EVENTS are the user-input events that count as activity: any of them restarts the idle
// timer. Captured on window so activity anywhere in the app counts, whatever element handled it.
const ACTIVITY_EVENTS = ['keydown', 'pointerdown', 'pointermove', 'wheel'] as const

// useIdleRefocus invokes onIdle once each time the user has been inactive for IDLE_REFOCUS_MS. The
// timer restarts on any activity; after firing it stays quiet until the user is active again, so an
// untouched app is not re-focused every ten seconds. The callback is kept in a ref, so the listener
// set is installed once and never re-bound as the callback's dependencies change.
export function useIdleRefocus(onIdle: () => void) {
    const callbackRef = useRef(onIdle)
    callbackRef.current = onIdle

    useEffect(() => {
        let timer: number | null = null
        const restart = () => {
            if (timer !== null) {
                window.clearTimeout(timer)
            }
            timer = window.setTimeout(() => {
                timer = null
                callbackRef.current()
            }, IDLE_REFOCUS_MS)
        }
        ACTIVITY_EVENTS.forEach((event) => window.addEventListener(event, restart, true))
        restart()
        return () => {
            if (timer !== null) {
                window.clearTimeout(timer)
            }
            ACTIVITY_EVENTS.forEach((event) => window.removeEventListener(event, restart, true))
        }
    }, [])
}

// isTypingTarget reports whether the element is a text-entry surface (an input, a textarea or a
// contenteditable region such as the compose editor). Idle refocusing skips these: pulling the caret
// out of a field someone paused in would lose their place mid-entry.
export function isTypingTarget(element: Element | null): boolean {
    if (!(element instanceof HTMLElement)) {
        return false
    }
    if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
        return true
    }
    // The attribute check backs up isContentEditable, which jsdom does not implement; an inherited
    // editable region (a child of the compose editor) still reports true via isContentEditable in a
    // real browser.
    return element.isContentEditable || element.getAttribute('contenteditable') === 'true'
}
