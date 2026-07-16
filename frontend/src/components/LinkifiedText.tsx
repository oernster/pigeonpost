import type {ReactNode} from 'react'

// LinkifiedText renders plain message text with web addresses turned into clickable links, the way
// mainstream mail clients treat a text/plain body. It recognises http and https URLs, mailto addresses,
// www.-prefixed hosts (opened with an http scheme added) and markdown-style labelled links
// ("[label](url)"), which render as their label. A link standing alone on its own line reads as a call
// to action, so it carries the pp-solo-link class and the stylesheet presents it as a button. Trailing
// sentence punctuation stays outside a bare link. Clicks never navigate the app: the href is handed to
// onOpenLink, which opens the OS browser. This mirrors the Go-side linkify in
// internal/infrastructure/mailparse, which covers HTML bodies.

const TARGET_SOURCE = '(?:https?:\\/\\/|mailto:|www\\.)'
const LINK_TOKEN_RE = new RegExp(
    `\\[([^\\][]+)\\]\\((${TARGET_SOURCE}[^\\s()<>]+)\\)|(${TARGET_SOURCE}[^\\s<>"']+)`,
    'gi',
)
const TRAILING_PUNCTUATION_RE = /[.,;:!?)\]}]+$/

interface LinkifiedTextProps {
    text: string
    onOpenLink: (href: string) => void
}

// soloOnLine reports whether the match is the only non-whitespace content on its line of the text.
function soloOnLine(text: string, start: number, end: number): boolean {
    const lineStart = text.lastIndexOf('\n', start - 1) + 1
    const lineEndIndex = text.indexOf('\n', end)
    const lineEnd = lineEndIndex === -1 ? text.length : lineEndIndex
    return text.slice(lineStart, start).trim() === '' && text.slice(end, lineEnd).trim() === ''
}

export function LinkifiedText({text, onOpenLink}: LinkifiedTextProps) {
    const nodes: ReactNode[] = []
    const re = new RegExp(LINK_TOKEN_RE.source, 'gi')
    let cursor = 0
    for (let match = re.exec(text); match !== null; match = re.exec(text)) {
        let display: string
        let target: string
        let matchEnd: number
        if (match[1] !== undefined && match[2] !== undefined) {
            display = match[1]
            target = match[2]
            matchEnd = match.index + match[0].length
        } else {
            target = (match[3] ?? '').replace(TRAILING_PUNCTUATION_RE, '')
            if (target === '') {
                continue
            }
            display = target
            matchEnd = match.index + target.length
            re.lastIndex = matchEnd
        }
        if (match.index > cursor) {
            nodes.push(text.slice(cursor, match.index))
        }
        const href = /^www\./i.test(target) ? `http://${target}` : target
        const solo = soloOnLine(text, match.index, matchEnd)
        nodes.push(
            <a
                key={`${match.index}-${target}`}
                href={href}
                className={solo ? 'pp-solo-link' : undefined}
                onClick={(event) => {
                    event.preventDefault()
                    onOpenLink(href)
                }}
            >
                {display}
            </a>,
        )
        cursor = matchEnd
    }
    if (cursor < text.length) {
        nodes.push(text.slice(cursor))
    }
    return <>{nodes}</>
}
