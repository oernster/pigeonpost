//go:build linux

package main

import (
	"os/exec"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// openExternalURL opens an already scheme-vetted URL in the user's default browser via xdg-open. It
// deliberately does not go straight to runtime.BrowserOpenURL: Wails validates the URL against a
// shell-metacharacter blocklist that rejects legal URL characters (tilde, exclamation mark, asterisk,
// parentheses and others, all RFC 3986 unreserved or sub-delims), silently dropping the
// click-tracking links bulk-mail senders wrap around every button. xdg-open receives the URL as a
// single argv entry with no shell involved, so those characters are inert; the caller's scheme
// allowlist (http, https, mailto) is what prevents a message driving the app to a dangerous URI
// handler. On any failure it falls back to the Wails runtime call, which at worst re-applies the
// stricter filter.
func (a *App) openExternalURL(u string) {
	if err := exec.Command("xdg-open", u).Start(); err != nil {
		runtime.BrowserOpenURL(a.ctx, u)
	}
}
