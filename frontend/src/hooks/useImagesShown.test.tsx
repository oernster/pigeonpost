// useImagesShown is the one record of whether the reader has shown a message's remote images, read by the
// reader's Load images bar and by printing alike. These pin its contract: it starts from the auto-load
// setting, Load images shows a message's images; the record lasts only while that message is on screen,
// so a message the reader moved away from (or never opened) prints with its images still parked.
import {afterEach, describe, expect, it} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import {imagesShownFor, useImagesShown} from './useImagesShown'

afterEach(() => cleanup())

describe('useImagesShown', () => {
    it('keeps images hidden until Load images is pressed, then records the message as shown', () => {
        const {result} = renderHook(() => useImagesShown('m1', false))
        expect(result.current[0]).toBe(false)
        expect(imagesShownFor('m1')).toBe(false)
        act(() => result.current[1]())
        expect(result.current[0]).toBe(true)
        expect(imagesShownFor('m1')).toBe(true)
    })

    it('shows images from the first render when auto-load is on', () => {
        const {result} = renderHook(() => useImagesShown('m2', true))
        expect(result.current[0]).toBe(true)
        expect(imagesShownFor('m2')).toBe(true)
    })

    it('forgets a message once the reader moves to another one', () => {
        const {result, rerender} = renderHook(({id}) => useImagesShown(id, false), {initialProps: {id: 'm3'}})
        act(() => result.current[1]())
        rerender({id: 'm4'})
        expect(result.current[0]).toBe(false)
        expect(imagesShownFor('m3')).toBe(false)
        expect(imagesShownFor('m4')).toBe(false)
    })

    it('re-blocks a shown message when auto-load is switched off, as the reader always has', () => {
        const {result, rerender} = renderHook(({auto}) => useImagesShown('m5', auto), {initialProps: {auto: true}})
        rerender({auto: false})
        expect(result.current[0]).toBe(false)
        expect(imagesShownFor('m5')).toBe(false)
    })

    it('forgets a message when its view closes', () => {
        const {result, unmount} = renderHook(() => useImagesShown('m6', false))
        act(() => result.current[1]())
        unmount()
        expect(imagesShownFor('m6')).toBe(false)
    })
})
