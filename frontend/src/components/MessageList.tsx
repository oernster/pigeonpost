import {useEffect, useMemo, useRef, type CSSProperties, type ReactNode, type RefObject} from 'react'
import {useVirtualizer, VirtualItem} from '@tanstack/react-virtual'
import {Message, SEARCH_MATCH_END, SEARCH_MATCH_START} from '../api'
import {ConversationHead} from '../threads'
import type {AccountChip} from '../unified'

// SearchScope is the search bar's reach: every account, the selected folder or the selected account.
export type SearchScope = 'all' | 'folder' | 'account'

// messageDragType is the dataTransfer MIME type carrying a dragged message's id to a folder drop target.
// When several messages are selected, dropping any one of them moves the whole selection; the parent
// expands the dragged id to the full selection, so a single id on the drag is enough.
export const messageDragType = 'application/x-pigeonpost-message'

// REPLIED_GLYPH and FORWARDED_GLYPH are the small arrows shown at the top-left of a row once the message has
// been replied to (\Answered) or forwarded ($Forwarded). The FE0E text-presentation selector keeps them as
// flat monochrome glyphs matching the dimmed attachment clip rather than a colourful emoji on some platforms.
const REPLIED_GLYPH = '\u{21A9}\u{FE0E}'
const FORWARDED_GLYPH = '\u{21AA}\u{FE0E}'

// The list is virtualized: only the rows on screen are in the DOM, so a folder of tens of thousands of
// messages renders without freezing. These size hints seed the scrollbar before a row's real height is
// measured (a message row is one to three lines; a conversation header is a single line) and OVERSCAN
// keeps a few rows rendered just past the viewport so keyboard navigation and scrolling stay smooth.
const MESSAGE_ROW_ESTIMATE = 64
const CONVERSATION_HEADER_ESTIMATE = 26
const ROW_OVERSCAN = 8
// LOAD_MORE_ROW_THRESHOLD is how close to the last loaded row the viewport must reach before the next
// page is requested, so the following page is in flight before the user hits the end.
const LOAD_MORE_ROW_THRESHOLD = 12

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
    // The search scope selector: its current value, the change handler and whether the folder and
    // account scopes have a selection to bind to (a scope without one is disabled, not silently global).
    searchScope: SearchScope
    onScopeChange: (scope: SearchScope) => void
    canScopeFolder: boolean
    canScopeAccount: boolean
    // searchDegraded reports that the query text failed structural parsing and was searched as plain
    // text, so the bar can hint that operators were ignored.
    searchDegraded: boolean
    // matchSnippets maps a message id to its matched-text snippet (terms wrapped in the match markers).
    // A row with an entry shows it, highlighted, in place of the stored preview snippet.
    matchSnippets: Map<string, string>
    // accountChips labels a row carrying an accountId (a unified-mailbox row) with its account's colour
    // dot and email tooltip. Rows without an accountId (every per-folder listing) show nothing.
    accountChips: Map<string, AccountChip>
    // searchInputRef lets Edit > Search (Ctrl+K) focus the search box from outside the list.
    searchInputRef: RefObject<HTMLInputElement>
    onActivate: (message: Message, mods: ClickMods) => void
    onClearSelection: () => void
    onToggleFlag: (message: Message) => void
    onContextMenu: (message: Message, x: number, y: number) => void
    onOpenInNewTab: (message: Message, fromKeyboard?: boolean) => void
    // onLoadMore is called as the viewport nears the last loaded row, so the flat view can fetch and
    // append the next page. The parent guards it (it is a no-op in conversation view, in search or when
    // there is no more to load or a load is already running), so it is safe to call freely.
    onLoadMore: () => void
    // sortAscending is the current date order of the list (false is newest first); onToggleSort flips it.
    sortAscending: boolean
    onToggleSort: () => void
}

// ListRow is one rendered row: a conversation header (the label above the first message of a multi-message
// conversation) or a message. The two are flattened into a single sequence so the virtualizer can measure
// and position them together.
type ListRow =
    | {kind: 'header'; key: string; head: ConversationHead}
    | {kind: 'message'; key: string; message: Message}

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

// escapeId makes a message id safe to embed in an attribute selector (message ids can carry characters
// that are special in CSS), falling back to the raw id where CSS.escape is unavailable.
function escapeId(id: string): string {
    return typeof CSS !== 'undefined' && CSS.escape ? CSS.escape(id) : id
}

