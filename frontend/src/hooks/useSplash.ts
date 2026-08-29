import {useEffect, useState} from 'react'
import {api} from '../api'

// useSplash owns the launch splash: the identity it names and how long it stays. It runs once, on mount.
//
// The version and author are read for the splash alone; About asks the backend for its own fuller
// answer. Neither read is worth interrupting a launch over, so a failure is swallowed and the splash
// simply carries no line for it rather than raising an error before the window has settled.
//
// The splash fades before it goes, so the two timers are one gesture: SPLASH_FADE_MS starts the fade and
// SPLASH_HIDE_MS removes it, the gap between them being the length of the fade itself. Both are cleared
// on unmount so a splash that outlives its component cannot set state on it.
const SPLASH_FADE_MS = 1600
const SPLASH_HIDE_MS = 2000

export function useSplash() {
    const [version, setVersion] = useState<string>('')
    const [author, setAuthor] = useState<string>('')
    const [visible, setVisible] = useState<boolean>(true)
    const [fading, setFading] = useState<boolean>(false)

    useEffect(() => {
        void api.version().then(setVersion).catch(() => undefined)
        void api.author().then(setAuthor).catch(() => undefined)
        const fade = window.setTimeout(() => setFading(true), SPLASH_FADE_MS)
        const hide = window.setTimeout(() => setVisible(false), SPLASH_HIDE_MS)
        return () => {
            window.clearTimeout(fade)
            window.clearTimeout(hide)
        }
    }, [])

    return {version, author, visible, fading}
}
