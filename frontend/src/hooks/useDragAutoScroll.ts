import {useEffect, type RefObject} from 'react'
import {dragScrollStep} from '../dragScroll'

// useDragAutoScroll gives a scrollable pane its own edge auto-scroll while something is dragged over it.
// Two things are wrong with leaving this to the browser: its hot zone is only a couple of pixels deep at
// the pane's edge, so reaching a folder below the fold means holding the drag on a target barely wider than
// the cursor, and it scrolls only while the pointer moves. This widens the zone (see dragScroll) and drives
// the scroll from an animation frame loop keyed off the last pointer position, so resting in the zone keeps
// the pane moving.
//
// The loop stops on drop, on dragend and when the pointer leaves the pane for something outside it. A
// dragleave whose relatedTarget is still inside the pane is the pointer crossing between rows, not leaving.
export function useDragAutoScroll(ref: RefObject<HTMLElement | null>): void {
    useEffect(() => {
        const el = ref.current
        if (!el) {
            return
        }
        let pointerY: number | null = null
        let frame = 0

        const tick = () => {
            frame = 0
            if (pointerY === null) {
                return
            }
            const step = dragScrollStep(pointerY, el.getBoundingClientRect())
            if (step !== 0) {
                el.scrollTop += step
            }
            frame = requestAnimationFrame(tick)
        }

        const stop = () => {
            pointerY = null
            if (frame !== 0) {
                cancelAnimationFrame(frame)
                frame = 0
            }
        }

        const onDragOver = (e: DragEvent) => {
            pointerY = e.clientY
            if (frame === 0) {
                frame = requestAnimationFrame(tick)
            }
        }

        const onDragLeave = (e: DragEvent) => {
            const to = e.relatedTarget
            if (!(to instanceof Node) || !el.contains(to)) {
                stop()
            }
        }

        el.addEventListener('dragover', onDragOver)
        el.addEventListener('dragleave', onDragLeave)
        el.addEventListener('drop', stop)
        // A drag can end anywhere, including outside the window, so the end of it is watched globally.
        window.addEventListener('dragend', stop)
        window.addEventListener('drop', stop)
        return () => {
            stop()
            el.removeEventListener('dragover', onDragOver)
            el.removeEventListener('dragleave', onDragLeave)
            el.removeEventListener('drop', stop)
            window.removeEventListener('dragend', stop)
            window.removeEventListener('drop', stop)
        }
    }, [ref])
}
