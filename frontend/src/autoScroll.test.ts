import {describe, expect, it} from 'vitest'
import {
    BOTTOM_HOLD_MS,
    DESCENT_PX,
    DESCENT_TICKS_PER_STEP,
    MANUAL_HOLD_MS,
    REWIND_PX,
    START_HOLD_MS,
    TICK_MS,
    TOP_HOLD_MS,
    autoScrollTick,
    initialAutoScrollState,
    suspended,
    type AutoScrollState,
    type ScrollView,
} from './autoScroll'

const view = (scrollTop: number, maxScrollTop = 1000): ScrollView => ({scrollTop, maxScrollTop})

// run drives the machine for a number of ticks against a view that moves with it, and reports where it
// ended up. The view is recomputed each tick, as the hook does against a real element.
function run(state: AutoScrollState, ticks: number, start = 0, max = 1000) {
    let scrollTop = start
    let current = state
    for (let i = 0; i < ticks; i++) {
        const result = autoScrollTick(current, view(scrollTop, max))
        current = result.state
        scrollTop += result.delta
    }
    return {state: current, scrollTop}
}

// ticksFor converts a hold into the number of ticks that exhausts it.
const ticksFor = (ms: number) => Math.ceil(ms / TICK_MS)

describe('autoScroll: opening', () => {
    it('holds still on open, before the first descent', () => {
        const {state, scrollTop} = run(initialAutoScrollState(), ticksFor(START_HOLD_MS) - 1)
        expect(state.phase).toBe('pauseTop')
        expect(scrollTop).toBe(0)
    })

    it('starts reading down once the start hold has run', () => {
        const {state} = run(initialAutoScrollState(), ticksFor(START_HOLD_MS))
        expect(state.phase).toBe('down')
    })
})

describe('autoScroll: the reading pass', () => {
    const reading = (): AutoScrollState => ({phase: 'down', waitMs: 0, ticksToStep: DESCENT_TICKS_PER_STEP})

    it('advances a pixel every second tick, not every tick', () => {
        expect(run(reading(), 1).scrollTop).toBe(0)
        expect(run(reading(), 2).scrollTop).toBe(DESCENT_PX)
        expect(run(reading(), 20).scrollTop).toBe(10 * DESCENT_PX)
    })

    it('stops exactly at the end and holds there', () => {
        const {state, scrollTop} = run(reading(), 4, 998, 1000)
        expect(scrollTop).toBe(1000)
        expect(state.phase).toBe('pauseBottom')
        expect(state.waitMs).toBe(BOTTOM_HOLD_MS)
    })

    it('lands on the end without overshooting a part-pixel remainder', () => {
        const {scrollTop} = run(reading(), 2, 999.5, 1000)
        expect(scrollTop).toBe(1000)
    })
})

describe('autoScroll: the rewind', () => {
    const rewinding = (): AutoScrollState => ({phase: 'up', waitMs: 0, ticksToStep: DESCENT_TICKS_PER_STEP})

    it('travels far faster than the reading pass', () => {
        expect(run(rewinding(), 1, 500).scrollTop).toBe(500 - REWIND_PX)
        expect(REWIND_PX).toBeGreaterThan(DESCENT_PX * DESCENT_TICKS_PER_STEP)
    })

    it('settles at the top and holds before the next pass', () => {
        const {state, scrollTop} = run(rewinding(), 1, REWIND_PX - 1)
        expect(scrollTop).toBe(0)
        expect(state.phase).toBe('pauseTop')
        expect(state.waitMs).toBe(TOP_HOLD_MS)
    })

    it('goes back to reading after the top hold', () => {
        const {state} = run({phase: 'pauseTop', waitMs: TOP_HOLD_MS, ticksToStep: 1}, ticksFor(TOP_HOLD_MS))
        expect(state.phase).toBe('down')
    })
})

describe('autoScroll: the bottom hold', () => {
    it('waits, then rewinds rather than reading on', () => {
        const held: AutoScrollState = {phase: 'pauseBottom', waitMs: BOTTOM_HOLD_MS, ticksToStep: 1}
        expect(run(held, ticksFor(BOTTOM_HOLD_MS) - 1, 1000).state.phase).toBe('pauseBottom')
        expect(run(held, ticksFor(BOTTOM_HOLD_MS), 1000).state.phase).toBe('up')
    })
})

describe('autoScroll: manual reading', () => {
    it('suspends the cycle without losing the phase it was in', () => {
        const reading: AutoScrollState = {phase: 'down', waitMs: 0, ticksToStep: 2}
        const held = suspended(reading)
        expect(held.phase).toBe('manual')
        expect(held.waitMs).toBe(MANUAL_HOLD_MS)
    })

    it('holds still for the whole suspension', () => {
        const {scrollTop, state} = run(suspended(initialAutoScrollState()), ticksFor(MANUAL_HOLD_MS) - 1, 300)
        expect(scrollTop).toBe(300)
        expect(state.phase).toBe('manual')
    })

    it('resumes reading down from where the reader left it', () => {
        const {state} = run(suspended(initialAutoScrollState()), ticksFor(MANUAL_HOLD_MS), 300)
        expect(state.phase).toBe('down')
        // Nothing in the state carries a position: the cycle picks up at whatever the element now shows.
        expect(state.waitMs).toBe(0)
    })

    it('rewinds instead when the reader has already scrolled to the very end', () => {
        const {state} = run(suspended(initialAutoScrollState()), ticksFor(MANUAL_HOLD_MS), 1000)
        expect(state.phase).toBe('up')
    })

    it('suspends again from a manual hold rather than switching the cycle off', () => {
        const twice = suspended(suspended(initialAutoScrollState()))
        expect(twice.phase).toBe('manual')
        expect(run(twice, ticksFor(MANUAL_HOLD_MS)).state.phase).toBe('down')
    })
})

describe('autoScroll: content that does not overflow', () => {
    it('consumes nothing at all, so attaching the cycle to a short surface is free', () => {
        const opening = initialAutoScrollState()
        const {state, delta} = autoScrollTick(opening, {scrollTop: 0, maxScrollTop: 0})
        expect(state).toBe(opening)
        expect(delta).toBe(0)
    })

    it('leaves a mid-cycle hold exactly where it was', () => {
        const held: AutoScrollState = {phase: 'pauseBottom', waitMs: 1234, ticksToStep: 2}
        expect(autoScrollTick(held, {scrollTop: 0, maxScrollTop: 0}).state.waitMs).toBe(1234)
    })
})
