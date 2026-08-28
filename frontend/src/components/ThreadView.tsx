import {useEffect, useState} from 'react'
import {Message} from '../api'
import {ThreadMessage} from './ThreadMessage'
import {useConversation} from '../hooks/useConversation'

interface ThreadViewProps {
    // headMessageId is any message of the conversation; the whole thread is looked up from it.
    headMessageId: string
    // subject labels the thread while its messages are still loading, so the pane is never blank.
    subject: string
    onClose: () => void
    onOpenMessage: (message: Message) => void
    autoLoadImages: boolean
    dark: boolean
}

// ThreadView shows one conversation whole: every cached message that threads with the one the list was
// asked to open, gathered across the account's folders and ordered oldest first, each expandable to read
// in place. It answers what the reading list cannot: the list groups a conversation within one folder, so
// the reply you sent is in Sent and never appears beside the message it answers. Here the two halves of
// an exchange sit together, each row naming the folder its message lives in.
export function ThreadView({headMessageId, subject, onClose, onOpenMessage, autoLoadImages, dark}: ThreadViewProps) {
    const entries = useConversation(headMessageId)
    // expandedIds are the messages open for reading. The newest is opened for you when the thread arrives,
    // because that is the message the conversation has most recently arrived at; the rest wait to be asked
    // for. A thread you have opened before reopens with only its newest message expanded, deliberately: a
    // remembered set of open messages would restore a wall of text rather than a conversation.
    const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())

    useEffect(() => {
        if (entries.length === 0) {
            setExpandedIds(new Set())
            return
        }
        setExpandedIds(new Set([entries[entries.length - 1].message.id]))
    }, [headMessageId, entries])

    const toggle = (id: string) => {
        setExpandedIds((open) => {
            const next = new Set(open)
            if (next.has(id)) {
                next.delete(id)
            } else {
                next.add(id)
            }
            return next
        })
    }

    const latestSubject = entries.length > 0 ? entries[entries.length - 1].message.subject : subject
    return (
        <section className="pane reader thread-view">
            <div className="thread-header">
                <button type="button" className="btn" onClick={onClose}>Back</button>
                <div className="thread-title">
                    <span className="thread-subject">{latestSubject || '(no subject)'}</span>
                    <span className="thread-count">
                        {entries.length === 0 ? 'Gathering the conversation…' : `${entries.length} messages`}
                    </span>
                </div>
            </div>
            <div className="thread-scroll">
                {entries.length === 0 ? (
                    <p className="empty-body">This conversation has no other messages in the cache.</p>
                ) : (
                    <ul className="thread-list">
                        {entries.map((entry, index) => (
                            <ThreadMessage
                                key={entry.message.id}
                                entry={entry}
                                position={index + 1}
                                expanded={expandedIds.has(entry.message.id)}
                                onToggle={() => toggle(entry.message.id)}
                                onOpen={onOpenMessage}
                                autoLoadImages={autoLoadImages}
                                dark={dark}
                            />
                        ))}
                    </ul>
                )}
            </div>
        </section>
    )
}
