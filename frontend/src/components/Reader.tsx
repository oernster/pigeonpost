import {useEffect, useRef, useState} from 'react'
import type {MouseEvent as ReactMouseEvent, RefObject} from 'react'
import {api, EmailView, Folder, Message, MessageBody, Tag} from '../api'
import {EmailViewerModal} from './EmailViewerModal'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {isOutboxMessage} from '../outbox'
import {ReaderTabs} from './ReaderTabs'
import {InviteCard} from './InviteCard'
import {formatAddress, formatAddressList, formatBytes, readableInk} from '../readerFormat'

// The reader body is a scrollable focus stop: the arrow keys scroll it so a long email can be read from
// the keyboard. READER_SCROLL_STEP_PX is one arrow press; PageUp/PageDown move by READER_PAGE_FRACTION of
// the visible height.
const READER_SCROLL_STEP_PX = 40
const READER_PAGE_FRACTION = 0.9

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
    // bodyRef is attached to the scrollable email body so the parent can move focus onto it when a message
    // is opened, so the keyboard lands on the email rather than jumping back to the start of the ring.
    bodyRef?: RefObject<HTMLDivElement>
    // sinkRef is a neutral anchor at the top of the full-width reader; the parent focuses it on a mouse
    // open so the first Tab lands on the Back button.
    sinkRef?: RefObject<HTMLSpanElement>
}

