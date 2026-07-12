import {useEffect, useRef, useState} from 'react'
import type {RefObject} from 'react'
import {api, EmailView, Folder, Message, MessageBody, Tag} from '../api'
import {EmailHtmlFrame} from './EmailHtmlFrame'
import {EmailViewerModal} from './EmailViewerModal'
import {ReaderToolbar} from './ReaderToolbar'
import {ReaderAttachments} from './ReaderAttachments'
import {isOutboxMessage} from '../outbox'
import {ReaderTabs} from './ReaderTabs'
import {InviteCard} from './InviteCard'
import {formatAddressList, readableInk} from '../readerFormat'

// The reader body is a scrollable focus stop: the arrow keys scroll it so a long email can be read from
// the keyboard. READER_SCROLL_STEP_PX is one arrow press; PageUp/PageDown move by READER_PAGE_FRACTION of
// the visible height.
const READER_SCROLL_STEP_PX = 40
const READER_PAGE_FRACTION = 0.9

// openLinkExternally opens a link from a rendered email in the OS browser rather than letting it navigate
// the app's own webview. EmailHtmlFrame has already restricted this to http, https and mailto hrefs.
function openLinkExternally(href: string) {
    void api.openExternal(href)
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
    const [imagesShown, setImagesShown] = useState(false)
    // viewedEmail holds a parsed .eml attachment while the in-app viewer shows it.
    const [viewedEmail, setViewedEmail] = useState<EmailView | null>(null)
    // backButtonRef is the Back button; the reader's neutral sink hands the first Tab to it on a mouse open.
    const backButtonRef = useRef<HTMLButtonElement>(null)

    const tabStrip = tabs.length > 0
        ? <ReaderTabs tabs={tabs} activeMessageId={message?.id ?? ''} onSelectTab={onSelectTab} onCloseTab={onCloseTab}/>
        : null

    // Re-block images whenever the selected message changes. The colour menu and the attachments block own
    // their own per-message resets.
    useEffect(() => {
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

    const outbox = isOutboxMessage(message)
    const sender = message.fromName
        ? `${message.fromName} <${message.fromAddress}>`
        : message.fromAddress || '(unknown sender)'
    const recipients = message.to.map((a) => a.address).filter(Boolean).join(', ')

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
                <ReaderToolbar
                    message={message}
                    outbox={outbox}
                    onBack={onBack}
                    backButtonRef={backButtonRef}
                    onReply={onReply}
                    onReplyAll={onReplyAll}
                    onForward={onForward}
                    onToggleRead={onToggleRead}
                    onDelete={onDelete}
                    onCancelSend={onCancelSend}
                    folders={folders}
                    canMoveCopy={canMoveCopy}
                    onMove={onMove}
                    onCopy={onCopy}
                    messageTags={messageTags}
                    onToggleTag={onToggleTag}
                />
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
                        <EmailHtmlFrame
                            html={renderedHtml}
                            imagesShown={imagesShown}
                            onOpenLink={openLinkExternally}
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
                <ReaderAttachments
                    message={message}
                    attachments={body.attachments}
                    onViewEmail={setViewedEmail}
                />
            )}
            {viewedEmail && (
                <EmailViewerModal email={viewedEmail} onClose={() => setViewedEmail(null)}/>
            )}
        </section>
    )
}

