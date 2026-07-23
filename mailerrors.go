package main

// mailerrors.go is the single point where a mail error crossing from Go into the user interface is made
// fit to read. Infrastructure adapters wrap a connectivity failure with domain.ErrOffline; the technical
// detail that carries (the host and port, "dial", "unreachable") is meaningless to a user, so the Wails
// facade runs every mail error through friendlyMailError before returning it.

import (
	"errors"

	"github.com/oernster/pigeonpost/internal/domain"
)

// errOffline is the message shown when a mail action fails because the server could not be reached. It
// is returned verbatim by the Wails facade and rendered as-is in the interface, so a capitalised,
// punctuated sentence is intended here.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errOffline = errors.New("Can't reach the mail server. You may be offline; check your internet connection and try again.")

// isOffline reports whether err was caused by the mail server being unreachable (domain.ErrOffline
// wrapped anywhere in the chain), as opposed to the server rejecting a well-formed request.
func isOffline(err error) bool {
	return err != nil && errors.Is(err, domain.ErrOffline)
}

// friendlyMailError converts an internal mail error into one fit to show the user: a connectivity
// failure becomes the plain offline message, while every other error is returned unchanged so a genuine
// fault still surfaces its detail. A nil error stays nil.
func friendlyMailError(err error) error {
	if isOffline(err) {
		return errOffline
	}
	return err
}
