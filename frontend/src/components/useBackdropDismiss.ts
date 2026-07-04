import {useCallback, useEffect, useRef} from 'react'
import type {MouseEvent} from 'react'

// A freshly opened dialog will not dismiss on a backdrop click until this delay has passed. It stops the
// second click of a double-click (or any rapid click used to open the dialog) from landing on the
// just-rendered backdrop and closing the dialog before it is usable, which showed up as a dialog that
// flashed open and vanished.
const DISMISS_ARM_MS = 400

// useBackdropDismiss returns handlers for a modal backdrop that close the dialog only when a full press
// and release both happen on the backdrop itself (not on the dialog content, and not a drag that started
// inside it) and only after a short arming delay. Spread the result onto the backdrop element:
//   const dismiss = useBackdropDismiss(onClose)
//   <div className="modal-backdrop" {...dismiss}>
export function useBackdropDismiss(onClose: () => void) {
    const armedRef = useRef(false)
    const pressedOnBackdropRef = useRef(false)

    useEffect(() => {
        armedRef.current = false
        const timer = window.setTimeout(() => {
            armedRef.current = true
        }, DISMISS_ARM_MS)
        return () => window.clearTimeout(timer)
    }, [])

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
