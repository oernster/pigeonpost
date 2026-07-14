import {describe, expect, it} from 'vitest'
import {stepTool} from './toolbarNav'

describe('stepTool', () => {
    it('moves right and wraps at the end', () => {
        expect(stepTool(0, 'ArrowRight', 3)).toBe(1)
        expect(stepTool(2, 'ArrowRight', 3)).toBe(0)
    })

    it('moves left and wraps at the start', () => {
        expect(stepTool(2, 'ArrowLeft', 3)).toBe(1)
        expect(stepTool(0, 'ArrowLeft', 3)).toBe(2)
    })

    it('jumps to the edges with Home and End', () => {
        expect(stepTool(1, 'Home', 3)).toBe(0)
        expect(stepTool(1, 'End', 3)).toBe(2)
    })

    it('ignores keys that are not toolbar navigation', () => {
        expect(stepTool(1, 'ArrowDown', 3)).toBeNull()
        expect(stepTool(1, 'Enter', 3)).toBeNull()
        expect(stepTool(1, 'a', 3)).toBeNull()
    })

    it('ignores every key on an empty toolbar', () => {
        expect(stepTool(0, 'ArrowRight', 0)).toBeNull()
    })
})
