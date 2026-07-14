import {useRef, useState} from 'react'
import type {KeyboardEvent, Ref} from 'react'
import {stepTool} from '../toolbarNav'

// useToolbarNav makes a horizontal toolbar a single focus-ring stop (the WAI-ARIA toolbar
// pattern). Exactly one tool is tabbable at a time (a roving tabindex, remembered across
// visits); Left and Right move between tools wrapping at the ends and Home and End jump to the
// edges. The hook owns the container props and hands each tool its tabIndex by position.
export function useToolbarNav(count: number) {
    const [current, setCurrent] = useState(0)
    const ref = useRef<HTMLDivElement | null>(null)
    const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
        const next = stepTool(current, e.key, count)
        if (next === null) {
            return
        }
        e.preventDefault()
        e.stopPropagation()
        setCurrent(next)
        const tools = ref.current?.querySelectorAll<HTMLButtonElement>('button.compose-tool')
        tools?.[next]?.focus()
    }
    return {
        toolbarProps: {ref: ref as Ref<HTMLDivElement>, role: 'toolbar' as const, onKeyDown},
        toolTabIndex: (index: number) => (index === current ? 0 : -1),
    }
}
