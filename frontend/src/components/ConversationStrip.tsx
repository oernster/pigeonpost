import {ConversationEntry, Message} from '../api'

interface ConversationStripProps {
    // entries are the whole thread, oldest first, including the message being read.
    entries: ConversationEntry[]
    // currentId marks which entry is the open message: it is shown as the one you are on rather than as
    // somewhere to go.
    currentId: string
    onOpen: (message: Message) => void
}

// A thread of one is just the message you are reading, so the strip stays out of the way until there is
// something to move between.
const MIN_ENTRIES = 2

// formatWhen renders an entry's date compactly: the day and month with the time, which is enough to tell
// the messages of a thread apart without repeating the full date on every row.
function formatWhen(iso: string): string {
    const at = new Date(iso)
    if (isNaN(at.getTime())) {
        return ''
    }
    return at.toLocaleString(undefined, {day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit'})
}

// entryWho names who the message is with: the sender for anything received, the first recipient for a
// message you sent, since every row of your own Sent folder would otherwise read as you.
function entryWho(entry: ConversationEntry): string {
    const outgoing = entry.folderKind === 'sent' || entry.folderKind === 'drafts'
    const message = entry.message
    if (outgoing) {
        const to = message.to[0]
        return to ? to.name || to.address : 'you'
    }
    return message.fromName || message.fromAddress
}

// ConversationStrip lists the messages of the open message's thread and opens any of them in a reader
// tab. It answers the question a grouped list raises but cannot settle: a conversation view shows that a
// message has replies, while the replies themselves live in other folders. Each row names where its
// message sits, so the received message and the answer sent back read as the two halves they are.
export function ConversationStrip(props: ConversationStripProps) {
    if (props.entries.length < MIN_ENTRIES) {
        return null
    }
    return (
        <div className="conversation-strip">
            <div className="conversation-head">{`Conversation (${props.entries.length})`}</div>
            <ul className="conversation-list">
                {props.entries.map((entry, index) => {
                    const current = entry.message.id === props.currentId
                    return (
                        <li key={entry.message.id}>
                            <button
                                type="button"
                                className={'conversation-item' + (current ? ' current' : '')}
                                aria-current={current ? 'true' : undefined}
                                disabled={current}
                                onClick={() => props.onOpen(entry.message)}
                            >
                                <span className="conversation-index" aria-hidden="true">{index + 1}</span>
                                <span className="conversation-who">{entryWho(entry)}</span>
                                <span className="conversation-folder">{entry.folderName}</span>
                                <span className="conversation-when">{formatWhen(entry.message.date)}</span>
                            </button>
                        </li>
                    )
                })}
            </ul>
        </div>
    )
}