// renderMarkedSnippet turns a backend match snippet into text nodes with each matched run wrapped in
// <mark>. It only ever splits on the two control-character markers, so message content is rendered as
// plain text and never interpreted as markup.
function renderMarkedSnippet(marked: string): ReactNode[] {
    const out: ReactNode[] = []
    const chunks = marked.split(SEARCH_MATCH_START)
    out.push(chunks[0])
    for (let i = 1; i < chunks.length; i++) {
        const end = chunks[i].indexOf(SEARCH_MATCH_END)
        if (end < 0) {
            out.push(chunks[i])
            continue
        }
        out.push(<mark key={i}>{chunks[i].slice(0, end)}</mark>)
        out.push(chunks[i].slice(end + SEARCH_MATCH_END.length))
    }
    return out
}

// stripMarkers removes the match markers for plain-text uses of a match snippet (the row tooltip).
function stripMarkers(marked: string): string {
    return marked.split(SEARCH_MATCH_START).join('').split(SEARCH_MATCH_END).join('')
}

export function MessageList(props: MessageListProps) {
    const {messages, selectedIds, activeId, folderSelected, searchQuery, searchActive} = props
    const selectionCount = selectedIds.size
    const scrollRef = useRef<HTMLDivElement>(null)

    // Flatten the visible list into the exact row sequence the list renders: a conversation-header row
    // precedes the first message of each multi-message conversation, then the message row itself.
    const rows = useMemo<ListRow[]>(() => {
        const out: ListRow[] = []
        for (const message of messages) {
            const head = props.conversationHeads.get(message.id)
            if (head) {
                out.push({kind: 'header', key: 'header:' + message.id, head})
            }
            out.push({kind: 'message', key: message.id, message})
        }
        return out
    }, [messages, props.conversationHeads])

    // messageIndex maps a message id to its row position, so keyboard navigation can scroll a row that is
    // not currently mounted into view before focusing it. A ref holds the latest map so the scroll effect
    // can read it without listing it as a dependency (which would re-scroll whenever the list changes, for
    // example when a page is appended).
    const messageIndex = useMemo(() => {
        const map = new Map<string, number>()
        rows.forEach((row, index) => {
            if (row.kind === 'message') {
                map.set(row.message.id, index)
            }
        })
        return map
    }, [rows])
    const messageIndexRef = useRef(messageIndex)
    messageIndexRef.current = messageIndex

    const virtualizer = useVirtualizer({
        count: rows.length,
        getScrollElement: () => scrollRef.current,
        estimateSize: (index) => (rows[index].kind === 'header' ? CONVERSATION_HEADER_ESTIMATE : MESSAGE_ROW_ESTIMATE),
        getItemKey: (index) => rows[index].key,
        overscan: ROW_OVERSCAN,
    })

    const virtualItems = virtualizer.getVirtualItems()

    // When the active row changes (a click or a keyboard move) scroll it into view. With virtualization the
    // target row may be unmounted, so focus() alone cannot reach it; scrolling mounts it, then focus lands
    // on it once it is in the DOM so Enter, Space and the roving tabindex work. Keyed on activeId alone: a
    // list change (an appended page) must not yank the scroll back to the active row.
    useEffect(() => {
        if (activeId == null) {
            return
        }
        const index = messageIndexRef.current.get(activeId)
        if (index == null) {
            return
        }
        virtualizer.scrollToIndex(index, {align: 'auto'})
        const raf = requestAnimationFrame(() => {
            // Only take focus from a neutral spot (the start sink or the body, both tabindex -1) or another
            // row, mirroring the window key handler, so focus already moved into the reader or a control is
            // not yanked back to the list.
            const active = document.activeElement as HTMLElement | null
            if (active && active.tabIndex >= 0 && !active.classList.contains('message-row')) {
                return
            }
            scrollRef.current?.querySelector<HTMLElement>(`[data-mid="${escapeId(activeId)}"]`)?.focus()
        })
        return () => cancelAnimationFrame(raf)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [activeId])

    // Request the next page as the viewport nears the last loaded row. onLoadMore is guarded by the parent,
    // so calling it on every approach to the end is safe.
    const lastIndex = virtualItems.length > 0 ? virtualItems[virtualItems.length - 1].index : -1
    useEffect(() => {
        if (lastIndex >= 0 && lastIndex >= rows.length - LOAD_MORE_ROW_THRESHOLD) {
            props.onLoadMore()
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [lastIndex, rows.length])

    // The message list is a single focus-ring stop: the roving tabbable row (tabindex 0) must be one that is
    // actually rendered, so Tab from the search box always reaches a row in the DOM. It is the active row
    // when that row is on screen, otherwise the first rendered row.
    const renderedMessageIds = virtualItems
        .map((item) => rows[item.index])
        .filter((row): row is Extract<ListRow, {kind: 'message'}> => row.kind === 'message')
        .map((row) => row.message.id)
    const tabStopId = activeId && renderedMessageIds.includes(activeId)
        ? activeId
        : (renderedMessageIds[0] ?? null)

    const emptyState = () => {
        if (searchActive) {
            return messages.length === 0
                ? <div className="empty-state"><p className="empty-body">No messages match your search.</p></div>
                : null
        }
        if (!folderSelected) {
            return <div className="empty-state"><p className="empty-body">Select a folder to see its messages.</p></div>
        }
        if (messages.length === 0) {
            return <div className="empty-state"><p className="empty-body">No messages in this folder.</p></div>
        }
        return null
    }

    const renderRow = (item: VirtualItem) => {
        const row = rows[item.index]
        const index = item.index
        const style: CSSProperties = {
            position: 'absolute',
            top: 0,
            left: 0,
            width: '100%',
            transform: `translateY(${item.start}px)`,
        }
        if (row.kind === 'header') {
            const {head} = row
            return (
                <li
                    key={row.key}
                    data-index={index}
                    ref={virtualizer.measureElement}
                    className="conversation-header"
                    aria-hidden="true"
                    style={style}
                >
                    <span className="conversation-subject" title={head.subject || '(no subject)'}>{head.subject || '(no subject)'}</span>
                    <span className="conversation-count">{head.count} messages</span>
                </li>
            )
        }
        const message = row.message
        const matchSnippet = props.matchSnippets.get(message.id)
        const accountChip = message.accountId ? props.accountChips.get(message.accountId) : undefined
        return (
            <li
                key={row.key}
                data-index={index}
                ref={virtualizer.measureElement}
                className={
                    'message-row' +
                    (message.read ? '' : ' unread') +
                    (selectedIds.has(message.id) ? ' selected' : '') +
                    (activeId === message.id ? ' active' : '')
                }
                data-mid={message.id}
                tabIndex={tabStopId === message.id ? 0 : -1}
                aria-selected={selectedIds.has(message.id)}
                style={style}
                // Shift-click would otherwise select the page text across the rows it spans; suppress
                // that here so a range selection stays a message selection.
                onMouseDown={(e) => {
                    if (e.shiftKey) {
                        e.preventDefault()
                    }
                }}
                onClick={(e) => props.onActivate(message, {ctrl: e.ctrlKey || e.metaKey, shift: e.shiftKey})}
                onDoubleClick={() => props.onOpenInNewTab(message, false)}
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
                    {message.answered && (
                        <span className="replied" title="Replied" aria-label="Replied">{REPLIED_GLYPH}</span>
                    )}
                    {message.forwarded && (
                        <span className="forwarded" title="Forwarded" aria-label="Forwarded">{FORWARDED_GLYPH}</span>
                    )}
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
                    {accountChip && (
                        <span
                            className="account-dot"
                            style={{backgroundColor: accountChip.colour}}
                            title={accountChip.label}
                            aria-label={`Account ${accountChip.label}`}
                        />
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
                {matchSnippet ? (
                    <div className="message-snippet" title={stripMarkers(matchSnippet)}>
                        {renderMarkedSnippet(matchSnippet)}
                    </div>
                ) : (
                    message.snippet && <div className="message-snippet" title={message.snippet}>{message.snippet}</div>
                )}
            </li>
        )
    }

    const empty = emptyState()

    return (
        <section className="pane message-list">
            <div className="message-search">
                <div className="message-search-box">
                    <input
                        ref={props.searchInputRef}
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
                <select
                    className="message-search-scope"
                    aria-label="Search scope"
                    value={props.searchScope}
                    onChange={(e) => props.onScopeChange(e.target.value as SearchScope)}
                >
                    <option value="all">All mail</option>
                    <option value="folder" disabled={!props.canScopeFolder}>This folder</option>
                    <option value="account" disabled={!props.canScopeAccount}>This account</option>
                </select>
            </div>
            {searchActive && props.searchDegraded && (
                <div className="message-search-hint" role="status">Searched as plain text</div>
            )}
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
            <div className="message-list-scroll" ref={scrollRef}>
                {empty ?? (
                    <ul className="list" style={{position: 'relative', height: virtualizer.getTotalSize()}}>
                        {virtualItems.map((item) => renderRow(item))}
                    </ul>
                )}
            </div>
        </section>
    )
}
