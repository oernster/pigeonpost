// autoScroll is the gentle self-reading cycle for long help content: hold still on open, read down slowly,
// hold at the tail, rewind fast, repeat, and step aside the moment the reader takes over. It is the state
// machine only, pure and free of the DOM, so it sits under the 100% coverage gate; useAutoScroll drives it
// against a real element on a timer.
//
// The pace is the same one the desktop apps use and the constants belong to the app, not to any one
// surface: if a surface seems to need a different pace, the pace is wrong everywhere.

// TICK_MS is the clock. Every hold is counted down in whole ticks, so the granularity of a wait and the
// granularity of movement are the same.
export const TICK_MS = 40

// START_HOLD_MS is the stillness before the first descent: the reader orients before anything moves. It is
// the initial phase's wait rather than a special case, so it costs no extra state.
export const START_HOLD_MS = 5000

// The descent is one pixel every second tick. The divider is a countdown of ticks rather than a slower
// timer, so holds keep their TICK_MS granularity.
export const DESCENT_PX = 1
export const DESCENT_TICKS_PER_STEP = 2

// BOTTOM_HOLD_MS is long enough to finish reading the tail before the rewind takes it away.
export const BOTTOM_HOLD_MS = 5000

// REWIND_PX is a reposition, not a reading pass, so it travels fast. Never read at this pace and never
// rewind at the reading pace.
export const REWIND_PX = 15

// TOP_HOLD_MS is the breath before the next pass.
export const TOP_HOLD_MS = 2000

// MANUAL_HOLD_MS is the stillness required after any manual reading input before the cycle picks up again,
// from wherever the reader left it. Manual input suspends the cycle; it never switches it off.
export const MANUAL_HOLD_MS = 2500

// The phases: two reading movements and three holds. manual is a hold like the others, differing only in
// what it resumes into.
export type AutoScrollPhase = 'down' | 'pauseBottom' | 'up' | 'pauseTop' | 'manual'

export interface AutoScrollState {
    phase: AutoScrollPhase
    // waitMs is what is left of the current hold, ignored by the movement phases.
    waitMs: number
    // ticksToStep counts down to the next descent pixel.
    ticksToStep: number
}

// ScrollView is the part of a scrollable element the machine reads: where it sits and how far it can go.
export interface ScrollView {
    scrollTop: number
    maxScrollTop: number
}

// initialAutoScrollState opens in the top hold seeded with the start hold, so a fresh surface stands still
// before it first moves.
export function initialAutoScrollState(): AutoScrollState {
    return {phase: 'pauseTop', waitMs: START_HOLD_MS, ticksToStep: DESCENT_TICKS_PER_STEP}
}

// suspended returns the state a manual reading input puts the cycle into: a hold, from which it resumes at
// the reader's own position rather than restarting.
export function suspended(state: AutoScrollState): AutoScrollState {
    return {...state, phase: 'manual', waitMs: MANUAL_HOLD_MS}
}

// autoScrollTick advances the cycle by one tick and reports how far the surface should move, already
// clamped to its bounds. Content that does not overflow consumes nothing: no wait counts down and no phase
// changes, so attaching the cycle to a surface that currently fits is free and correct.
export function autoScrollTick(
    state: AutoScrollState, view: ScrollView,
): {state: AutoScrollState; delta: number} {
    if (view.maxScrollTop <= 0) {
        return {state, delta: 0}
    }
    if (state.phase === 'down') {
        return descend(state, view)
    }
    if (state.phase === 'up') {
        return rewind(state, view)
    }
    return hold(state, view)
}

// hold counts down the current wait and, when it runs out, chooses the direction to resume in: after the
// bottom hold the rewind, after a manual hold whatever is left to do from where the reader stopped (a
// reader who scrolled to the very end has only the rewind left), and otherwise the reading pass.
function hold(state: AutoScrollState, view: ScrollView): {state: AutoScrollState; delta: number} {
    const waitMs = state.waitMs - TICK_MS
    if (waitMs > 0) {
        return {state: {...state, waitMs}, delta: 0}
    }
    if (state.phase === 'pauseBottom') {
        return {state: {...state, phase: 'up', waitMs: 0}, delta: 0}
    }
    if (state.phase === 'manual' && view.scrollTop >= view.maxScrollTop) {
        return {state: {...state, phase: 'up', waitMs: 0}, delta: 0}
    }
    return {state: {phase: 'down', waitMs: 0, ticksToStep: DESCENT_TICKS_PER_STEP}, delta: 0}
}

// descend advances the reading pass by a pixel every DESCENT_TICKS_PER_STEP ticks, and hands over to the
// bottom hold on arrival.
function descend(state: AutoScrollState, view: ScrollView): {state: AutoScrollState; delta: number} {
    const ticksToStep = state.ticksToStep - 1
    if (ticksToStep > 0) {
        return {state: {...state, ticksToStep}, delta: 0}
    }
    const remaining = view.maxScrollTop - view.scrollTop
    if (remaining <= DESCENT_PX) {
        return {
            state: {phase: 'pauseBottom', waitMs: BOTTOM_HOLD_MS, ticksToStep: DESCENT_TICKS_PER_STEP},
            delta: Math.max(0, remaining),
        }
    }
    return {state: {...state, ticksToStep: DESCENT_TICKS_PER_STEP}, delta: DESCENT_PX}
}

// rewind travels back at the repositioning pace and hands over to the top hold on arrival.
function rewind(state: AutoScrollState, view: ScrollView): {state: AutoScrollState; delta: number} {
    if (view.scrollTop <= REWIND_PX) {
        return {
            state: {phase: 'pauseTop', waitMs: TOP_HOLD_MS, ticksToStep: DESCENT_TICKS_PER_STEP},
            delta: -view.scrollTop,
        }
    }
    return {state, delta: -REWIND_PX}
}
