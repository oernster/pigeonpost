import {useEffect, useRef, useState} from 'react'
import type {MouseEvent as ReactMouseEvent} from 'react'
import {api, Folder, Message, MessageBody, Tag} from '../api'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {isOutboxMessage} from '../outbox'
import {ReaderTabs} from './ReaderTabs'
import {InviteCard} from './InviteCard'

// handleBodyClick opens links from rendered message HTML in the external browser rather than letting
// them navigate the app's own webview. The HTML is sanitised server-side, so anchors are safe.
function handleBodyClick(e: ReactMouseEvent<HTMLDivElement>) {
    const anchor = (e.target as HTMLElement).closest('a')
    if (anchor) {
        e.preventDefault()
        const href = anchor.getAttribute('href')
        if (href) {
            void api.openExternal(href)
        }
    }
}

// formatAddress renders one correspondent as "Name <address>", or just the address when it has no name.
function formatAddress(a: {name: string; address: string}): string {
    return a.name ? `${a.name} <${a.address}>` : a.address
}

// formatAddressList joins a recipient list for display, dropping any empty entries.
function formatAddressList(list: {name: string; address: string}[]): string {
    return list.map(formatAddress).filter(Boolean).join(', ')
}

interface ReaderProps {
    message: Message | null
    onToggleRead: (message: Message) => void
    onReply: (message: Message) => void
    onReplyAll: (message: Message) => void
    onForward: (message: Message) => void
    onDelete: (message: Message) => void
    // onCancelSend discards a queued outbox item; an outbox message shows only this action.
    onCancelSend: (message: Message) => void
    folders: Folder[]
    onMove: (message: Message, destFolderId: string) => void
    onCopy: (message: Message, destFolderId: string) => void
    // canMoveCopy is false for POP3 accounts, which have a single inbox and no server-side move/copy.
    canMoveCopy: boolean
    tags: Tag[]
    messageTags: Tag[]
    onToggleTag: (tagId: string, assigned: boolean) => void
    body: MessageBody | null
    bodyLoading: boolean
    tabs: Message[]
    onSelectTab: (message: Message) => void
    onCloseTab: (id: string) => void
    // onBack is set only when the reader is shown full-width (reading pane off); it returns to the list.
    onBack?: () => void
}

