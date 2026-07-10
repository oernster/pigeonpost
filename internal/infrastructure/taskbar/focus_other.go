//go:build !windows

package taskbar

// FocusMainWindow is a no-op off Windows; the WebView keyboard-focus-on-launch issue is Windows-specific.
func FocusMainWindow(string) {}
