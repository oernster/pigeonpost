import {describe, expect, it} from 'vitest'
import {
    clampList,
    clampSidebar,
    defaultPaneWidths,
    LIST_DEFAULT_PX,
    LIST_MIN_PX,
    LIST_STORED_MAX_PX,
    parsePaneWidths,
    READER_MIN_PX,
    serialisePaneWidths,
    SIDEBAR_DEFAULT_PX,
    SIDEBAR_MAX_PX,
    SIDEBAR_MIN_PX,
} from './paneLayout'

describe('clampSidebar', () => {
    it('passes an in-range width through, rounded', () => {
        expect(clampSidebar(300.4)).toBe(300)
    })

    it('bounds the width to its range', () => {
        expect(clampSidebar(10)).toBe(SIDEBAR_MIN_PX)
        expect(clampSidebar(9999)).toBe(SIDEBAR_MAX_PX)
    })
})

describe('clampList', () => {
    it('passes an in-range width through, rounded', () => {
        expect(clampList(400.6, 260, 1200)).toBe(401)
    })

    it('stops where the reader would fall below its floor', () => {
        expect(clampList(900, 260, 1200)).toBe(1200 - 260 - READER_MIN_PX)
    })

    it('holds the list floor', () => {
        expect(clampList(10, 260, 1200)).toBe(LIST_MIN_PX)
    })

    it('resolves a container too narrow for every floor to the list minimum', () => {
        // Degenerate bounds (max below min): the deterministic fallback is the minimum, so a drag in
        // a tiny window cannot produce a negative or thrashing width.
        expect(clampList(300, 260, 400)).toBe(LIST_MIN_PX)
    })
})

describe('parsePaneWidths', () => {
    it('returns the defaults for an absent value', () => {
        expect(parsePaneWidths(null)).toEqual(defaultPaneWidths())
    })

    it('returns the defaults for malformed JSON', () => {
        expect(parsePaneWidths('{nope')).toEqual(defaultPaneWidths())
    })

    it('returns the defaults for a non-object value', () => {
        expect(parsePaneWidths('"wat"')).toEqual(defaultPaneWidths())
        expect(parsePaneWidths('null')).toEqual(defaultPaneWidths())
    })

    it('round-trips serialised widths', () => {
        const widths = {sidebar: 320, list: 500}
        expect(parsePaneWidths(serialisePaneWidths(widths))).toEqual(widths)
    })

    it('clamps stored values and defaults non-numeric fields individually', () => {
        expect(parsePaneWidths('{"sidebar": 5, "list": 99999}')).toEqual({
            sidebar: SIDEBAR_MIN_PX,
            list: LIST_STORED_MAX_PX,
        })
        expect(parsePaneWidths('{"sidebar": "wide", "list": 400}')).toEqual({
            sidebar: SIDEBAR_DEFAULT_PX,
            list: 400,
        })
        expect(parsePaneWidths('{"sidebar": 300}')).toEqual({
            sidebar: 300,
            list: LIST_DEFAULT_PX,
        })
        expect(parsePaneWidths('{"list": NaN}')).toEqual(defaultPaneWidths())
    })
})
