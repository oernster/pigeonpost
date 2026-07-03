import {Message} from '../api'

// messageDragType is the dataTransfer MIME type carrying a dragged message's id to a folder drop target.
export const messageDragType = 'application/x-pigeonpost-message'

interface MessageListProps {
    messages: Message[]
    selectedMessage: Message | null
    folderSelected: boolean
    searchQuery: string
    searchActive: boolean
    onSearchChange: (query: string) => void
    onSelectMessage: (message: Message) => void
    onToggleFlag: (message: Message) => void
    onContextMenu: (message: Message, x: number, y: number) => void
    onOpenInNewTab: (message: Message) => void
}

function formatDate(iso: string): string {
    if (!iso) {
        return ''
    }
    const date = new Date(iso)
    if (isNaN(date.getTime())) {
        return ''
    }
    return date.toLocaleString(undefined, {
        year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
    })
}

export function MessageList(props: MessageListProps) {
    const {messages, selectedMessage, folderSelected, searchQuery, searchActive} = props

    const content = () => {
        if (searchActive) {
            if (messages.length === 0) {
                return <div className="empty-state"><p className="empty-body">No messages match your search.</p></div>
            }
        } else if (!folderSelected) {
            return <div className="empty-state"><p className="empty-body">Select a folder to see its messages.</p></div>
        } else if (messages.length === 0) {
            return <div className="empty-state"><p className="empty-body">No messages in this folder.</p></div>
        }
        return (
            <ul className="list">
                {messages.map((message) => (
                    <li
                        key={message.id}
                        className={
                            'message-row' +
                            (message.read ? '' : ' unread') +
                            (selectedMessage?.id === message.id ? ' selected' : '')
                        }
                        tabIndex={selectedMessage?.id === message.id ? 0 : -1}
                        onClick={() => props.onSelectMessage(message)}
                        onDoubleClick={() => props.onOpenInNewTab(message)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                props.onOpenInNewTab(message)
                            }
                        }}
                        draggable
                        onDragStart={(e) => {
                            e.dataTransfer.setData(messageDragType, message.id)
                            e.dataTransfer.effectAllowed = 'move'
                        }}
                        onContextMenu={(e) => {
                            e.preventDefault()
                            props.onContextMenu(message, e.clientX, e.clientY)
                        }}
                    >
                        <div className="message-row-top">
                            <button
                                className={'message-star' + (message.flagged ? ' on' : '')}
                                aria-label={message.flagged ? 'Remove star' : 'Add star'}
                                aria-pressed={message.flagged}
                                title={message.flagged ? 'Starred' : 'Star'}
                                onClick={(e) => {
                                    e.stopPropagation()
                                    props.onToggleFlag(message)
                                }}
                            >
                                {message.flagged ? '★' : '☆'}
                            </button>
                            <span className="message-from">
                                {message.fromName || message.fromAddress || '(unknown sender)'}
                            </span>
                            <span className="message-date">{formatDate(message.date)}</span>
                        </div>
                        <div className="message-subject">
                            {message.hasAttachments && <span className="attach">{'\u{1F4CE}'}</span>}
                            {message.subject || '(no subject)'}
                        </div>
                        {message.snippet && <div className="message-snippet">{message.snippet}</div>}
                    </li>
                ))}
            </ul>
        )
    }

    return (
        <section className="pane message-list">
            <div className="message-search">
                <input
                    className="message-search-input"
                    value={searchQuery}
                    placeholder="Search mail…"
                    aria-label="Search mail"
                    onChange={(e) => props.onSearchChange(e.target.value)}
                />
                {searchQuery !== '' && (
                    <button className="message-search-clear" aria-label="Clear search" onClick={() => props.onSearchChange('')}>
                        &times;
                    </button>
                )}
            </div>
            <div className="message-list-scroll">{content()}</div>
        </section>
    )
}
