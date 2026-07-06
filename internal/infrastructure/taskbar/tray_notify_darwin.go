//go:build darwin

package taskbar

import (
	"os/exec"
	"strings"
)

// Notify raises a macOS notification via osascript, the route that avoids a CGO bridge to the
// UserNotifications framework. The strings are escaped as AppleScript string literals and the command
// runs without a shell, so the reminder text cannot break out of the script. Any failure is ignored so
// a missing notification never disturbs the reminder scheduler.
func (t *Tray) Notify(title, body string, _ bool) {
	if title == "" && body == "" {
		return
	}
	script := "display notification " + appleScriptString(body) + " with title " + appleScriptString(title)
	_ = exec.Command("osascript", "-e", script).Run()
}

// appleScriptString quotes and escapes a Go string as an AppleScript string literal.
func appleScriptString(s string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
	return `"` + escaped + `"`
}
