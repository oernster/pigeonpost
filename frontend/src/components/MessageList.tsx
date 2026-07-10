import {Fragment} from 'react'
import {Message} from '../api'
import {ConversationHead} from '../threads'

// messageDragType is the dataTransfer MIME type carrying a dragged message's id to a folder drop target.
// When several messages are selected, dropping any one of them moves the whole selection; the parent
// expands the dragged id to the full selection, so a single id on the drag is enough.
export const messageDragType = 'application/x-pigeonpost-message'

// ClickMods carries the modifier keys of a row click so the parent can apply the standard selection
// gestures: plain click selects one, Ctrl (or Cmd) toggles a row in or out, Shift selects a range.
export interface ClickMods {
    ctrl: boolean
    shift: boolean
}

interface MessageListProps {
    messages: Message[]
    // conversationHeads labels the first row of each multi-message conversation, keyed by that row's id.
    // Empty when the conversation view is off, so the list renders flat.
    conversationHeads: Map<string, ConversationHead>
    // selectedIds is every highlighted row; activeId is the one shown in the reader (the range anchor and
    // the roving-tabindex target). With a single selection the two coincide.
    selectedIds: Set<string>
    activeId: string | null
    folderSelected: boolean
    searchQuery: string
    searchActive: boolean
    onSearchChange: (query: string) => void
    onActivate: (message: Message, mods: ClickMods) => void
    onClearSelection: () => void
    onToggleFlag: (message: Message) => void
    onContextMenu: (message: Message, x: number, y: number) => void
    onOpenInNewTab: (message: Message) => void
    // sortAscending is the current date order of the list (false is newest first); onToggleSort flips it.
    sortAscending: boolean
    onToggleSort: () => void
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
    const {messages, selectedIds, activeId, folderSelected, searchQuery, searchActive} = props
    const selectionCount = selectedIds.size

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
                {messages.map((message) => {
                    const head = props.conversationHeads.get(message.id)
                    return (
                    <Fragment key={message.id}>
                    {head && (
                        <li className="conversation-header" aria-hidden="true">
                            <span className="conversation-subject" title={head.subject || '(no subject)'}>{head.subject || '(no subject)'}</span>
                            <span className="conversation-count">{head.count} messages</span>
                        </li>
                    )}
                    <li
                        className={
                            'message-row' +
                            (message.read ? '' : ' unread') +
                            (selectedIds.has(message.id) ? ' selected' : '') +
                            (activeId === message.id ? ' active' : '')
                        }
                        data-mid={message.id}
                        tabIndex={activeId === message.id ? 0 : -1}
                        aria-selected={selectedIds.has(message.id)}
                        // Shift-click would otherwise select the page text across the rows it spans; suppress
                        // that here so a range selection stays a message selection.
                        onMouseDown={(e) => {
                            if (e.shiftKey) {
                                e.preventDefault()
                            }
                        }}
                        onClick={(e) => props.onActivate(message, {ctrl: e.ctrlKey || e.metaKey, shift: e.shiftKey})}
                        onDoubleClick={() => props.onOpenInNewTab(message)}
                        onKeyDown={(e) => {
                            // Enter or Space opens the message in a tab. Ctrl or Shift with Space are left to
                            // the window handler, which uses them to build a multi-selection.
                            if ((e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') &&
                                !e.ctrlKey && !e.metaKey && !e.shiftKey) {
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
                            {message.hasAttachments && (
                                <span className="attach" title="Has attachments" aria-label="Has attachments">
                                    {'\u{1F4CE}'}
                                </span>
                            )}
                            <span className="message-from" title={message.fromName || message.fromAddress || '(unknown sender)'}>
                                {message.fromName || message.fromAddress || '(unknown sender)'}
                            </span>
                            {message.tagColours.length > 0 && (
                                <span className="message-tags" aria-hidden="true">
                                    {message.tagColours.map((colour, i) => (
                                        <span key={i} className="message-tag-dot" style={{backgroundColor: colour}}/>
                                    ))}
                                </span>
                            )}
                            <span className="message-date">{formatDate(message.date)}</span>
                        </div>
                        <div className="message-subject">
                            {message.subject || '(no subject)'}
                        </div>
                        {message.snippet && <div className="message-snippet" title={message.snippet}>{message.snippet}</div>}
                    </li>
                    </Fragment>
                    )
                })}
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
            {selectionCount > 1 && (
                <div className="selection-bar" role="status">
                    <span className="selection-count">{selectionCount} selected</span>
                    <button className="selection-clear" onClick={props.onClearSelection}>Clear</button>
                </div>
            )}
            {folderSelected && !searchActive && messages.length > 0 && (
                <div className="list-sort-bar">
                    <button
                        className="list-sort-btn"
                        onClick={props.onToggleSort}
                        aria-label={`Sort by date, ${props.sortAscending ? 'oldest first' : 'newest first'}`}
                        title={props.sortAscending ? 'Oldest first (click for newest first)' : 'Newest first (click for oldest first)'}
                    >
                        Date {props.sortAscending ? '▲' : '▼'}
                    </button>
                </div>
            )}
            <div className="message-list-scroll">{content()}</div>
        </section>
    )
}
