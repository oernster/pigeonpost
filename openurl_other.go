//go:build !windows

package main

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// openExternalURL opens an already scheme-vetted URL in the user's default browser. On non-Windows
// platforms the Wails runtime call is used as is; the Windows build replaces it with a direct
// ShellExecute because Wails' validator there rejects legal URL characters (see openurl_windows.go).
func (a *App) openExternalURL(u string) {
	runtime.BrowserOpenURL(a.ctx, u)
}
