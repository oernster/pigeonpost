// backdrop holds the one way a test clicks beside a dialog, for the nested editors that must survive it.
//
// A backdrop only arms DISMISS_ARM_MS after its dialog mounts, so a click any sooner is ignored whatever
// the dialog's rule; a test asserting "still open" would then pass for the wrong reason. The clock is
// therefore run to the arming point first, which needs fake timers started before the dialog mounted.
import {vi} from 'vitest'
import {act, fireEvent, screen} from '@testing-library/react'
import {DISMISS_ARM_MS} from '../components/useBackdropDismiss'

// clickBesideDialog presses and releases on the backdrop behind the dialog with the given accessible name,
// once that backdrop has armed. Call it with fake timers in force since before the dialog opened.
export function clickBesideDialog(name: string) {
    act(() => {
        vi.advanceTimersByTime(DISMISS_ARM_MS)
    })
    const backdrop = screen.getByRole('dialog', {name}).parentElement!
    fireEvent.mouseDown(backdrop)
    fireEvent.click(backdrop)
}
