import type {ReactNode} from 'react'

// LinkifiedText renders plain message text with bare web addresses turned into clickable links, the way
// mainstream mail clients treat a text/plain body. It recognises http and https URLs, mailto addresses
// and www.-prefixed hosts (opened with an http scheme added). Trailing sentence punctuation stays outside
// the link. Clicks never navigate the app: the href is handed to onOpenLink, which opens the OS browser.
// This mirrors the Go-side LinkifyHTML in internal/infrastructure/mailparse, which covers HTML bodies.

const LINK_TARGET_RE = /(?:https?:\/\/|mailto:|www\.)[^\s<>"']+/gi
const TRAILING_PUNCTUATION_RE = /[.,;:!?)\]}]+$/

interface LinkifiedTextProps {
    text: string
    onOpenLink: (href: string) => void
}

export function LinkifiedText({text, onOpenLink}: LinkifiedTextProps) {
    const nodes: ReactNode[] = []
    const re = new RegExp(LINK_TARGET_RE.source, 'gi')
    let cursor = 0
    for (let match = re.exec(text); match !== null; match = re.exec(text)) {
        const target = match[0].replace(TRAILING_PUNCTUATION_RE, '')
        if (target === '') {
            continue
        }
        if (match.index > cursor) {
            nodes.push(text.slice(cursor, match.index))
        }
        const href = /^www\./i.test(target) ? `http://${target}` : target
        nodes.push(
            <a
                key={`${match.index}-${target}`}
                href={href}
                onClick={(event) => {
                    event.preventDefault()
                    onOpenLink(href)
                }}
            >
                {target}
            </a>,
        )
        cursor = match.index + target.length
        re.lastIndex = cursor
    }
    if (cursor < text.length) {
        nodes.push(text.slice(cursor))
    }
    return <>{nodes}</>
}
