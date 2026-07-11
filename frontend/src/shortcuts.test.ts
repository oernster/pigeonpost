import {describe, expect, it} from 'vitest'
import {matchesShortcut} from './shortcuts'

const ev = (init: KeyboardEventInit): KeyboardEvent => new KeyboardEvent('keydown', init)

describe('matchesShortcut', () => {
    it('matches Ctrl plus a key', () => {
        expect(matchesShortcut(ev({key: 'n', ctrlKey: true}), 'Ctrl+N')).toBe(true)
    })

    it('treats the Cmd (meta) key as Ctrl', () => {
        expect(matchesShortcut(ev({key: 'n', metaKey: true}), 'Ctrl+N')).toBe(true)
    })

    it('does not match when the required Ctrl is absent', () => {
        expect(matchesShortcut(ev({key: 'n'}), 'Ctrl+N')).toBe(false)
    })

    it('matches a Ctrl plus Shift combination', () => {
        expect(matchesShortcut(ev({key: 'k', ctrlKey: true, shiftKey: true}), 'Ctrl+Shift+K')).toBe(true)
    })

    it('does not match when a required Shift is absent', () => {
        expect(matchesShortcut(ev({key: 'k', ctrlKey: true}), 'Ctrl+Shift+K')).toBe(false)
    })

    it('matches an Alt combination', () => {
        expect(matchesShortcut(ev({key: 'x', altKey: true}), 'Alt+X')).toBe(true)
    })

    it('does not match when a required Alt is absent', () => {
        expect(matchesShortcut(ev({key: 'x'}), 'Alt+X')).toBe(false)
    })

    it('matches a bare function key with no modifiers', () => {
        expect(matchesShortcut(ev({key: 'F9'}), 'F9')).toBe(true)
    })

    it('does not match a different key', () => {
        expect(matchesShortcut(ev({key: 'm', ctrlKey: true}), 'Ctrl+N')).toBe(false)
    })
})
