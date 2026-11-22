import {emojiName} from './emoji'

// hoverStatus decides what the bottom tray says about the thing under the pointer in a message: where a
// link really goes (so it can be judged before it is clicked) or the name of an emoji. It is pure: the
// DOM probing that finds the link or the emoji lives in hoverProbe, the drawing in BottomBar.

// LINK_SCHEMES are the URL schemes a link inside an email may open externally; any other scheme is
// ignored on click and reported as refused on hover. EmailHtmlFrame's click handler reads the same list.
const LINK_SCHEMES = ['http:', 'https:', 'mailto:']
const MAILTO = 'mailto:'

// isOpenableHref reports whether a link's href is one PigeonPost opens externally (http, https or
// mailto). Other schemes (javascript:, data:, cid:, tel: and the rest) are never opened.
export function isOpenableHref(href: string): boolean {
    const scheme = href.trim().toLowerCase()
    return LINK_SCHEMES.some((s) => scheme.startsWith(s))
}

// LinkStatus describes a hovered link. The destination is split around its host (before, host, after)
// so the tray can make the host, the part that says whose site it is, stand out. shown is the site or
// address the link's visible text names when that differs from where it goes: the classic phishing
// shape of "www.yourbank.com" written over a link to somewhere else. openable is false for a scheme
// PigeonPost will not open.
export interface LinkStatus {
    kind: 'link'
    before: string
    host: string
    after: string
    openable: boolean
    shown: string | null
}

export interface EmojiStatus {
    kind: 'emoji'
    glyph: string
    name: string
}

export type HoverStatus = LinkStatus | EmojiStatus

// emojiStatus answers the tray's reading of a glyph; null when it is not an emoji.
export function emojiStatus(glyph: string): EmojiStatus | null {
    const name = emojiName(glyph)
    return name === null ? null : {kind: 'emoji', glyph, name}
}

// A visible text names a site when it is one word holding a dot and ending in a letter-only top-level
// label: "example.com", "www.example.co.uk", "https://example.com/path". "v1.2" and "Click here" do not.
const SITE_TEXT_RE = /^(?:https?:\/\/)?[^\s/@]+\.[a-z]{2,}(?:[/:?#]\S*)?$/i
const EMAIL_TEXT_RE = /^[^\s@]+@[^\s@]+\.[a-z]{2,}$/i
const SCHEME_RE = /^[a-z][a-z0-9+.-]*:\/\//i
const WWW = 'www.'

function parseUrl(text: string): URL | null {
    try {
        return new URL(text)
    } catch {
        return null
    }
}

function bareHost(host: string): string {
    const lower = host.toLowerCase()
    return lower.startsWith(WWW) ? lower.slice(WWW.length) : lower
}

// sameSite reports whether two hosts belong together: equal once a leading www. is set aside; else one
// a subdomain of the other ("example.com" over a link to "mail.example.com" is ordinary).
function sameSite(a: string, b: string): boolean {
    const x = bareHost(a)
    const y = bareHost(b)
    return x === y || x.endsWith(`.${y}`) || y.endsWith(`.${x}`)
}

// shownHost answers the host a link's visible text names, normalised the way the URL parser normalises
// the real one (lower case, an international name in its xn-- form) so the two compare like for like.
function shownHost(text: string): string | null {
    if (!SITE_TEXT_RE.test(text)) {
        return null
    }
    return parseUrl(SCHEME_RE.test(text) ? text : `http://${text}`)?.hostname ?? null
}

function mailtoStatus(href: string, text: string): LinkStatus {
    const rest = href.slice(MAILTO.length)
    const queryAt = rest.indexOf('?')
    const rawAddress = queryAt === -1 ? rest : rest.slice(0, queryAt)
    const address = safeDecode(rawAddress)
    const shown = EMAIL_TEXT_RE.test(text) && text.toLowerCase() !== address.toLowerCase() ? text : null
    return {
        kind: 'link', before: MAILTO, host: address, after: queryAt === -1 ? '' : rest.slice(queryAt),
        openable: true, shown,
    }
}

function safeDecode(text: string): string {
    try {
        return decodeURIComponent(text)
    } catch {
        return text
    }
}

// hostOffset answers where the host starts in url.href. It is worked out from the parts rather than
// searched for, because "https://bank.com@elsewhere.com" carries a decoy ahead of its real host: the
// user-info before the @, which is exactly the text a search would find first.
const AUTHORITY_MARK = '//'
function hostOffset(url: URL): number {
    const password = url.password === '' ? '' : `:${url.password}`
    const userInfo = url.username === '' && password === '' ? '' : `${url.username}${password}@`
    return url.protocol.length + AUTHORITY_MARK.length + userInfo.length
}

function refused(href: string): LinkStatus {
    return {kind: 'link', before: href, host: '', after: '', openable: false, shown: null}
}

// HoverChannel carries the current reading from wherever the pointer is (a message frame, a plain-text
// body, the compose editor) to the tray. Reports arrive on every pointer move, so report keeps quiet
// unless the reading actually changed; only the tray subscribes, so nothing else re-renders on a hover.
export interface HoverChannel {
    report: (status: HoverStatus | null) => void
    current: () => HoverStatus | null
    subscribe: (listener: () => void) => () => void
}

function statusKey(status: HoverStatus | null): string {
    return status === null ? '' : JSON.stringify(status)
}

export function createHoverChannel(): HoverChannel {
    let status: HoverStatus | null = null
    const listeners = new Set<() => void>()
    return {
        report: (next) => {
            if (statusKey(next) === statusKey(status)) {
                return
            }
            status = next
            listeners.forEach((listener) => listener())
        },
        current: () => status,
        subscribe: (listener) => {
            listeners.add(listener)
            return () => {
                listeners.delete(listener)
            }
        },
    }
}

// linkStatus answers the tray's reading of a link from its href and the text it is drawn with.
export function linkStatus(href: string, text: string): LinkStatus {
    const trimmed = href.trim()
    const visible = text.trim()
    if (!isOpenableHref(trimmed)) {
        return refused(trimmed)
    }
    if (trimmed.toLowerCase().startsWith(MAILTO)) {
        return mailtoStatus(trimmed, visible)
    }
    const url = parseUrl(trimmed)
    if (url === null || url.host === '') {
        return refused(trimmed)
    }
    const full = url.href
    const hostAt = hostOffset(url)
    const named = shownHost(visible)
    return {
        kind: 'link',
        before: full.slice(0, hostAt),
        host: url.host,
        after: full.slice(hostAt + url.host.length),
        openable: true,
        shown: named !== null && !sameSite(named, url.hostname) ? named : null,
    }
}
