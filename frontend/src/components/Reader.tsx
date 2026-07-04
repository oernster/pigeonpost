import {useEffect, useRef, useState} from 'react'
import type {MouseEvent as ReactMouseEvent} from 'react'
import {api, Folder, Message, MessageBody, Tag} from '../api'
import {ReaderTabs} from './ReaderTabs'

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

interface ReaderProps {
    message: Message | null
    onToggleRead: (message: Message) => void
    onReply: (message: Message) => void
    onReplyAll: (message: Message) => void
    onForward: (message: Message) => void
    onDelete: (message: Message) => void
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
}

export function Reader({message, onToggleRead, onReply, onReplyAll, onForward, onDelete, folders, onMove, onCopy, canMoveCopy, tags, messageTags, onToggleTag, body, bodyLoading, tabs, onSelectTab, onCloseTab}: ReaderProps) {
    const [tagMenuOpen, setTagMenuOpen] = useState(false)
    const [imagesShown, setImagesShown] = useState(false)
    const menuRef = useRef<HTMLDivElement>(null)

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
    }, [message?.id])

    if (!message) {
        return (
            <section className="pane reader">
                {tabStrip}
                <div className="empty-state"><p className="empty-body">Select a message to read.</p></div>
            </section>
        )
    }

    const sender = message.fromName
        ? `${message.fromName} <${message.fromAddress}>`
        : message.fromAddress || '(unknown sender)'

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
                        <button className="btn" onClick={() => setTagMenuOpen((v) => !v)}>Tags &#9662;</button>
                        {tagMenuOpen && (
                            <div className="tag-menu-dropdown" role="menu">
                                {tags.length === 0 ? (
                                    <div className="tag-menu-empty">No tags defined yet.</div>
                                ) : (
                                    tags.map((tag) => (
                                        <button
                                            key={tag.id}
                                            className="tag-menu-item"
                                            role="menuitemcheckbox"
                                            aria-checked={assigned.has(tag.id)}
                                            onClick={() => onToggleTag(tag.id, !assigned.has(tag.id))}
                                        >
                                            <span className="tag-check">{assigned.has(tag.id) ? '✓' : ''}</span>
                                            <span className="tag-swatch" style={{backgroundColor: tag.colour}}/>
                                            <span className="tag-menu-name">{tag.name}</span>
                                        </button>
                                    ))
                                )}
                            </div>
                        )}
                    </div>
                </div>
                <h2 className="reader-subject">{message.subject || '(no subject)'}</h2>
                {messageTags.length > 0 && (
                    <div className="reader-tags">
                        {messageTags.map((tag) => (
                            <span
                                key={tag.id}
                                className="tag-chip"
                                style={{backgroundColor: tag.colour, color: readableInk(tag.colour)}}
                            >
                                {tag.name}
                                <button
                                    className="tag-chip-remove"
                                    aria-label={`Remove tag ${tag.name}`}
                                    onClick={() => onToggleTag(tag.id, false)}
                                >
                                    &times;
                                </button>
                            </span>
                        ))}
                    </div>
                )}
                <div className="reader-meta">
                    <span className="reader-label">From</span>
                    <span>{sender}</span>
                </div>
                {message.date && (
                    <div className="reader-meta">
                        <span className="reader-label">Date</span>
                        <span>{new Date(message.date).toLocaleString()}</span>
                    </div>
                )}
            </div>
            <div className="reader-body">
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
        </section>
    )
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
