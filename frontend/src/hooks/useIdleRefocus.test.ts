// Tests for the idle-refocus timer: it fires its callback once after IDLE_REFOCUS_MS without user
// activity, restarts on any activity, stays quiet after firing until the user is active again and
// unhooks its listeners on unmount. isTypingTarget pins which elements count as text entry.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, renderHook} from '@testing-library/react'
import {IDLE_REFOCUS_MS, isTypingTarget, useIdleRefocus} from './useIdleRefocus'

describe('useIdleRefocus', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        cleanup()
        vi.useRealTimers()
    })

    it('fires once after the idle interval and not again while idle continues', () => {
        const onIdle = vi.fn()
        renderHook(() => useIdleRefocus(onIdle))

        vi.advanceTimersByTime(IDLE_REFOCUS_MS - 1)
        expect(onIdle).not.toHaveBeenCalled()
        vi.advanceTimersByTime(1)
        expect(onIdle).toHaveBeenCalledTimes(1)

        // No further activity: the callback must not repeat every interval.
        vi.advanceTimersByTime(IDLE_REFOCUS_MS * 3)
        expect(onIdle).toHaveBeenCalledTimes(1)
    })

    it('restarts the countdown on user activity', () => {
        const onIdle = vi.fn()
        renderHook(() => useIdleRefocus(onIdle))

        vi.advanceTimersByTime(IDLE_REFOCUS_MS - 1)
        window.dispatchEvent(new Event('pointermove'))
        vi.advanceTimersByTime(IDLE_REFOCUS_MS - 1)
        expect(onIdle).not.toHaveBeenCalled()
        vi.advanceTimersByTime(1)
        expect(onIdle).toHaveBeenCalledTimes(1)
    })

    it('re-arms after firing once the user is active again', () => {
        const onIdle = vi.fn()
        renderHook(() => useIdleRefocus(onIdle))

        vi.advanceTimersByTime(IDLE_REFOCUS_MS)
        expect(onIdle).toHaveBeenCalledTimes(1)

        window.dispatchEvent(new Event('keydown'))
        vi.advanceTimersByTime(IDLE_REFOCUS_MS)
        expect(onIdle).toHaveBeenCalledTimes(2)
    })

    it('stops firing after unmount', () => {
        const onIdle = vi.fn()
        const {unmount} = renderHook(() => useIdleRefocus(onIdle))
        unmount()

        window.dispatchEvent(new Event('keydown'))
        vi.advanceTimersByTime(IDLE_REFOCUS_MS * 2)
        expect(onIdle).not.toHaveBeenCalled()
    })

    it('invokes the latest callback, not the one from first render', () => {
        const first = vi.fn()
        const second = vi.fn()
        const {rerender} = renderHook(({cb}) => useIdleRefocus(cb), {initialProps: {cb: first}})
        rerender({cb: second})

        vi.advanceTimersByTime(IDLE_REFOCUS_MS)
        expect(first).not.toHaveBeenCalled()
        expect(second).toHaveBeenCalledTimes(1)
    })
})

describe('isTypingTarget', () => {
    it('treats inputs, textareas and contenteditable regions as typing targets', () => {
        expect(isTypingTarget(document.createElement('input'))).toBe(true)
        expect(isTypingTarget(document.createElement('textarea'))).toBe(true)

        const editable = document.createElement('div')
        editable.setAttribute('contenteditable', 'true')
        expect(isTypingTarget(editable)).toBe(true)
    })

    it('treats other elements and null as not typing targets', () => {
        expect(isTypingTarget(document.createElement('button'))).toBe(false)
        expect(isTypingTarget(document.createElement('li'))).toBe(false)
        expect(isTypingTarget(null)).toBe(false)
    })
})
