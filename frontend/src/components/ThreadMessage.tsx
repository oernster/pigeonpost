import {useEffect, useRef, useState} from 'react'
import {ConversationEntry, Message, MessageBody, api} from '../api'
import {MessageBodyView} from './MessageBodyView'
import {formatAddressList} from '../readerFormat'

interface ThreadMessageProps {
    entry: ConversationEntry
    // position is the message's place in the thread, shown so a long exchange can be talked about by
    // number rather than by date.
    position: number
    expanded: boolean
    onToggle: () => void
    // onOpen lifts this message out of the thread into a reader tab of its own, for the moment you want
    // its toolbar, its attachments and its full header rather than a read in place.
    onOpen: (message: Message) => void
    autoLoadImages: boolean
    dark: boolean
}

// who names the party a message is with: the sender for anything received, the first recipient for a
// message you sent, since every message of your own Sent folder would otherwise read as you.
function who(entry: ConversationEntry): string {
    const outgoing = entry.folderKind === 'sent' || entry.folderKind === 'drafts'
    const message = entry.message
    if (outgoing) {
        const to = message.to[0]
        return to ? to.name || to.address : 'you'
    }
    return message.fromName || message.fromAddress || '(unknown sender)'
}

// when renders a message's date in full: the thread view has the room for it, while a conversation
// spanning weeks is unreadable when every row shows a bare time.
function when(iso: string): string {
    const at = new Date(iso)
    return isNaN(at.getTime()) ? '' : at.toLocaleString()
}

// ThreadMessage is one message of the thread view: a header row that expands the message in place, then
// the body once it is open. The body is fetched the first time the message is expanded and kept
// afterwards, so collapsing and reopening costs nothing and a thread of twenty messages fetches only what
// was actually read.
export function ThreadMessage({entry, position, expanded, onToggle, onOpen, autoLoadImages, dark}: ThreadMessageProps) {
    const message = entry.message
    const [body, setBody] = useState<MessageBody | null>(null)
    const [loading, setLoading] = useState(false)
    // requested marks that this message's body has been asked for, so reopening a row it has already
    // read costs nothing and a failed fetch is not retried in a loop.
    const requested = useRef(false)

    // The fetch depends on the expansion and the message alone. Neither the body nor the loading flag may
    // appear here: each is set by this effect, so listing one re-runs the effect, whose cleanup then
    // flags the very fetch that set it as stale. That left a row reading "Loading message" for good,
    // because the completion that would have cleared the flag was discarded as belonging to a load the
    // row had moved on from. requested is what stops a refetch, so collapsing and reopening costs nothing.
    useEffect(() => {
        if (!expanded || requested.current) {
            return
        }
        requested.current = true
        let cancelled = false
        setLoading(true)
        void api.messageBody(message.id)
            .then((loaded) => {
                if (!cancelled) {
                    setBody(loaded)
                }
            })
            .catch(() => undefined)
            .finally(() => {
                if (!cancelled) {
                    setLoading(false)
                }
            })
        return () => {
            cancelled = true
        }
    }, [expanded, message.id])

    return (
        <li className={'thread-message' + (expanded ? ' expanded' : '') + (message.read ? '' : ' unread')}>
            <div className="thread-message-head">
                <button
                    type="button"
                    className="thread-message-toggle"
                    aria-expanded={expanded}
                    onClick={onToggle}
                >
                    <span className="thread-position" aria-hidden="true">{position}</span>
                    <span className="thread-who">{who(entry)}</span>
                    <span className="thread-folder">{entry.folderName}</span>
                    <span className="thread-when">{when(message.date)}</span>
                </button>
                <button
                    type="button"
                    className="thread-open btn"
                    title="Open this message in its own tab"
                    onClick={() => onOpen(message)}
                >
                    Open
                </button>
            </div>
            {expanded ? (
                <div className="thread-message-body">
                    {message.to.length > 0 && (
                        <div className="thread-recipients">
                            <span className="reader-label">To</span>
                            <span>{formatAddressList(message.to)}</span>
                        </div>
                    )}
                    <MessageBodyView
                        messageId={message.id}
                        body={body}
                        loading={loading}
                        snippet={message.snippet}
                        autoLoadImages={autoLoadImages}
                        dark={dark}
                    />
                </div>
            ) : (
                <p className="thread-snippet">{message.snippet}</p>
            )}
        </li>
    )
}
