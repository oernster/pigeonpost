//go:build windows

package main

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

// openExternalURL opens an already scheme-vetted URL in the user's default browser via ShellExecuteW.
// It deliberately does not use runtime.BrowserOpenURL: Wails validates the URL against a
// shell-metacharacter blocklist that rejects legal URL characters (tilde, exclamation mark, asterisk,
// parentheses and others), silently dropping the click-tracking links bulk-mail senders wrap around
// every button. ShellExecute takes the URL as a single argument straight to the protocol handler, no
// shell is involved, so those characters are inert; the caller's scheme allowlist (http, https,
// mailto) is what prevents a message driving the app to a dangerous URI handler. On any failure it
// falls back to the Wails runtime call, which at worst re-applies the stricter filter.
func (a *App) openExternalURL(u string) {
	verb, verbErr := windows.UTF16PtrFromString("open")
	target, targetErr := windows.UTF16PtrFromString(u)
	if verbErr == nil && targetErr == nil {
		if err := windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL); err == nil {
			return
		}
	}
	runtime.BrowserOpenURL(a.ctx, u)
}
