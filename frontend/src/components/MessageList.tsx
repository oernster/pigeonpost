import {Message} from '../api'

interface MessageListProps {
    messages: Message[]
    selectedMessage: Message | null
    folderSelected: boolean
    onSelectMessage: (message: Message) => void
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
    const {messages, selectedMessage, folderSelected} = props

    if (!folderSelected) {
        return (
            <section className="pane message-list">
                <div className="empty-state"><p className="empty-body">Select a folder to see its messages.</p></div>
            </section>
        )
    }
    if (messages.length === 0) {
        return (
            <section className="pane message-list">
                <div className="empty-state"><p className="empty-body">No messages in this folder.</p></div>
            </section>
        )
    }

    return (
        <section className="pane message-list">
            <ul className="list">
                {messages.map((message) => (
                    <li
                        key={message.id}
                        className={
                            'message-row' +
                            (message.read ? '' : ' unread') +
                            (selectedMessage?.id === message.id ? ' selected' : '')
                        }
                        onClick={() => props.onSelectMessage(message)}
                    >
                        <div className="message-row-top">
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
        </section>
    )
}
