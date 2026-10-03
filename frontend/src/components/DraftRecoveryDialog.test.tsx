// The recovery prompt must say plainly what a restore brings back. The snapshot holds the address
// fields, the subject and the body only, so attached files are gone; restoring once said nothing about
// that and the files simply went missing (audit P-11).
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, render, screen} from '@testing-library/react'
import {DraftRecoveryDialog} from './DraftRecoveryDialog'
import type {DraftRecoveryResult} from '../api'

afterEach(cleanup)

function renderDialog() {
    const recovery = {subject: 'Quarterly figures'} as DraftRecoveryResult
    render(<DraftRecoveryDialog recovery={recovery} setRecovery={vi.fn()} discardDraft={vi.fn()}
                                restoreDraft={vi.fn()}/>)
    return screen.getByRole('alertdialog', {name: 'Restore unsent message'})
}

describe('DraftRecoveryDialog', () => {
    it('names the message by its subject', () => {
        expect(renderDialog()).toHaveTextContent('"Quarterly figures"')
    })

    it('says plainly that attached files are not kept', () => {
        expect(renderDialog()).toHaveTextContent('Attached files are not kept')
    })
})
