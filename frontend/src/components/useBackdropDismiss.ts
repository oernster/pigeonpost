import {useCallback, useEffect, useRef, useState} from 'react'
import type {MouseEvent} from 'react'

// A freshly opened dialog will not dismiss on a backdrop click until this delay has passed. It stops the
// second click of a double-click (or any rapid click used to open the dialog) from landing on the
// just-rendered backdrop and closing the dialog before it is usable, which showed up as a dialog that
// flashed open and vanished.
const DISMISS_ARM_MS = 400

// escapeStack is a LIFO stack of the open dialogs' close callbacks (the newest sits last). Only the
// topmost dialog reacts to Escape, so when dialogs are stacked (a confirm over the calendar, say) Escape
// closes one layer at a time rather than all of them at once. Each entry is a ref so a dialog's latest
// onClose is used without pushing and popping on every render.
const escapeStack: Array<{current: () => void}> = []
let escapeListenerBound = false

// ensureEscapeListener binds the single shared Escape handler the first time a dialog registers. It routes
// Escape to the topmost dialog only and stays bound for the life of the app (one document, one listener).
function ensureEscapeListener() {
    if (escapeListenerBound) {
        return
    }
    escapeListenerBound = true
    document.addEventListener('keydown', (e) => {
        if (e.key !== 'Escape' || escapeStack.length === 0) {
            return
        }
        e.preventDefault()
        escapeStack[escapeStack.length - 1].current()
    })
}

// useEscapeToClose closes the calling dialog when Escape is pressed; it acts only while it is the topmost open
// dialog. Use it in any modal surface not already covered by useBackdropDismiss (which calls it), such as a
// nested sub-dialog that is deliberately not dismissable by a backdrop click. Pass active to gate
// registration on whether the surface is open, so the hook can be called unconditionally (as hook rules
// require) yet only take part in the stack while its surface is showing.
export function useEscapeToClose(onClose: () => void, active: boolean = true) {
    const ref = useRef(onClose)
    ref.current = onClose
    useEffect(() => {
        if (!active) {
            return
        }
        ensureEscapeListener()
        escapeStack.push(ref)
        return () => {
            const index = escapeStack.indexOf(ref)
            if (index >= 0) {
                escapeStack.splice(index, 1)
            }
        }
    }, [active])
}

// useBackdropDismiss returns handlers for a modal backdrop that close the dialog only when a full press
// and release both happen on the backdrop itself (not on the dialog content, and not a drag that started
// inside it) and only after a short arming delay. It also closes the dialog on Escape and returns focus to
// whatever opened it once it closes, so every dialog using it meets the keyboard-navigation contract.
// Spread the mouse handlers onto the backdrop element:
//   const dismiss = useBackdropDismiss(onClose)
//   <div className="modal-backdrop" {...dismiss}>
export function useBackdropDismiss(onClose: () => void) {
    const armedRef = useRef(false)
    const pressedOnBackdropRef = useRef(false)

    // Capture the element that had focus when the dialog opened. The initialiser runs during the first
    // render, before the dialog's own autoFocus moves focus inside it, so this is the opener rather than a
    // control within the dialog.
    const [opener] = useState<HTMLElement | null>(() => document.activeElement as HTMLElement | null)

    useEscapeToClose(onClose)

    useEffect(() => {
        armedRef.current = false
        const timer = window.setTimeout(() => {
            armedRef.current = true
        }, DISMISS_ARM_MS)
        return () => window.clearTimeout(timer)
    }, [])

    // On close, return focus to the opener if it is still on the page, so keyboard focus does not fall back
    // to nothing (which would leave the next Tab with no starting point in the ring).
    useEffect(() => {
        return () => {
            if (opener && opener.isConnected && typeof opener.focus === 'function') {
                opener.focus()
            }
        }
    }, [opener])

    const onMouseDown = useCallback((e: MouseEvent) => {
        pressedOnBackdropRef.current = e.target === e.currentTarget
    }, [])

    const onClick = useCallback((e: MouseEvent) => {
        if (e.target !== e.currentTarget) return
        if (!pressedOnBackdropRef.current) return
        if (!armedRef.current) return
        onClose()
    }, [onClose])

    return {onMouseDown, onClick}
}