export function Reader({message, onToggleRead, onReply, onReplyAll, onForward, onDelete, onCancelSend, folders, onMove, onCopy, canMoveCopy, messageTags, onToggleTag, body, bodyLoading, tabs, onSelectTab, onCloseTab, onBack, bodyRef, sinkRef}: ReaderProps) {
    const [tagMenuOpen, setTagMenuOpen] = useState(false)
    const [imagesShown, setImagesShown] = useState(false)
    const [attachError, setAttachError] = useState('')
    // viewedEmail holds a parsed .eml attachment while the in-app viewer shows it.
    const [viewedEmail, setViewedEmail] = useState<EmailView | null>(null)
    const menuRef = useRef<HTMLDivElement>(null)
    // tagButtonRef is the Colour trigger and tagRowRef the swatch row; openedByKeyRef records a keyboard
    // open so focus lands on the first swatch (a mouse open leaves focus on the trigger).
    const tagButtonRef = useRef<HTMLButtonElement>(null)
    const tagRowRef = useRef<HTMLDivElement>(null)
    const openedByKeyRef = useRef(false)
    // backButtonRef is the Back button; the reader's neutral sink hands the first Tab to it on a mouse open.
    const backButtonRef = useRef<HTMLButtonElement>(null)

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

    // openAttachment shows an attached .eml in the in-app viewer (so it never hands off to an external mail
    // client). Any other file opens with the OS default app after writing it to a temporary file.
    const openAttachment = async (index: number, filename: string) => {
        if (!message) return
        setAttachError('')
        try {
            if (filename.toLowerCase().endsWith('.eml')) {
                setViewedEmail(await api.openEmailAttachment(message.id, index))
            } else {
                await api.openAttachment(message.id, index)
            }
        } catch (e) {
            setAttachError(String(e))
        }
    }

    // saveAllAttachments writes every attachment into a folder chosen through a native dialog, in one step.
    const saveAllAttachments = async () => {
        if (!message) return
        setAttachError('')
        try {
            await api.saveAllAttachments(message.id)
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

    // When the colour menu opens by keyboard, move focus to the first swatch so Left/Right walk them.
    useEffect(() => {
        if (tagMenuOpen && openedByKeyRef.current) {
            tagRowRef.current?.querySelector<HTMLButtonElement>('.tag-colour')?.focus()
        }
        if (!tagMenuOpen) {
            openedByKeyRef.current = false
        }
    }, [tagMenuOpen])

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
        <section className={'pane reader' + (onBack ? ' reader-scoped' : '')}>
            {onBack && (
                <span
                    ref={sinkRef}
                    tabIndex={-1}
                    aria-hidden="true"
                    style={{position: 'absolute', width: 0, height: 0, overflow: 'hidden', outline: 'none'}}
                    onKeyDown={(e) => {
                        // A mouse open lands focus here; the first Tab goes to Back so the reader starts there.
                        if (e.key === 'Tab' && !e.shiftKey) {
                            e.preventDefault()
                            e.stopPropagation()
                            backButtonRef.current?.focus()
                        }
                    }}
                />
            )}
            {tabStrip}
            <div className="reader-header">
                <div className="reader-toolbar">
                    {onBack && <button ref={backButtonRef} className="btn" onClick={onBack}>&#8592; Back</button>}
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
                            ref={tagButtonRef}
                            className="btn"
                            onClick={() => setTagMenuOpen((v) => !v)}
                            onKeyDown={(e) => {
                                // Down, Enter or Space drops the swatch menu open and lands on the first
                                // swatch, from where Left/Right walk them. Up or Escape retracts it.
                                if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
                                    e.preventDefault()
                                    e.stopPropagation()
                                    openedByKeyRef.current = true
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
                                <div
                                    className="tag-colour-row"
                                    role="group"
                                    aria-label="Tag colour"
                                    ref={tagRowRef}
                                    onKeyDown={(e) => {
                                        // Left/Right walk the swatches, wrapping. Escape or Up closes back to
                                        // the Colour button. Tab and Shift+Tab are left to bubble so the window
                                        // ring steps out of the menu (they exit it). Enter/Space toggle a
                                        // swatch via its own click. Up/Down are swallowed so they never leak to
                                        // the message list.
                                        const swatches = Array.from(
                                            tagRowRef.current?.querySelectorAll<HTMLButtonElement>('.tag-colour') ?? [],
                                        )
                                        if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                                            e.preventDefault()
                                            e.stopPropagation()
                                            if (swatches.length === 0) {
                                                return
                                            }
                                            const at = swatches.indexOf(document.activeElement as HTMLButtonElement)
                                            const next = e.key === 'ArrowRight'
                                                ? (at + 1) % swatches.length
                                                : (at - 1 + swatches.length) % swatches.length
                                            swatches[next].focus()
                                        } else if (e.key === 'Escape' || e.key === 'ArrowUp') {
                                            e.preventDefault()
                                            e.stopPropagation()
                                            setTagMenuOpen(false)
                                            tagButtonRef.current?.focus()
                                        } else if (e.key === 'ArrowDown') {
                                            e.preventDefault()
                                            e.stopPropagation()
                                        } else if (e.key === 'Tab') {
                                            // Exit: close the menu and hand focus back to the Colour button,
                                            // then let the window handler step the ring on from there.
                                            setTagMenuOpen(false)
                                            tagButtonRef.current?.focus()
                                        }
                                    }}
                                >
                                    {TAG_PALETTE.map((c) => {
                                        const id = colourTagId(c.colour)
                                        const isOn = assigned.has(id)
                                        return (
                                            <button
                                                key={id}
                                                className={'tag-colour' + (isOn ? ' selected' : '')}
                                                role="menuitemcheckbox"
                                                aria-checked={isOn}
                                                tabIndex={-1}
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
            <div
                ref={bodyRef}
                className="reader-body"
                tabIndex={0}
                onKeyDown={(e) => {
                    // The reader body is a scrollable stop: the arrow keys plus Page/Home/End scroll the
                    // reader pane so a long email is read from the keyboard, stopped from reaching the window
                    // ring handler. Tab and Shift+Tab are left alone, so they step the ring out of the body.
                    const scroller = e.currentTarget.closest<HTMLElement>('.pane')
                    if (!scroller) {
                        return
                    }
                    const page = scroller.clientHeight * READER_PAGE_FRACTION
                    let dx = 0
                    let dy = 0
                    if (e.key === 'ArrowDown') {
                        dy = READER_SCROLL_STEP_PX
                    } else if (e.key === 'ArrowUp') {
                        dy = -READER_SCROLL_STEP_PX
                    } else if (e.key === 'ArrowRight') {
                        dx = READER_SCROLL_STEP_PX
                    } else if (e.key === 'ArrowLeft') {
                        dx = -READER_SCROLL_STEP_PX
                    } else if (e.key === 'PageDown') {
                        dy = page
                    } else if (e.key === 'PageUp') {
                        dy = -page
                    } else if (e.key === 'Home') {
                        e.preventDefault()
                        e.stopPropagation()
                        scroller.scrollTo({top: 0})
                        return
                    } else if (e.key === 'End') {
                        e.preventDefault()
                        e.stopPropagation()
                        scroller.scrollTo({top: scroller.scrollHeight})
                        return
                    } else {
                        return
                    }
                    e.preventDefault()
                    e.stopPropagation()
                    scroller.scrollBy({left: dx, top: dy})
                }}
            >
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
                        <span>{body.attachments.length === 1 ? '1 attachment' : `${body.attachments.length} attachments`}</span>
                        {body.attachments.length > 1 && (
                            <button type="button" className="btn" onClick={() => void saveAllAttachments()}>
                                Save all
                            </button>
                        )}
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
                                    onClick={() => void openAttachment(att.index, att.filename)}
                                >
                                    Open
                                </button>
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
            {viewedEmail && (
                <EmailViewerModal email={viewedEmail} onClose={() => setViewedEmail(null)}/>
            )}
        </section>
    )
}

