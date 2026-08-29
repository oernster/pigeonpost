import {useCallback} from 'react'
import {Message, api} from '../api'
import {emlFilename, escapeHtml} from '../messageText'
import {printDocument, printFrameId, printFrameStyle, printReadyMarkerId} from '../print'

// useMessageExport owns the two ways a message leaves the app as a document: saved to disk as .eml and
// printed. Both are message-in, side-effect-out with no app state of their own beyond reporting a
// failure, so they belong together and away from the composition root. The pure halves live elsewhere
// already (`emlFilename` names the file, `printDocument` builds the printable HTML); what is here is the
// orchestration those two cannot do.
interface MessageExportOptions {
    setError: (message: string) => void
}

export function useMessageExport({setError}: MessageExportOptions) {
    // saveMessageAs exports the message as a .eml file via a native save dialog, named from its subject.
    const saveMessageAs = useCallback(async (message: Message) => {
        try {
            await api.saveMessageAs(message.id, emlFilename(message.subject || ''))
        } catch (e) {
            setError(String(e))
        }
    }, [setError])

    // printMessage prints one message by rendering it into a hidden, page-sized iframe parked off-screen and
    // invoking the browser's print dialog on that frame, so only the message (not the whole app window) is
    // printed. Remote images, parked in the reader for privacy, are restored for the printed copy. The frame
    // is given real off-screen dimensions (a zero-size frame prints blank) and is pinned to a light colour
    // scheme (it otherwise inherits the app's dark scheme) so the message prints as dark text on white paper.
    const printMessage = useCallback(async (message: Message) => {
        try {
            const body = await api.messageBody(message.id)
            const html = body.html?.trim() ? body.html.replace(/data-pp-src=/g, 'src=') : ''
            const content = html || `<pre>${escapeHtml(body.plain || message.snippet || '')}</pre>`
            const sender = escapeHtml(message.fromName || message.fromAddress || '(unknown sender)')
            const when = message.date ? escapeHtml(new Date(message.date).toLocaleString()) : ''
            const doc = printDocument(escapeHtml(message.subject || '(no subject)'), sender, when, content)

            document.getElementById(printFrameId)?.remove()
            const frame = document.createElement('iframe')
            frame.id = printFrameId
            frame.setAttribute('aria-hidden', 'true')
            frame.style.cssText = printFrameStyle
            frame.onload = () => {
                const win = frame.contentWindow
                // Ignore the empty about:blank document a fresh iframe momentarily holds: print only once the
                // real print document (which carries the print-ready marker) has loaded, so the dialog never
                // captures a blank page.
                if (!win || !frame.contentDocument?.getElementById(printReadyMarkerId)) {
                    return
                }
                win.onafterprint = () => frame.remove()
                win.focus()
                win.print()
            }
            // The print document is written into the frame rather than set through srcdoc: WebKit (WKWebView
            // on macOS, WebKitGTK on Linux) does not reliably fire load for a srcdoc navigation (the same
            // failure that broke the reader frame's click interception), while open()/write()/close() fires
            // load on every engine once the written document has parsed. The frame must be in the DOM before
            // writing, since a detached iframe has no document; the print-ready marker check above keeps a
            // stray about:blank load from printing a blank page.
            document.body.appendChild(frame)
            const contentDocument = frame.contentDocument
            if (contentDocument) {
                contentDocument.open()
                contentDocument.write(doc)
                contentDocument.close()
            }
        } catch (e) {
            setError(String(e))
        }
    }, [setError])

    return {saveMessageAs, printMessage}
}
