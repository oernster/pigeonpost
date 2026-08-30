import {useCallback, useEffect, useState} from 'react'
import {api, UpdateStatus} from '../api'

// The localStorage key holding the release the user chose to skip. Stored front-side with the
// other presentation preferences (conversationView, previewEnabled); the backend stays stateless
// about it and receives the skipped tag with each automatic check.
export const SKIPPED_UPDATE_KEY = 'skippedUpdateVersion'

// Delay before the automatic launch check, so startup is never contended.
const LAUNCH_CHECK_DELAY_MS = 3000
// One re-check per day while the app stays running.
const CHECK_INTERVAL_HOURS = 24
const MS_PER_HOUR = 60 * 60 * 1000

// Owns the update-check triggers and the resulting modal state. An automatic check (shortly after
// launch, then daily) surfaces the modal only when a newer, non-skipped release exists; the manual
// Help > Check for Updates ignores the skip and surfaces every outcome, up to date and unreachable
// included. The backend reports failure as a status rather than an error, so automatic checks stay
// silent without special casing here.
export function useUpdateCheck() {
    const [updateStatus, setUpdateStatus] = useState<UpdateStatus | null>(null)

    const runCheck = useCallback(async (manual: boolean) => {
        try {
            const skipped = manual ? '' : (localStorage.getItem(SKIPPED_UPDATE_KEY) ?? '')
            const status = await api.checkForUpdates(skipped)
            if (status.updateAvailable || manual) {
                setUpdateStatus(status)
            }
        } catch {
            // A missing binding or a rejected call must never surface from an automatic check.
        }
    }, [])

    const checkUpdates = useCallback(() => {
        void runCheck(true)
    }, [runCheck])

    useEffect(() => {
        const launch = window.setTimeout(() => void runCheck(false), LAUNCH_CHECK_DELAY_MS)
        const periodic = window.setInterval(() => void runCheck(false), CHECK_INTERVAL_HOURS * MS_PER_HOUR)
        return () => {
            window.clearTimeout(launch)
            window.clearInterval(periodic)
        }
    }, [runCheck])

    const skipUpdate = useCallback((version: string) => {
        localStorage.setItem(SKIPPED_UPDATE_KEY, version)
    }, [])

    return {updateStatus, setUpdateStatus, checkUpdates, skipUpdate}
}
