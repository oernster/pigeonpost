//go:build !windows && !darwin && !linux

package main

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// openExternalURL opens an already scheme-vetted URL in the user's default browser. On the remaining
// Unix platforms the Wails runtime call is used as is; the Windows, macOS and Linux builds replace it
// with a direct OS launch because Wails' URL validator rejects legal URL characters (see
// openurl_windows.go, openurl_darwin.go and openurl_linux.go).
func (a *App) openExternalURL(u string) {
	runtime.BrowserOpenURL(a.ctx, u)
}
