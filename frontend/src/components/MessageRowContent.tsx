import {type ReactNode} from 'react'
import {Message, SEARCH_MATCH_END, SEARCH_MATCH_START} from '../api'
import type {AccountChip} from '../unified'
import {snoozedUntilLabel} from '../snooze'

// REPLIED_GLYPH and FORWARDED_GLYPH are the small arrows shown at the top-left of a row once the message
// has been answered or passed on. The variation selector forces the text form, so they take the row's own
// colour rather than arriving as a coloured emoji.
const REPLIED_GLYPH = '\u{21A9}\u{FE0E}'
const FORWARDED_GLYPH = '\u{21AA}\u{FE0E}'
const ATTACHMENT_GLYPH = '\u{1F4CE}'

// formatDate renders a row's date: the day with the time, spelled out enough that a message from another
// year is never mistaken for a recent one.
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

interface MessageRowContentProps {
    message: Message
    // matchSnippet is the search-matched preview (terms wrapped in the match markers) when the row came
    // from a search; without one the row shows the message's stored snippet.
    matchSnippet: string | undefined
    // accountChip labels a unified-mailbox row with its account's colour and address. Absent on every
    // per-folder listing, where every row belongs to the same account.
    accountChip: AccountChip | undefined
    onToggleFlag: (message: Message) => void
}

// MessageRowContent is what a message row shows: its status glyphs and star, who it is from, its tag
// dots, its date, its subject and its preview. The row element around it owns selection, dragging and the
// context menu; this owns only what is painted inside, so the list stays readable at its own level.
export function MessageRowContent({message, matchSnippet, accountChip, onToggleFlag}: MessageRowContentProps) {
    return (
        <>
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
                        onToggleFlag(message)
                    }}
                >
                    {message.flagged ? '★' : '☆'}
                </button>
                {message.hasAttachments && (
                    <span className="attach" title="Has attachments" aria-label="Has attachments">
                        {ATTACHMENT_GLYPH}
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
                {message.snoozedUntilMs > 0 && (
                    <span className="snoozed-until" title="Snoozed until">
                        {snoozedUntilLabel(message.snoozedUntilMs)}
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
        </>
    )
}
