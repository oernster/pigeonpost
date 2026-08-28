import {api} from './api'

// openLinkExternally opens a link from a rendered email in the OS browser rather than letting it navigate
// the app's own webview. EmailHtmlFrame has already restricted this to http, https and mailto hrefs. It
// lives here because three surfaces render email content: the reader, the attached-.eml viewer and each
// expanded message of the thread view, which must all treat a link the same way.
export function openLinkExternally(href: string) {
    void api.openExternal(href)
}
