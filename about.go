package main

import (
	"log"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	appAuthor    = "Oliver Ernster"
	appCopyright = "© 2026 Oliver Ernster"
	appTagline   = "A calmer, local-first cross-platform email client."
	releasesURL  = "https://github.com/oernster/pigeonpost/releases"
	// appAttribution states the licence's section 7(b) additional term, shown in Help > About so
	// the attribution requirement travels with every copy of the app.
	appAttribution = "Credit to the original author is preserved in all copies and derivative works," +
		" as the licence requires (GPLv3 section 7(b))."
)

// CreditDTO names one open-source dependency and its licence for the About dialog.
type CreditDTO struct {
	Name    string `json:"name"`
	Licence string `json:"licence"`
}

// AboutDTO is the data shown in Help > About. The icon itself is a bundled front-end asset.
type AboutDTO struct {
	Name        string      `json:"name"`
	Tagline     string      `json:"tagline"`
	Version     string      `json:"version"`
	Author      string      `json:"author"`
	Copyright   string      `json:"copyright"`
	Licence     string      `json:"licence"`
	Attribution string      `json:"attribution"`
	Credits     []CreditDTO `json:"credits"`
}

// About returns the application metadata and open-source credits for the About dialog.
func (a *App) About() AboutDTO {
	return AboutDTO{
		Name:        appName,
		Tagline:     appTagline,
		Version:     version(),
		Author:      appAuthor,
		Copyright:   appCopyright,
		Licence:     "GPL-3.0",
		Attribution: appAttribution,
		Credits: []CreditDTO{
			{Name: "Go", Licence: "BSD-3-Clause"},
			{Name: "Wails", Licence: "MIT"},
			{Name: "React", Licence: "MIT"},
			{Name: "Vite", Licence: "MIT"},
			{Name: "TipTap", Licence: "MIT"},
			{Name: "google/uuid", Licence: "BSD-3-Clause"},
			{Name: "emersion go-imap", Licence: "MIT"},
			{Name: "emersion go-smtp", Licence: "MIT"},
			{Name: "emersion go-message", Licence: "MIT"},
			{Name: "emersion go-ical", Licence: "MIT"},
			{Name: "microcosm-cc/bluemonday", Licence: "BSD-3-Clause"},
			{Name: "emersion go-sasl", Licence: "MIT"},
			{Name: "modernc.org/sqlite", Licence: "BSD-3-Clause"},
			{Name: "zalando/go-keyring", Licence: "MIT"},
			{Name: "teambition/rrule-go", Licence: "MIT"},
		},
	}
}

// OpenReleasesPage opens the GitHub releases page in the user's default browser.
func (a *App) OpenReleasesPage() {
	runtime.BrowserOpenURL(a.ctx, releasesURL)
}

// OpenExternal opens an http(s) or mailto link from message content in the user's default browser.
// Other schemes are ignored so a message cannot drive the app to arbitrary URI handlers. The launch
// goes through openExternalURL rather than runtime.BrowserOpenURL: Wails' Windows implementation
// rejects any URL containing characters it deems shell metacharacters, a set that includes tilde,
// exclamation mark, asterisk and parentheses, all legal URL characters (RFC 3986 unreserved and
// sub-delims) that click-tracking links in real email routinely carry. Those links then failed
// silently, the click apparently doing nothing. The scheme allowlist above is the security boundary
// here; openExternalURL passes the URL to the OS without a shell, so no metacharacter can execute.
// The stdlib log lines below are the diagnostic for a silently dead link click: run the app from a
// terminal and a click either logs here (the frontend chain works, so any failure is in the OS
// launch) or logs nothing (the click never left the frontend). Wails' own info logging is suppressed
// in production builds, so stdlib log to stderr is the channel that is always visible.
func (a *App) OpenExternal(url string) {
	u := strings.TrimSpace(url)
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "mailto:") {
		log.Printf("open-external: launching %q", u)
		a.openExternalURL(u)
		return
	}
	log.Printf("open-external: ignoring URL with unsupported scheme %q", u)
}

// LicenceText returns the full GPL-3.0 licence text bundled with the application.
func (a *App) LicenceText() string {
	return licenceText
}

// Version returns the application version from the VERSION file, for the splash screen.
func (a *App) Version() string {
	return version()
}

// Author returns the application author, for the splash screen.
func (a *App) Author() string {
	return appAuthor
}
