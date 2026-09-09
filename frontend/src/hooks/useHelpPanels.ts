import {useCallback, useState} from 'react'
import {api} from '../api'
import type {AboutInfo} from '../api'
import {useLoadedPanel} from './useLoadedPanel'
import type {LoadedPanel} from './useLoadedPanel'

// useHelpPanels owns the three panels behind the Help menu, so App holds one value for the menu rather
// than a state flag and two panels it wires separately. They belong together: each is opened by one menu
// entry, each shows one read-only dialog and none of them is written back.
//
// The guide is the odd one and is why this hook exists as more than a rename. Its words ship with the
// front end, so there is nothing to fetch and nothing to drop on close: it is a flag, not a loaded panel,
// and useLoadedPanel would have to grow a no-op load to pretend otherwise.
export interface HelpPanels {
    // guideOpen is whether the guide dialog is showing; the guide holds no fetched content of its own.
    guideOpen: boolean
    showGuide: () => void
    closeGuide: () => void
    // about and licence each fetch on open and drop their content on close.
    about: LoadedPanel<AboutInfo>
    licence: LoadedPanel<string>
}

export function useHelpPanels(setError: (message: string) => void): HelpPanels {
    const [guideOpen, setGuideOpen] = useState(false)
    const showGuide = useCallback(() => setGuideOpen(true), [])
    const closeGuide = useCallback(() => setGuideOpen(false), [])
    const about = useLoadedPanel<AboutInfo>(api.about, setError)
    const licence = useLoadedPanel<string>(api.licence, setError)
    return {guideOpen, showGuide, closeGuide, about, licence}
}
