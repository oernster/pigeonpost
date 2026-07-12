import {useEffect, useState} from 'react'
import {api} from '../api'

// PARKED_IMAGE_MARKER is the attribute a blocked remote image's source is parked in; its presence means the
// body has remote images the reader can choose to load. It matches the backend's data-pp-src wire name.
const PARKED_IMAGE_MARKER = 'data-pp-src='

export interface RemoteImages {
    // renderedHtml is the body to put in the frame: the proxy-resolved, image-inlined HTML once images are
    // shown and ready, otherwise the parked HTML, which loads no remote image.
    renderedHtml: string
    // loadingImages is true while the server-side resolve is in flight, for a brief loading hint.
    loadingImages: boolean
    // hasBlockedImages is true when the body carries at least one parked remote image.
    hasBlockedImages: boolean
}

// useRemoteImages resolves a message body's blocked remote images on demand. While images are hidden it
// returns the parked HTML unchanged. Once imagesShown is set and the body has parked images, it fetches them
// through the server-side proxy (api.loadRemoteImages), which returns the HTML with each fetched image inlined
// as a data: URI, and renders that instead. The fetch runs once per distinct body; a failure falls back to the
// parked HTML, so a broken fetch never loops or shows a remote image. A change of body resets the state, so a
// previous message's images are never shown against a new one. It is shared by the reader and the attached-.eml
// viewer, which drive it identically.
export function useRemoteImages(rawHtml: string, imagesShown: boolean): RemoteImages {
    const [resolvedHtml, setResolvedHtml] = useState<string | null>(null)
    const [loadingImages, setLoadingImages] = useState(false)
    const hasBlockedImages = rawHtml.includes(PARKED_IMAGE_MARKER)

    useEffect(() => {
        setResolvedHtml(null)
        setLoadingImages(false)
    }, [rawHtml])

    // loadingImages is deliberately neither a guard nor a dependency here: it is set inside the effect, so
    // depending on it would re-run the effect, whose cleanup would cancel its own in-flight fetch before it
    // could store the result. The resolvedHtml !== null guard already stops a refetch once one has completed,
    // and no other dependency changes while a fetch is in flight, so a single fetch runs per body.
    useEffect(() => {
        if (!imagesShown || !hasBlockedImages || resolvedHtml !== null) {
            return
        }
        let cancelled = false
        setLoadingImages(true)
        void api.loadRemoteImages(rawHtml)
            .catch(() => rawHtml)
            .then((html) => {
                if (!cancelled) {
                    setResolvedHtml(html)
                    setLoadingImages(false)
                }
            })
        return () => {
            cancelled = true
        }
    }, [imagesShown, hasBlockedImages, resolvedHtml, rawHtml])

    const renderedHtml = imagesShown && resolvedHtml ? resolvedHtml : rawHtml
    return {renderedHtml, loadingImages, hasBlockedImages}
}
