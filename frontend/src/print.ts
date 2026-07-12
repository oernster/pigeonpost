// print holds the pure print-document builder and the hidden print-frame constants. No React, no api
// runtime, so the builder and the frame style are unit-tested in isolation; the actual iframe orchestration
// stays in the component.

// printFrameId identifies the hidden iframe used for printing, so a previous one is removed before a
// new print rather than accumulating frames.
export const printFrameId = 'pp-print-frame'

// printReadyMarkerId is stamped on the print document's root header element. The print frame invokes the
// print dialog only once an element with this id is present, so the empty about:blank document a freshly
// appended iframe momentarily holds is never the document that reaches the printer.
export const printReadyMarkerId = 'pp-print-ready'

// printFrameStyle parks the print frame far off-screen but gives it a real, page-sized layout box. A
// zero-size iframe (the previous width:0;height:0) has no viewport for the engine to lay the document into,
// so some Chromium and WebView2 builds print it blank or fall back to printing the top window; a real
// off-screen box gives the document a page to render on while staying invisible. The engine re-paginates to
// the chosen paper regardless of these dimensions. The frame is pinned to a light colour scheme so it does
// not inherit the app's dark scheme (see printDocument for why that matters for printed output).
const PRINT_PAGE_WIDTH = '210mm' // A4 width
const PRINT_PAGE_HEIGHT = '297mm' // A4 height
const PRINT_FRAME_OFFSCREEN = '-10000px'
export const printFrameStyle =
    `position:fixed;left:${PRINT_FRAME_OFFSCREEN};top:0;` +
    `width:${PRINT_PAGE_WIDTH};height:${PRINT_PAGE_HEIGHT};border:0;color-scheme:light`

// printDocument renders a standalone HTML document for printing one message: a short header (subject,
// sender, date) followed by the message body. The body HTML is already sanitised server-side, so it is
// safe to inline here as it is in the reader. The document is pinned to a light colour scheme: an email
// that ships its own dark-mode CSS (a prefers-color-scheme:dark block, common with large senders) would
// otherwise render white text, which prints blank on white paper once the printer drops backgrounds.
export function printDocument(subject: string, sender: string, date: string, contentHtml: string): string {
    const head =
        '<!doctype html><html><head><meta charset="utf-8">' +
        '<meta name="color-scheme" content="light">' +
        `<title>${subject}</title>` +
        '<style>:root{color-scheme:light}body{font-family:sans-serif;color:#000;padding:24px}' +
        '.print-head{margin-bottom:16px;border-bottom:1px solid #ccc;padding-bottom:8px}' +
        '.print-subject{font-size:20px;font-weight:600;margin-bottom:6px}' +
        '.print-meta{color:#444;font-size:13px}img{max-width:100%}</style></head><body>'
    const header =
        `<div class="print-head" id="${printReadyMarkerId}"><div class="print-subject">${subject}</div>` +
        `<div class="print-meta">From: ${sender}</div>` +
        (date ? `<div class="print-meta">Date: ${date}</div>` : '') +
        '</div>'
    return `${head}${header}${contentHtml}</body></html>`
}
