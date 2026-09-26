// useAppEvents wires the backend's events to the views. ../../wailsjs/runtime is stubbed (the runtime
// seam): EventsOn records each handler under its event name so a test can raise the event itself. It
// answers the unsubscribe function every listener effect calls on cleanup.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import {useAppEvents} from './useAppEvents'
import {badgeRefresh} from '../test/badgeRefresh'

const badges = badgeRefresh()

const handlers = vi.hoisted(() => new Map<string, (arg?: unknown) => void>())

vi.mock('../../wailsjs/runtime', () => ({
    Environment: async () => ({platform: 'linux'}),
    EventsOn: (event: string, handler: (arg?: unknown) => void) => {
        handlers.set(event, handler)
        return () => handlers.delete(event)
    },
}))

const reloadCalls: string[] = []

function harness() {
    return renderHook(() => useAppEvents({
        showAbout: async () => {},
        showLicence: async () => {},
        checkUpdates: () => {},
        selectedFolder: 'f1',
        reloadFolder: async (id: string) => {
            reloadCalls.push(id)
        },
        ...badges.deps,
        loadEvents: async () => {},
        setError: () => {},
    }))
}

// raise fires a backend event the hook subscribed to, failing by name when it never subscribed.
async function raise(event: string) {
    const handler = handlers.get(event)
    expect(handler, `no handler subscribed to ${event}`).toBeDefined()
    await act(async () => {
        handler?.()
    })
}

beforeEach(() => {
    handlers.clear()
    reloadCalls.length = 0
    badges.reset()
})
afterEach(() => cleanup())

// The poller raises mail:new when mail arrives, so the badges must follow without waiting for a sync.
describe('useAppEvents: mail:new', () => {
    it('refreshes the unread badges and reloads the open folder', async () => {
        harness()

        await raise('mail:new')

        expect(badges.refreshed()).toBe(true)
        expect(reloadCalls).toEqual(['f1'])
    })
})
