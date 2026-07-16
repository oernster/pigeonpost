//go:build windows

package main

import (
	"os"

	"github.com/oernster/pigeonpost/internal/installer"
)

// registerMailtoHandler (re)writes PigeonPost's mailto: protocol registration for the current user, so
// the app can self-heal the registration before sending the user to the Default apps page. The installer
// normally writes it at install time; running from source or a moved executable still works because the
// registered command points at this executable.
func registerMailtoHandler() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return installer.RegisterMailtoProtocol(exe, exe)
}
