import {describe, expect, it} from 'vitest'
import {
    attachmentBytes,
    base64Size,
    fileToDataAttachment,
    fileToDataURI,
    inlineImageBytes,
    intakeSize,
    MAX_TOTAL_ATTACHMENT_BYTES,
    planFileIntake,
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
