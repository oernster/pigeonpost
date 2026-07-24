import {afterEach, describe, expect, it, vi} from 'vitest'
import {
    attachmentBytes,
    base64Size,
    fileToDataAttachment,
    fileToDataURI,
    inlineImageBytes,
    intakeSize,
    MAX_TOTAL_ATTACHMENT_BYTES,
    planFileIntake,
    transferFilePaths,
    transferFiles,
} from './composeIntake'

const png = new File([new Uint8Array([137, 80, 78, 71, 1, 2, 3])], 'shot.png', {type: 'image/png'})
const jpeg = new File([new Uint8Array([255, 216, 255])], 'photo.jpg', {type: 'image/jpeg'})
const pdf = new File([new Uint8Array([37, 80, 68, 70])], 'report.pdf', {type: 'application/pdf'})
const unknown = new File([new Uint8Array([1])], 'blob.bin', {type: ''})

describe('planFileIntake', () => {
    it('embeds image files and attaches everything else', () => {
        const plan = planFileIntake([png, pdf, jpeg, unknown])
        expect(plan.embed.map((f) => f.name)).toEqual(['shot.png', 'photo.jpg'])
        expect(plan.attach.map((f) => f.name)).toEqual(['report.pdf', 'blob.bin'])
    })

    it('handles an empty list', () => {
        expect(planFileIntake([])).toEqual({embed: [], attach: []})
    })
})

describe('transferFiles', () => {
    it('prefers the files list when the engine fills it', () => {
        expect(transferFiles({files: [png], items: []})).toEqual([png])
    })

    it('falls back to file items when files is empty (WebKit pasted images)', () => {
        const items = [
            {kind: 'string', getAsFile: () => null},
            {kind: 'file', getAsFile: () => png},
            {kind: 'file', getAsFile: () => null},
        ]
        expect(transferFiles({files: [], items})).toEqual([png])
    })

    it('is empty for a missing transfer or surfaces', () => {
        expect(transferFiles(null)).toEqual([])
        expect(transferFiles({})).toEqual([])
    })
})

describe('transferFilePaths', () => {
    it('converts file URIs to local paths, decoding and handling drive letters', () => {
        const dt = {getData: (format: string) => format === 'text/uri-list'
            ? 'file:///Users/oliver/My%20Report.pdf\r\nfile:///C:/docs/notes.txt\nfile://localhost/tmp/x.bin'
            : ''}
        expect(transferFilePaths(dt)).toEqual([
            '/Users/oliver/My Report.pdf',
            'C:/docs/notes.txt',
            '/tmp/x.bin',
        ])
    })

    it('ignores comments, remote URLs and blank lines', () => {
        const dt = {getData: () => '# comment\r\nhttps://example.org/a.pdf\r\n\r\nfile:///ok.txt'}
        expect(transferFilePaths(dt)).toEqual(['/ok.txt'])
    })

    it('is empty without a transfer or a getData surface', () => {
        expect(transferFilePaths(null)).toEqual([])
        expect(transferFilePaths({})).toEqual([])
    })
})

describe('sizes and budget', () => {
    it('sums incoming file sizes', () => {
        expect(intakeSize([png, pdf])).toBe(png.size + pdf.size)
    })

    it('computes decoded base64 sizes including padding', () => {
        expect(base64Size(btoa('a'))).toBe(1)
        expect(base64Size(btoa('ab'))).toBe(2)
        expect(base64Size(btoa('abc'))).toBe(3)
        expect(base64Size('')).toBe(0)
    })

    it('sums held attachment bytes', () => {
        const attachments = [
            {name: 'a', contentType: 'application/pdf', content: btoa('abcd')},
            {name: 'b', contentType: '', content: btoa('xy')},
        ]
        expect(attachmentBytes(attachments)).toBe(6)
    })

    it('estimates embedded image bytes from editor HTML', () => {
        const html = `<p><img src="data:image/png;base64,${btoa('12345')}"> and ` +
            `<img src="data:image/jpeg;base64,${btoa('678')}"></p>`
        expect(inlineImageBytes(html)).toBe(8)
    })

    it('ignores non-image data URIs and remote sources', () => {
        const html = `<img src="https://example.org/x.png"><img src="data:text/plain;base64,${btoa('hi')}">`
        expect(inlineImageBytes(html)).toBe(0)
    })

    it('exposes the backend cap', () => {
        expect(MAX_TOTAL_ATTACHMENT_BYTES).toBe(25 * 1024 * 1024)
    })
})

describe('file reading', () => {
    it('reads an image file into a data URI with its own bytes', async () => {
        const uri = await fileToDataURI(png)
        expect(uri.startsWith('data:image/png;base64,')).toBe(true)
        const decoded = atob(uri.slice(uri.indexOf(',') + 1))
        expect(Array.from(decoded, (c) => c.charCodeAt(0))).toEqual([137, 80, 78, 71, 1, 2, 3])
    })

    it('reads a file into an in-memory attachment', async () => {
        const attachment = await fileToDataAttachment(pdf)
        expect(attachment.name).toBe('report.pdf')
        expect(attachment.contentType).toBe('application/pdf')
        expect(atob(attachment.content)).toBe('%PDF')
    })

    it('names a nameless pasted file', async () => {
        const nameless = new File([new Uint8Array([1, 2])], '', {type: 'application/octet-stream'})
        const attachment = await fileToDataAttachment(nameless)
        expect(attachment.name).toBe('pasted-file')
    })
})

// stubFailingReader replaces FileReader with one that always reports failure, carrying the given
// error (or none, exercising the fallback), so the rejection paths are reachable in jsdom.
function stubFailingReader(failure: Error | null) {
    vi.stubGlobal('FileReader', class {
        onload: null | (() => void) = null
        onerror: null | (() => void) = null
        result: string | ArrayBuffer | null = null
        error = failure
        readAsDataURL() {
            queueMicrotask(() => this.onerror?.())
        }
    })
}

describe('read failures', () => {
    afterEach(() => vi.unstubAllGlobals())

    it('rejects with the reader error when one is reported', async () => {
        stubFailingReader(new Error('disk detached'))
        await expect(fileToDataAttachment(pdf)).rejects.toThrow('disk detached')
    })

    it('rejects with a fallback naming the file when the reader gives no error', async () => {
        stubFailingReader(null)
        const nameless = new File([new Uint8Array([1])], '', {type: 'application/octet-stream'})
        await expect(fileToDataURI(nameless)).rejects.toThrow('read pasted-file')
    })
})
