import {useEffect, useState} from 'react'
import {EditContext, readEditContext} from '../editClipboard'

const idleContext: EditContext = {editableFocused: false, hasSelection: false, selectionInEditable: false}

// useEditContext tracks what Edit > Cut / Copy / Paste could currently act on, refreshed on every
// selection or focus change so the menu items enable and disable live. Unchanged snapshots keep the
// previous state object, so the stream of selectionchange events does not re-render the app.
export function useEditContext(): EditContext {
    const [ctx, setCtx] = useState<EditContext>(idleContext)

    useEffect(() => {
        const update = () => setCtx((prev) => {
            const next = readEditContext(document)
            const same = next.editableFocused === prev.editableFocused &&
                next.hasSelection === prev.hasSelection &&
                next.selectionInEditable === prev.selectionInEditable
            return same ? prev : next
        })
        document.addEventListener('selectionchange', update)
        window.addEventListener('focusin', update)
        window.addEventListener('focusout', update)
        return () => {
            document.removeEventListener('selectionchange', update)
            window.removeEventListener('focusin', update)
            window.removeEventListener('focusout', update)
        }
    }, [])

    return ctx
}
