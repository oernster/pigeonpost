package main

import (
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// emlArgExtension is the extension of an email file PigeonPost opens when the OS launches it as the
	// registered .eml handler, for example from a double-click in Explorer.
	emlArgExtension = ".eml"
	// defaultAppsSettingsURL opens the Windows Settings "Default apps" page at PigeonPost's entry, the
	// supported one-click route for the user to make PigeonPost the default .eml handler. The
	// registeredAppUser query targets this app on Windows 11; where it is not honoured, the Default apps
	// page still opens.
	defaultAppsSettingsURL = "ms-settings:defaultapps?registeredAppUser=" + appName
)

// firstEmailFileArg returns the first .eml path among launch arguments or "" when there is none. It scans
// every argument (argv[0], the executable, ends in .exe so it never matches), so it is safe whether or not
// the runtime includes the program name in the slice.
func firstEmailFileArg(args []string) string {
	for _, arg := range args {
		if strings.EqualFold(filepath.Ext(arg), emlArgExtension) {
			return arg
		}
	}
	return ""
}

// openEmailFile parses a .eml file from disk and asks the front end to show it in the in-app viewer. It
// backs a double-click on a .eml once PigeonPost is the registered handler, whether the app was already
// running or was cold-started for it. A read or parse failure is reported to the front end so the user is
// told, rather than met with silence.
func (a *App) openEmailFile(path string) {
	view, err := emailFileView(path)
	if err != nil {
		runtime.EventsEmit(a.ctx, "eml:open-error", err.Error())
		return
	}
	runtime.EventsEmit(a.ctx, "eml:open", view)
}

// ShowDefaultAppSettings opens the Windows Default apps settings so the user can make PigeonPost the default
// for .eml files. Windows does not let an application set itself as the default silently, so this is the
// supported route; it is offered from the Mail menu on Windows. Bound to the front end.
func (a *App) ShowDefaultAppSettings() {
	runtime.BrowserOpenURL(a.ctx, defaultAppsSettingsURL)
}