export function Reader({message, onToggleRead, onReply, onReplyAll, onForward, onDelete, onCancelSend, folders, onMove, onCopy, canMoveCopy, messageTags, onToggleTag, body, bodyLoading, tabs, onSelectTab, onCloseTab, onBack}: ReaderProps) {
    const [tagMenuOpen, setTagMenuOpen] = useState(false)
    const [imagesShown, setImagesShown] = useState(false)
    const [attachError, setAttachError] = useState('')
    const menuRef = useRef<HTMLDivElement>(null)

    // saveAttachment writes a received attachment to disk through a native save dialog; its bytes come
    // from the locally cached body, so it works offline once the message has been opened.
    const saveAttachment = async (index: number) => {
        if (!message) return
        setAttachError('')
        try {
            await api.saveAttachment(message.id, index)
        } catch (e) {
            setAttachError(String(e))
        }
    }

    const tabStrip = tabs.length > 0
        ? <ReaderTabs tabs={tabs} activeMessageId={message?.id ?? ''} onSelectTab={onSelectTab} onCloseTab={onCloseTab}/>
        : null

    useEffect(() => {
        if (!tagMenuOpen) {
            return
        }
        const onDocClick = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
                setTagMenuOpen(false)
            }
        }
        document.addEventListener('mousedown', onDocClick)
        return () => document.removeEventListener('mousedown', onDocClick)
    }, [tagMenuOpen])

    // Close the tag menu and re-block images whenever the selected message changes.
    useEffect(() => {
        setTagMenuOpen(false)
        setImagesShown(false)
        setAttachError('')
    }, [message?.id])

    if (!message) {
        return (
            <section className="pane reader">
                {tabStrip}
                <div className="empty-state"><p className="empty-body">Select a message to read.</p></div>
            </section>
        )
    }

    const outbox = isOutboxMessage(message)
    const sender = message.fromName
        ? `${message.fromName} <${message.fromAddress}>`
        : message.fromAddress || '(unknown sender)'
    const recipients = message.to.map((a) => a.address).filter(Boolean).join(', ')

    const assigned = new Set(messageTags.map((t) => t.id))

    // Remote images are parked in data-pp-src at fetch time so they do not load automatically. When the
    // reader asks to load them, restore the src; the block resets when the message changes.
    const rawHtml = body?.html ?? ''
    const hasBlockedImages = rawHtml.includes('data-pp-src=')
    const renderedHtml = imagesShown ? rawHtml.replace(/data-pp-src=/g, 'src=') : rawHtml

    return (
        <section className="pane reader">
            {tabStrip}
            <div className="reader-header">
                <div className="reader-toolbar">
                    {onBack && <button className="btn" onClick={onBack}>&#8592; Back</button>}
                    {outbox ? (
                        <button className="btn danger-outline" onClick={() => onCancelSend(message)}>
                            Cancel send
                        </button>
                    ) : (
                    <>
                    <button className="btn" onClick={() => onReply(message)}>Reply</button>
                    {((message.to?.length || 0) + (message.cc?.length || 0)) > 0 && (
                        <button className="btn" onClick={() => onReplyAll(message)}>Reply all</button>
                    )}
                    <button className="btn" onClick={() => onForward(message)}>Forward</button>
                    <button className="btn" onClick={() => onToggleRead(message)}>
                        {message.read ? 'Mark as unread' : 'Mark as read'}
                    </button>
                    <button className="btn danger-outline" onClick={() => onDelete(message)}>Delete</button>
                    {canMoveCopy && folders.filter((f) => f.id !== message.folderId).length > 0 && (
                        <>
                            <select
                                className="move-select"
                                value=""
                                aria-label="Move to folder"
                                onChange={(e) => {
                                    if (e.target.value) {
                                        onMove(message, e.target.value)
                                    }
                                }}
                            >
                                <option value="">Move to…</option>
                                {folders.filter((f) => f.id !== message.folderId).map((f) => (
                                    <option key={f.id} value={f.id}>{f.name}</option>
                                ))}
                            </select>
                            <select
                                className="move-select"
                                value=""
                                aria-label="Copy to folder"
                                onChange={(e) => {
                                    if (e.target.value) {
                                        onCopy(message, e.target.value)
                                    }
                                }}
                            >
                                <option value="">Copy to…</option>
                                {folders.filter((f) => f.id !== message.folderId).map((f) => (
                                    <option key={f.id} value={f.id}>{f.name}</option>
                                ))}
                            </select>
                        </>
                    )}
                    <div className="tag-menu" ref={menuRef}>
                        <button
                            className="btn"
                            onClick={() => setTagMenuOpen((v) => !v)}
                            onKeyDown={(e) => {
                                // The colour dropdown drops open on Down and retracts on Up (Escape also
                                // closes it), matching the move and copy selects and the menu titles.
                                if (e.key === 'ArrowDown') {
                                    e.preventDefault()
                                    e.stopPropagation()
                                    setTagMenuOpen(true)
                                } else if (e.key === 'ArrowUp' || e.key === 'Escape') {
                                    e.preventDefault()
                                    e.stopPropagation()
                                    setTagMenuOpen(false)
                                }
                            }}
                        >
                            Colour &#9662;
                        </button>
                        {tagMenuOpen && (
                            <div className="tag-menu-dropdown" role="menu">
                                <div className="tag-colour-row" role="group" aria-label="Tag colour">
                                    {TAG_PALETTE.map((c) => {
                                        const id = colourTagId(c.colour)
                                        const isOn = assigned.has(id)
                                        return (
                                            <button
                                                key={id}
                                                className={'tag-colour' + (isOn ? ' selected' : '')}
                                                role="menuitemcheckbox"
                                                aria-checked={isOn}
                                                title={c.name}
                                                style={{backgroundColor: c.colour}}
                                                onClick={() => onToggleTag(id, !isOn)}
                                            >
                                                {isOn ? '✓' : ''}
                                            </button>
                                        )
                                    })}
                                </div>
                            </div>
                        )}
                    </div>
                    </>
                    )}
                </div>
                <h2 className="reader-subject">{message.subject || '(no subject)'}</h2>
                {messageTags.length > 0 && (
                    <div className="reader-tags">
                        {messageTags.map((tag) => (
                            <button
                                key={tag.id}
                                className="tag-dot"
                                title={`Remove ${tag.name}`}
                                aria-label={`Remove ${tag.name} colour`}
                                style={{backgroundColor: tag.colour, color: readableInk(tag.colour)}}
                                onClick={() => onToggleTag(tag.id, false)}
                            >
                                &times;
                            </button>
                        ))}
                    </div>
                )}
                <div className="reader-meta">
                    <span className="reader-label">{outbox ? 'To' : 'From'}</span>
                    <span>{outbox ? (recipients || '(no recipient)') : sender}</span>
                </div>
                {!outbox && message.to && message.to.length > 0 && (
                    <div className="reader-meta">
                        <span className="reader-label">To</span>
                        <span>{formatAddressList(message.to)}</span>
                    </div>
                )}
                {!outbox && message.cc && message.cc.length > 0 && (
                    <div className="reader-meta">
                        <span className="reader-label">Cc</span>
                        <span>{formatAddressList(message.cc)}</span>
                    </div>
                )}
                {message.date && (
                    <div className="reader-meta">
                        <span className="reader-label">Date</span>
                        <span>{new Date(message.date).toLocaleString()}</span>
                    </div>
                )}
            </div>
            <div className="reader-body">
                {!bodyLoading && body?.hasInvite && <InviteCard messageId={message.id}/>}
                {bodyLoading ? (
                    <p className="empty-body">Loading message…</p>
                ) : body && body.html.trim() !== '' ? (
                    <>
                        {hasBlockedImages && !imagesShown && (
                            <div className="images-blocked-bar">
                                <span>Remote images were not loaded to protect your privacy.</span>
                                <button className="btn" onClick={() => setImagesShown(true)}>Load images</button>
                            </div>
                        )}
                        <div
                            className="reader-html"
                            onClick={handleBodyClick}
                            dangerouslySetInnerHTML={{__html: renderedHtml}}
                        />
                    </>
                ) : body && body.plain.trim() !== '' ? (
                    <pre className="reader-text">{body.plain}</pre>
                ) : message.snippet ? (
                    <p>{message.snippet}</p>
                ) : (
                    <p className="empty-body">This message has no text content.</p>
                )}
            </div>
            {!bodyLoading && body && body.attachments && body.attachments.length > 0 && (
                <div className="reader-attachments">
                    <div className="reader-attachments-title">
                        {body.attachments.length === 1 ? '1 attachment' : `${body.attachments.length} attachments`}
                    </div>
                    {attachError && <div className="compose-error">{attachError}</div>}
                    <ul className="attachment-list">
                        {body.attachments.map((att) => (
                            <li key={att.index} className="attachment-chip">
                                <span className="attachment-name" title={att.filename}>{att.filename}</span>
                                <span className="attachment-size">{formatBytes(att.size)}</span>
                                <button
                                    type="button"
                                    className="btn"
                                    onClick={() => void saveAttachment(att.index)}
                                >
                                    Save
                                </button>
                            </li>
                        ))}
                    </ul>
                </div>
            )}
        </section>
    )
}

// formatBytes renders an attachment size in the largest unit that keeps the number readable.
function formatBytes(bytes: number): string {
    const kib = 1024
    if (bytes < kib) {
        return `${bytes} B`
    }
    const units = ['KB', 'MB', 'GB']
    let value = bytes / kib
    let unit = 0
    while (value >= kib && unit < units.length - 1) {
        value /= kib
        unit += 1
    }
    return `${value.toFixed(1)} ${units[unit]}`
}

// readableInk picks black or white text for a #rrggbb background using its perceived luminance, so a
// tag chip's label stays legible on any colour.
function readableInk(hex: string): string {
    const value = hex.replace('#', '')
    if (value.length !== 6) {
        return '#000000'
    }
    const r = parseInt(value.slice(0, 2), 16)
    const g = parseInt(value.slice(2, 4), 16)
    const b = parseInt(value.slice(4, 6), 16)
    const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
    return luminance > 0.6 ? '#000000' : '#ffffff'
}
