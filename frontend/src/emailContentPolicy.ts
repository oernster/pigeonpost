// emailContentPolicy is the one home for the Content-Security-Policy every document that renders a message
// body carries: the reader's frame (EmailHtmlFrame) and the print frame (print.ts). Keeping it in one place
// means printing a message can never be looser than reading it.
//
// The policy grants no script-src (so no JavaScript runs even if some slipped past the sanitiser), blocks
// every default source and permits only inline styles, data: fonts plus data: images. It never allows a
// remote http/https image: a message's remote images are fetched server-side and inlined as data: URIs before
// they reach either frame (see the LoadRemoteImages proxy), so neither frame makes a remote request and
// neither can leak that a message was opened, even for an image whose server-side fetch failed and stayed
// parked.
export const EMAIL_CONTENT_SECURITY_POLICY =
    "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:;"

// emailContentSecurityPolicyMeta is the policy as the meta element both frames put in their document head.
export const emailContentSecurityPolicyMeta =
    `<meta http-equiv="Content-Security-Policy" content="${EMAIL_CONTENT_SECURITY_POLICY}">`
