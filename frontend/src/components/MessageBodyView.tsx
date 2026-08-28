import {useEffect, useState} from 'react'
import {MessageBody} from '../api'
import {EmailHtmlFrame} from './EmailHtmlFrame'
import {LinkifiedText} from './LinkifiedText'
import {InviteCard} from './InviteCard'
import {useRemoteImages} from '../hooks/useRemoteImages'
import {openLinkExternally} from '../openLink'

interface MessageBodyViewProps {
    // messageId identifies the message being rendered: it keys the remote-image reset and is what the
    // invite card asks about.
    messageId: string
    body: MessageBody | null
    loading: boolean
    // snippet is the stored preview, shown when a message has no body text of its own.
    snippet: string
    autoLoadImages: boolean
    dark: boolean
}

// MessageBodyView renders one message's content: its invite card when it carries a scheduling payload,
// then its HTML behind the remote-image guard, else its plain text, else its stored snippet. It is the
// single home for that decision, shared by the reader and by each expanded message of the thread view, so
// a message reads the same wherever it is opened. It renders content only; the scrolling, the toolbar and
// the pinned attachment footer belong to the surface around it.
export function MessageBodyView({messageId, body, loading, snippet, autoLoadImages, dark}: MessageBodyViewProps) {
    const [imagesShown, setImagesShown] = useState(autoLoadImages)
    const {renderedHtml, loadingImages, hasBlockedImages} = useRemoteImages(body?.html ?? '', imagesShown)

    // Reset a new message's images to the current auto-load setting: shown at once when auto-load is on,
    // otherwise re-blocked behind the Load images bar. Toggling the setting re-applies it to the message
    // on screen too.
    useEffect(() => {
        setImagesShown(autoLoadImages)
    }, [messageId, autoLoadImages])

    if (loading) {
        return <p className="empty-body">Loading message…</p>
    }
    if (body && body.html.trim() !== '') {
        return (
            <>
                {body.hasInvite && <InviteCard messageId={messageId}/>}
                {hasBlockedImages && !imagesShown && (
                    <div className="images-blocked-bar">
                        <span>Remote images were not loaded to protect your privacy.</span>
                        <button className="btn" onClick={() => setImagesShown(true)}>Load images</button>
                    </div>
                )}
                {hasBlockedImages && imagesShown && loadingImages && (
                    <div className="images-blocked-bar">
                        <span>Loading images…</span>
                    </div>
                )}
                <EmailHtmlFrame html={renderedHtml} dark={dark} onOpenLink={openLinkExternally}/>
            </>
        )
    }
    if (body && body.plain.trim() !== '') {
        return (
            <>
                {body.hasInvite && <InviteCard messageId={messageId}/>}
                <pre className="reader-text">
                    <LinkifiedText text={body.plain} onOpenLink={openLinkExternally}/>
                </pre>
            </>
        )
    }
    return (
        <>
            {body?.hasInvite && <InviteCard messageId={messageId}/>}
            {snippet ? <p>{snippet}</p> : <p className="empty-body">This message has no text content.</p>}
        </>
    )
}
