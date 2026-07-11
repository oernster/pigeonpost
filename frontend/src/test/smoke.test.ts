import {describe, expect, it} from 'vitest'

// A smoke test proving the harness runs: the assertion library works and the jsdom
// environment provides a document. It exercises no product code.
describe('test harness', () => {
    it('runs assertions', () => {
        expect(1 + 1).toBe(2)
    })

    it('provides a jsdom document', () => {
        const el = document.createElement('div')
        el.textContent = 'pigeonpost'
        expect(el.textContent).toBe('pigeonpost')
    })
})
