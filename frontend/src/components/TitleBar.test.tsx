// Behaviour test for the title bar's action run. What it holds is the ORDER of that run and where its
// dividers fall, because the bar is read left to right and a control in the wrong group reads as
// belonging to the wrong thing. Nothing else asserted this, so the arrangement lived only in the JSX.
import {describe, expect, it, vi} from 'vitest'
import {cleanup, render, screen} from '@testing-library/react'
import {afterEach} from 'vitest'
import {TitleBar} from './TitleBar'
import type {ManagerOpeners} from './TitleBar'

const managers: ManagerOpeners = {
    contacts: vi.fn(),
    calendar: vi.fn(),
    rules: vi.fn(),
    templates: vi.fn(),
}

function renderBar() {
    render(
        <TitleBar
            unreadCounts={{total: 0, perAccount: {}} as never}
            fileMenu={[]}
            editMenu={[]}
            viewMenu={[]}
            mailMenu={[]}
            helpMenu={[]}
            selectedAccount="a1"
            accountSyncing={false}
            theme="dark"
            signatureHtml={() => ''}
            setComposeInitial={vi.fn()}
            setComposing={vi.fn()}
            setSettingUp={vi.fn()}
            sync={async () => {}}
            managers={managers}
            setTheme={vi.fn()}
        />,
    )
}

afterEach(cleanup)

describe('TitleBar', () => {
    // The run reads: the menus, then the two managers on their own, then Mail with the working controls.
    // A divider marks each seam; there is deliberately none between Mail and Compose, which belong
    // together.
    it('lays its actions out in groups divided where the groups change', () => {
        renderBar()
        const run = document.querySelector('.titlebar-actions')
        expect(run).not.toBeNull()

        // A menu renders a wrapper around its trigger, so the name is read from the button either way.
        const labelled = [...(run?.children ?? [])].map((el) => {
            if (el.className.includes('titlebar-sep')) {
                return '|'
            }
            const button = el.tagName === 'BUTTON' ? el : el.querySelector('button')
            return button?.getAttribute('aria-label') ?? ''
        })

        expect(labelled).toEqual([
            'File', 'Edit', 'View',
            '|',
            'Filter rules', 'Message templates',
            '|',
            'Mail', 'Compose', 'Add account', 'Sync',
            '|',
            'Contacts', 'Calendar',
        ])
    })

    it('opens the rules manager from the bar', () => {
        renderBar()
        screen.getByRole('button', {name: 'Filter rules'}).click()
        expect(managers.rules).toHaveBeenCalledWith(true)
    })

    it('opens the template manager from the bar', () => {
        renderBar()
        screen.getByRole('button', {name: 'Message templates'}).click()
        expect(managers.templates).toHaveBeenCalledWith(true)
    })
})
