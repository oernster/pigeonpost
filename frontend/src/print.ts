// print holds the pure print-document builder and the hidden print-frame id. No React, no api runtime,
// so the builder is unit-tested in isolation; the actual iframe orchestration stays in the component.

// printFrameId identifies the hidden iframe used for printing, so a previous one is removed before a
// new print rather than accumulating frames.
export const printFrameId = 'pp-print-frame'

// printDocument renders a standalone HTML document for printing one message: a short header (subject,
// sender, date) followed by the message body. The body HTML is already sanitised server-side, so it is
// safe to inline here as it is in the reader.
export function printDocument(subject: string, sender: string, date: string, contentHtml: string): string {
    const head =
        '<!doctype html><html><head><meta charset="utf-8">' +
        `<title>${subject}</title>` +
        '<style>body{font-family:sans-serif;color:#000;padding:24px}' +
        '.print-head{margin-bottom:16px;border-bottom:1px solid #ccc;padding-bottom:8px}' +
        '.print-subject{font-size:20px;font-weight:600;margin-bottom:6px}' +
        '.print-meta{color:#444;font-size:13px}img{max-width:100%}</style></head><body>'
    const header =
        `<div class="print-head"><div class="print-subject">${subject}</div>` +
        `<div class="print-meta">From: ${sender}</div>` +
        (date ? `<div class="print-meta">Date: ${date}</div>` : '') +
        '</div>'
    return `${head}${header}${contentHtml}</body></html>`
}
