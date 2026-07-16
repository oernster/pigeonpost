//go:build !windows

package main

// registerMailtoHandler is a no-op away from Windows: default-mail-client registration is a per-platform
// concern handled when the macOS and Linux builds take it on. The menu item that reaches it is
// Windows-only, so this exists to keep cross-platform builds compiling.
func registerMailtoHandler() error { return nil }
