package main

import (
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	appAuthor    = "Oliver Ernster"
	appCopyright = "© 2026 Oliver Ernster"
	appTagline   = "A calmer, local-first cross-platform email client."
	releasesURL  = "https://github.com/oernster/pigeonpost/releases"
)

// CreditDTO names one open-source dependency and its licence for the About dialog.
type CreditDTO struct {
	Name    string `json:"name"`
	Licence string `json:"licence"`
}

// AboutDTO is the data shown in Help > About. The icon itself is a bundled front-end asset.
type AboutDTO struct {
	Name      string      `json:"name"`
	Tagline   string      `json:"tagline"`
	Version   string      `json:"version"`
	Author    string      `json:"author"`
	Copyright string      `json:"copyright"`
	Licence   string      `json:"licence"`
	Credits   []CreditDTO `json:"credits"`
}

// About returns the application metadata and open-source credits for the About dialog.
func (a *App) About() AboutDTO {
	return AboutDTO{
		Name:      appName,
		Tagline:   appTagline,
		Version:   version(),
		Author:    appAuthor,
		Copyright: appCopyright,
		Licence:   "GPL-3.0",
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
// Other schemes are ignored so a message cannot drive the app to arbitrary URI handlers.
func (a *App) OpenExternal(url string) {
	u := strings.TrimSpace(url)
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "mailto:") {
		runtime.BrowserOpenURL(a.ctx, u)
	}
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
