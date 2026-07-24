// composeIntake classifies what the user pastes or drops into the compose window and reads it into
// the forms the composer needs. The rule is one sentence: images embed in the body, every other file
// becomes an attachment. An embedded image keeps its original bytes (the File object carries the real
// file content, never a re-encoded bitmap), so a pasted photo.jpg arrives as the same JPEG.

// MAX_ATTACHMENT_MEBIBYTES mirrors maxAttachmentMebibytes in send.go: the backend enforces the cap at
// send time; this front-end copy lets an oversized paste be refused at once instead of at send.
export const MAX_ATTACHMENT_MEBIBYTES = 25
const BYTES_PER_MEBIBYTE = 1 << 20
export const MAX_TOTAL_ATTACHMENT_BYTES = MAX_ATTACHMENT_MEBIBYTES * BYTES_PER_MEBIBYTE

// FALLBACK_FILE_NAME names a pasted file whose File object arrived nameless, so the attachment chip
// and the sent part always have something to show.
const FALLBACK_FILE_NAME = 'pasted-file'

// DataAttachment is a file carried in memory rather than by path: pasted or dropped into the compose
// window, where the webview has its name and bytes but no filesystem path. content is base64, matching
// AttachmentDataEntry in send.go.
export interface DataAttachment {
    name: string
    contentType: string
    content: string
}

// FileIntakePlan is the split of an incoming file list: images to embed at the cursor and everything
// else to attach.
export interface FileIntakePlan {
    embed: File[]
    attach: File[]
}

// FileTransfer is the slice of DataTransfer the intake reads, named so tests can hand in plain
// objects.
export interface FileTransfer {
    files?: ArrayLike<File>
    items?: ArrayLike<{kind: string; getAsFile(): File | null}>
    getData?(format: string): string
}

// transferFiles collects the File objects a paste or drop carries, reading both surfaces the
// engines use: Chromium fills files; WebKit (Safari and the macOS Wails webview) has historically
// exposed pasted image data only through items, leaving files empty. items is the fallback, read
// only when files is empty so no engine double-counts.
export function transferFiles(dt: FileTransfer | null): File[] {
    if (!dt) {
        return []
    }
    const files = Array.from(dt.files ?? [])
    if (files.length > 0) {
        return files
    }
    const out: File[] = []
    for (const item of Array.from(dt.items ?? [])) {
        if (item.kind === 'file') {
            const file = item.getAsFile()
            if (file) {
                out.push(file)
            }
        }
    }
    return out
}

// transferFilePaths reads the file paths a paste carries as file:// URIs (text/uri-list). WebKit
// never exposes a Finder-copied file as a File object in a paste event; where it exposes anything,
// it is the file's URI. Those paths ride the existing path-based attachment flow (the backend reads
// them from disk at send time), so a Finder copy-paste attaches like a picked file.
export function transferFilePaths(dt: FileTransfer | null): string[] {
    const raw = dt?.getData?.('text/uri-list') ?? ''
    const out: string[] = []
    for (const line of raw.split(/\r?\n/)) {
        const path = fileURIToPath(line.trim())
        if (path !== '') {
            out.push(path)
        }
    }
    return out
}

// fileURIToPath converts one file:// URI into a local filesystem path, or '' for anything else
// (a comment line, a remote URL). The host part is empty or localhost by RFC 8089; a Windows path
// arrives as file:///C:/... whose leading slash is dropped.
function fileURIToPath(uri: string): string {
    const lower = uri.toLowerCase()
    let rest: string
    if (lower.startsWith('file://localhost/')) {
        rest = uri.slice('file://localhost'.length)
    } else if (lower.startsWith('file:///')) {
        rest = uri.slice('file://'.length)
    } else {
        return ''
    }
    const decoded = decodeURIComponent(rest)
    // file:///C:/dir/x.pdf denotes the Windows path C:/dir/x.pdf; the leading slash is the URI's.
    if (/^\/[A-Za-z]:/.test(decoded)) {
        return decoded.slice(1)
    }
    return decoded
}

// planFileIntake applies the intake rule to a pasted or dropped file list: an image file embeds, any
// other file attaches.
export function planFileIntake(files: readonly File[]): FileIntakePlan {
    const plan: FileIntakePlan = {embed: [], attach: []}
    for (const file of files) {
        if (file.type.startsWith('image/')) {
            plan.embed.push(file)
        } else {
            plan.attach.push(file)
        }
    }
    return plan
}

// intakeSize is the total byte size of an incoming file list, for the budget check before reading.
export function intakeSize(files: readonly File[]): number {
    return files.reduce((total, file) => total + file.size, 0)
}

// base64Size returns the decoded byte size of a base64 string: three bytes per four characters, less
// the padding.
export function base64Size(encoded: string): number {
    const padding = encoded.endsWith('==') ? 2 : encoded.endsWith('=') ? 1 : 0
    return (encoded.length * 3) / 4 - padding
}

// attachmentBytes is the decoded total of the in-memory attachments already held.
export function attachmentBytes(attachments: readonly DataAttachment[]): number {
    return attachments.reduce((total, a) => total + base64Size(a.content), 0)
}

// inlineDataImageRe matches an embedded image's base64 payload inside editor HTML. The editor emits
// double-quoted attributes and base64 holds no quotes, so the payload runs to the closing quote.
const inlineDataImageRe = /src="data:image\/[^"]*?;base64,([^"]*)"/g

// inlineImageBytes estimates the decoded size of the images embedded in the editor HTML, so the budget
// check covers what is already pasted into the body. The backend recomputes this authoritatively at
// send time (see composeAttachments in send.go).
export function inlineImageBytes(html: string): number {
    let total = 0
    for (const match of html.matchAll(inlineDataImageRe)) {
        total += base64Size(match[1])
    }
    return total
}

// readAsDataURL wraps FileReader for one file, resolving to its data: URI.
function readAsDataURL(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result))
        reader.onerror = () => reject(reader.error ?? new Error(`read ${file.name || FALLBACK_FILE_NAME}`))
        reader.readAsDataURL(file)
    })
}

// fileToDataURI reads an image file into the data: URI the editor embeds. The bytes are the file's
// own, unrecoded.
export function fileToDataURI(file: File): Promise<string> {
    return readAsDataURL(file)
}

// fileToDataAttachment reads a file into the in-memory attachment form the send request carries.
export async function fileToDataAttachment(file: File): Promise<DataAttachment> {
    const dataURL = await readAsDataURL(file)
    // A data: URL is "data:<type>;base64,<payload>"; the payload starts after the first comma.
    const content = dataURL.slice(dataURL.indexOf(',') + 1)
    return {name: file.name || FALLBACK_FILE_NAME, contentType: file.type, content}
}
