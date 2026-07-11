// accountProviders holds the pure account-setup data and helpers: the generic endpoint defaults, the
// protocol and security option lists, the known-provider presets and the small host, domain and url
// helpers. No React, no api, so the helpers are unit-tested in isolation.

// normaliseSigUrl gives a bare host a scheme so a signature link is absolute, matching the composer.
export function normaliseSigUrl(url: string): string {
    const trimmed = url.trim()
    if (trimmed === '' || /^(https?:|mailto:)/i.test(trimmed)) {
        return trimmed
    }
    return `https://${trimmed}`
}

// Default endpoints for a generic IMAP + submission setup: implicit TLS on 993 for retrieval and
// STARTTLS on 587 for sending. The user can override every field.
export const DEFAULT_IN_PORT = 993
export const DEFAULT_OUT_PORT = 587
export const DEFAULT_IN_SECURITY = 'tls'
export const DEFAULT_OUT_SECURITY = 'starttls'
// POP3 retrieval uses implicit TLS on 995 rather than IMAP's 993.
export const DEFAULT_IN_PORT_POP3 = 995

export const PROTOCOL_OPTIONS: ReadonlyArray<{value: string; label: string}> = [
    {value: 'imap', label: 'IMAP (keeps mail on the server)'},
    {value: 'pop3', label: 'POP3 (downloads mail, single inbox)'},
]

// incomingHostPrefix is the conventional hostname prefix guessed for each retrieval protocol.
export function incomingHostPrefix(protocol: string): string {
    return protocol === 'pop3' ? 'pop' : 'imap'
}

export const SECURITY_OPTIONS: ReadonlyArray<{value: string; label: string}> = [
    {value: 'tls', label: 'TLS (implicit)'},
    {value: 'starttls', label: 'STARTTLS'},
    {value: 'none', label: 'None (plaintext)'},
]

// Provider is a known email service with its server settings pre-filled. Choosing one skips manual
// server entry; the account itself is still added through the same AddAccount path.
export interface Provider {
    id: string
    name: string
    note: string
    inHost: string
    inPort: number
    inSecurity: string
    outHost: string
    outPort: number
    outSecurity: string
}

export const PROVIDERS: readonly Provider[] = [
    {
        id: 'gmail', name: 'Gmail',
        inHost: 'imap.gmail.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.gmail.com', outPort: 587, outSecurity: 'starttls',
        note: 'Personal Gmail only. Turn on 2-Step Verification, then generate an app password and use it here.',
    },
    {
        id: 'icloud', name: 'iCloud Mail',
        inHost: 'imap.mail.me.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.mail.me.com', outPort: 587, outSecurity: 'starttls',
        note: 'Create an app-specific password in your Apple ID settings and use it here.',
    },
    {
        id: 'yahoo', name: 'Yahoo Mail',
        inHost: 'imap.mail.yahoo.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.mail.yahoo.com', outPort: 465, outSecurity: 'tls',
        note: 'Yahoo requires an app password generated in your account security settings.',
    },
    {
        id: 'fastmail', name: 'Fastmail',
        inHost: 'imap.fastmail.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.fastmail.com', outPort: 465, outSecurity: 'tls',
        note: 'Create an app password in Fastmail settings and use it here.',
    },
    {
        id: 'startmail', name: 'StartMail',
        inHost: 'imap.startmail.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.startmail.com', outPort: 587, outSecurity: 'starttls',
        note: 'Enable IMAP/SMTP under Settings in StartMail, then use your app-specific password.',
    },
    {
        id: 'zoho', name: 'Zoho Mail',
        inHost: 'imap.zoho.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.zoho.com', outPort: 465, outSecurity: 'tls',
        note: 'Turn on IMAP under Settings in Zoho Mail, then generate an app-specific password (Security, App Passwords) if two-factor is on and use it here. If your Zoho account is on a non-US data centre change the servers to the matching region (for example imap.zoho.eu and smtp.zoho.eu).',
    },
]

export function domainOf(email: string): string {
    const at = email.indexOf('@')
    return at >= 0 ? email.slice(at + 1).trim() : ''
}
