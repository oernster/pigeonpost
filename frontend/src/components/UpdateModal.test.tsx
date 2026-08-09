// Tests for the update-check modal at its outer interface (status, onClose, onDownload, onSkip):
// what each outcome renders and which callback each button fires with what.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {UpdateModal} from './UpdateModal'
import type {UpdateStatus} from '../api'

afterEach(cleanup)

const noop = () => undefined

function status(overrides: Partial<UpdateStatus> = {}): UpdateStatus {
    return {
        current: '1.13.2',
        latest: 'v1.14.0',
        updateAvailable: true,
        downloadUrl: 'https://dl/setup.exe',
        pageUrl: 'https://github.com/oernster/pigeonpost/releases/tag/v1.14.0',
        ...overrides,
    } as UpdateStatus
}

describe('UpdateModal', () => {
    it('renders nothing without a status', () => {
        const {container} = render(
            <UpdateModal status={null} onClose={noop} onDownload={noop} onSkip={noop}/>,
        )
        expect(container.innerHTML).toBe('')
    })

    it('offers the download for a newer release and closes after opening it', () => {
        const onDownload = vi.fn()
        const onClose = vi.fn()
        render(<UpdateModal status={status()} onClose={onClose} onDownload={onDownload} onSkip={noop}/>)
        expect(screen.getByText(/1\.14\.0 is available/)).toBeTruthy()
        expect(screen.getByText(/running 1\.13\.2/)).toBeTruthy()
        fireEvent.click(screen.getByText('Download'))
        expect(onDownload).toHaveBeenCalledWith('https://dl/setup.exe')
        expect(onClose).toHaveBeenCalled()
    })

    it('falls back to the release page when the release carries no platform asset', () => {
        const onDownload = vi.fn()
        render(
            <UpdateModal
                status={status({downloadUrl: ''})}
                onClose={noop}
                onDownload={onDownload}
                onSkip={noop}
            />,
        )
        fireEvent.click(screen.getByText('Download'))
        expect(onDownload).toHaveBeenCalledWith('https://github.com/oernster/pigeonpost/releases/tag/v1.14.0')
    })

    it('skips the offered release and closes', () => {
        const onSkip = vi.fn()
        const onClose = vi.fn()
        render(<UpdateModal status={status()} onClose={onClose} onDownload={noop} onSkip={onSkip}/>)
        fireEvent.click(screen.getByText('Skip This Version'))
        expect(onSkip).toHaveBeenCalledWith('v1.14.0')
        expect(onClose).toHaveBeenCalled()
    })

    it('Later just closes without downloading or skipping', () => {
        const onDownload = vi.fn()
        const onSkip = vi.fn()
        const onClose = vi.fn()
        render(<UpdateModal status={status()} onClose={onClose} onDownload={onDownload} onSkip={onSkip}/>)
        fireEvent.click(screen.getByText('Later'))
        expect(onClose).toHaveBeenCalled()
        expect(onDownload).not.toHaveBeenCalled()
        expect(onSkip).not.toHaveBeenCalled()
    })

    it('reports up to date when a manual check found nothing newer', () => {
        render(
            <UpdateModal
                status={status({updateAvailable: false})}
                onClose={noop}
                onDownload={noop}
                onSkip={noop}
            />,
        )
        expect(screen.getByText('You are running the latest version.')).toBeTruthy()
    })

    it('reports an unreachable check when there is no latest version', () => {
        render(
            <UpdateModal
                status={status({updateAvailable: false, latest: '', downloadUrl: '', pageUrl: ''})}
                onClose={noop}
                onDownload={noop}
                onSkip={noop}
            />,
        )
        expect(screen.getByText(/could not reach GitHub/)).toBeTruthy()
    })
})
