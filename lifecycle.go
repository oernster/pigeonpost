package main

import (
	"context"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/infrastructure/taskbar"
)

// startup captures the Wails runtime context, starts the tray (its menu items emit Wails events the
// front end turns into the matching dialogs, so this layer owns the callbacks and the taskbar package
// stays free of Wails), then starts the reminder scheduler and the new-mail notifier.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// A double-clicked .eml or a clicked mailto: link (PigeonPost being the registered handler) launches the
	// app with the path or URI as an argument; capture it now and open it from domReady, once the front end
	// is mounted to receive it. On macOS the same payloads arrive through onURLOpen and onFileOpen instead,
	// which park in the same slots when they beat the front end.
	a.pendingMu.Lock()
	a.pendingEmail = firstEmailFileArg(os.Args)
	a.pendingMailto = firstMailtoArg(os.Args)
	a.pendingMu.Unlock()
	if a.tray != nil {
		a.tray.Start(taskbar.TrayActions{
			// Open must go through the Wails runtime, not a Win32 window search: when the window is hidden
			// to the tray it is no longer a findable visible window.
			Open:         func() { a.revealWindow() },
			About:        func() { runtime.EventsEmit(ctx, "menu:about") },
			Licence:      func() { runtime.EventsEmit(ctx, "menu:licence") },
			CheckUpdates: func() { runtime.EventsEmit(ctx, "menu:check-updates") },
			Quit:         func() { a.quit() },
		})
	}
	go a.runReminderScheduler()
	go a.runMailNotifier()
	go a.runOutboxDispatcher()
	go a.runSnoozeScheduler()
}

// focusOnLaunchDelay lets the window finish showing before the WebView is given keyboard focus, so the
// focus change lands on a settled window.
const focusOnLaunchDelay = 250 * time.Millisecond

// domReady runs once the frontend has loaded. On Windows the WebView2 control does not take keyboard focus
// when the window first appears, so a cold launch drops every keystroke (including the first Tab) until the
// user clicks in the window. Give the WebView its keyboard focus here so the keyboard works from launch.
func (a *App) domReady(_ context.Context) {
	go func() {
		time.Sleep(focusOnLaunchDelay)
		taskbar.FocusMainWindow(a.title)
		// Flush a .eml or mailto: captured at cold launch now the front end is up; the focus delay above
		// also gives the event listeners time to attach before the viewer or composer is asked to open.
		// frontendReady flips inside the same lock, so a macOS open event either lands in the pending
		// slots read here or opens directly, never neither.
		a.pendingMu.Lock()
		a.frontendReady = true
		email, mailto := a.pendingEmail, a.pendingMailto
		a.pendingEmail, a.pendingMailto = "", ""
		a.pendingMu.Unlock()
		if email != "" {
			a.openEmailFile(email)
		}
		if mailto != "" {
			a.openMailto(mailto)
		}
	}()
}
