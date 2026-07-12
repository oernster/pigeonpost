import {Dispatch, SetStateAction, useEffect, useState} from 'react'
import {EmailView} from '../api'
import {Environment, EventsOn} from '../../wailsjs/runtime'

// AppEventsDeps is what the backend-event wiring needs from the rest of App: the Help-dialog openers the tray
// menu mirrors (showAbout, showLicence, checkUpdates), the open folder and the folder reloader (mail:new
// reloads the folder on screen, resetting the flat view's pagination rather than pulling every row), the
// unread-count and events loaders the poll events refresh plus the error sink.
export interface AppEventsDeps {
    showAbout: () => Promise<void>
    showLicence: () => Promise<void>
    checkUpdates: () => void
    selectedFolder: string
    // reloadFolder resets pagination and reloads the folder view; skipSync loads once without re-syncing,
    // because the arrival was already fetched by the backend poller that raised mail:new.
    reloadFolder: (id: string, opts?: {skipSync?: boolean}) => Promise<void>
    loadUnread: () => Promise<void>
    loadEvents: () => Promise<void>
    setError: (message: string) => void
}

export interface AppEvents {
    // launchedEmail feeds the EmailViewer, isWindows gates the Mail menu's Windows-only item, and closeChoice
    // drives the minimise-or-quit dialog. All three are owned here and consumed by App's render.
    launchedEmail: EmailView | null
    setLaunchedEmail: Dispatch<SetStateAction<EmailView | null>>
    isWindows: boolean
    closeChoice: boolean
    setCloseChoice: Dispatch<SetStateAction<boolean>>
}

// useAppEvents owns the backend-event wiring: the Windows tray menu and the close-request, an OS-handed .eml
// (eml:open), and the poll events that refresh the unread counts and the open folder (mail:new) or the
// calendar (calendar:changed). It also detects Windows once on mount. The launchedEmail, isWindows and
// closeChoice state live here; App's render consumes them (the EmailViewer, the Mail menu and the CloseChoice
// dialog), so they are returned.
export function useAppEvents(deps: AppEventsDeps): AppEvents {
    const {showAbout, showLicence, checkUpdates, selectedFolder, reloadFolder, loadUnread, loadEvents, setError} = deps

    // launchedEmail holds a .eml the OS handed to PigeonPost (a double-click on the file) while the in-app
    // viewer shows it; isWindows gates the Windows-only "Set as default for .eml" menu item.
    const [launchedEmail, setLaunchedEmail] = useState<EmailView | null>(null)
    const [isWindows, setIsWindows] = useState(false)
    const [closeChoice, setCloseChoice] = useState(false)

    // The Windows tray context menu mirrors the Help menu: its items emit these events from the backend,
    // which open the same dialogs the in-window Help menu does. The close button emits close-request so
    // the choice of minimise-to-tray or quit uses the app's own themed dialog rather than a native one.
    useEffect(() => {
        const off = [
            EventsOn('menu:about', () => void showAbout()),
            EventsOn('menu:licence', () => void showLicence()),
            EventsOn('menu:check-updates', () => checkUpdates()),
            EventsOn('app:close-request', () => setCloseChoice(true)),
        ]
        return () => off.forEach((unsubscribe) => unsubscribe())
    }, [showAbout, showLicence, checkUpdates])

    // Detect Windows so the Mail menu can offer "Set as default for .eml", which deep-links to a Windows-only
    // settings page. The default is off, so the item stays hidden on other platforms.
    useEffect(() => {
        void Environment().then((env) => setIsWindows(env.platform === 'windows')).catch(() => undefined)
    }, [])

    // A .eml the OS handed to PigeonPost (double-clicked once PigeonPost is the registered handler) arrives as
    // an event from the backend, which has already revealed the window; show the parsed email in the in-app
    // viewer or surface a parse failure in the error bar rather than doing nothing.
    useEffect(() => {
        const off = [
            EventsOn('eml:open', (email) => setLaunchedEmail(email as EmailView)),
            EventsOn('eml:open-error', (message) => setError(String(message))),
        ]
        return () => off.forEach((unsubscribe) => unsubscribe())
    }, [])

    // The background mail poller emits mail:new when it brings in newly arrived messages (the same event
    // that raises the desktop notification), so refresh the unread counts and the folder on screen to show
    // the arrivals without waiting for the next on-screen sync.
    useEffect(() => {
        const off = EventsOn('mail:new', () => {
            void loadUnread()
            if (selectedFolder) {
                void reloadFolder(selectedFolder, {skipSync: true})
            }
        })
        return () => off()
    }, [selectedFolder, reloadFolder, loadUnread])

    // The poller emits calendar:changed after auto-applying an incoming meeting reply or cancellation, so
    // reload the events for the calendar view to reflect the updated attendee status or removed meeting.
    useEffect(() => {
        const off = EventsOn('calendar:changed', () => void loadEvents())
        return () => off()
    }, [loadEvents])

    return {launchedEmail, setLaunchedEmail, isWindows, closeChoice, setCloseChoice}
}
