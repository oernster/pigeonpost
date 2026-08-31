//go:build darwin

package taskbar

import (
	"os/exec"
	"strings"

	"github.com/oernster/pigeonpost/internal/infrastructure/sound"
)

// Notify raises a macOS notification via osascript, the route that avoids a CGO bridge to the
// UserNotifications framework. The strings are escaped as AppleScript string literals and the command
// runs without a shell, so the reminder text cannot break out of the script. Any failure is ignored so
// a missing notification never disturbs the reminder scheduler. The chime kind is ignored here: macOS
// sounds a notification from the user's own notification settings, which PigeonPost has no business
// overriding.
func (t *Tray) Notify(title, body string, _ bool, _ sound.Kind) {
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
