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
    licence: LoadedPanel<LicenceView>
}

// LicenceView is what the licence dialog shows: the licence's name for its title and the full text.
export interface LicenceView {
    name: string
    text: string
}

// loadLicence fetches the licence text with its name. The name is About's licence field, its one home on
// the backend, rather than a second statement of it or a guess parsed from the text's opening lines. The
// name only titles the dialog, so a failed About read leaves it empty (the plain title) rather than
// keeping the licence itself from opening; a failed text read still fails the load.
async function loadLicence(): Promise<LicenceView> {
    const [text, name] = await Promise.all([
        api.licence(),
        api.about().then((about) => about.licence, () => ''),
    ])
    return {name, text}
}

export function useHelpPanels(setError: (message: string) => void): HelpPanels {
    const [guideOpen, setGuideOpen] = useState(false)
    const showGuide = useCallback(() => setGuideOpen(true), [])
    const closeGuide = useCallback(() => setGuideOpen(false), [])
    const about = useLoadedPanel<AboutInfo>(api.about, setError)
    const licence = useLoadedPanel<LicenceView>(loadLicence, setError)
    return {guideOpen, showGuide, closeGuide, about, licence}
}
