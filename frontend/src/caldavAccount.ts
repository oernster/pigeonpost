// caldavAccount holds the pure add-account form logic for a remote CalDAV account: the form shape, an
// empty-form factory and the client-side validation that mirrors the Go domain's CalendarAccount rules (a
// non-empty display name, an http(s) base URL and a username) plus a required password, so the form reports
// a problem before the backend round trip. No React, no api, so it is unit-tested in isolation.

// CalDAVAccountForm is the add-account form's state: the four fields the user fills to configure a DAV
// account. The password is submitted to the backend (which stores it in the OS keychain) and never kept
// beyond the add.
export interface CalDAVAccountForm {
    displayName: string
    baseUrl: string
    username: string
    password: string
}

// emptyCalDAVAccountForm is a blank form, used to open the add-account fields and to reset them after a save.
export function emptyCalDAVAccountForm(): CalDAVAccountForm {
    return {displayName: '', baseUrl: '', username: '', password: ''}
}

// BASE_URL_SCHEME matches the http:// or https:// prefix the server address must carry. It is the client
// echo of the domain's NewCalendarAccount check, so a bad address is caught before the backend call.
const BASE_URL_SCHEME = /^https?:\/\//i

// validateCalDAVAccountForm returns a human-readable problem with the form or an empty string when the form
// is ready to submit. The checks and their order mirror the domain's NewCalendarAccount so the client and
// the backend agree on what a valid account is; the password check is additional, as the domain holds no
// password (it lives in the keychain). The password is not trimmed, since an app password may legitimately
// contain spaces.
export function validateCalDAVAccountForm(form: CalDAVAccountForm): string {
    if (form.displayName.trim() === '') {
        return 'Enter a name for this account.'
    }
    const url = form.baseUrl.trim()
    if (url === '') {
        return 'Enter the server address.'
    }
    if (!BASE_URL_SCHEME.test(url)) {
        return 'The server address must start with http:// or https://.'
    }
    if (form.username.trim() === '') {
        return 'Enter the username.'
    }
    if (form.password === '') {
        return 'Enter the password.'
    }
    return ''
}
