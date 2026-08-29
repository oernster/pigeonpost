package main

// mailerrors.go is the single point where a mail error crossing from Go into the user interface is made
// fit to read. Infrastructure adapters mark a failure with a domain sentinel; the technical detail that
// carries (the host and port, "dial", the server's own tagged response) is meaningless to a user, so the
// Wails facade runs every mail error through friendlyMailError before returning it.

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

// errIMAPDisabled is the message shown when the server accepts the sign-in and then refuses the session
// because IMAP is switched off for the mailbox. It names the setting and the steps, because the person
// reading it has just signed in successfully and has no reason to suspect a switch they have never seen.
// Microsoft ships personal accounts this way, so this is the first thing a new Outlook.com or Hotmail
// user meets; the words are theirs, from the page that documents the default.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errIMAPDisabled = errors.New(
	"Signed in successfully; IMAP is switched off for this mailbox, which is how Microsoft ships a " +
		"new Outlook.com or Hotmail account. Turn it on at outlook.com under Settings, Mail, Forwarding " +
		"and IMAP: switch on \"Let devices and apps use IMAP\", save, then add the account again.")

// isOffline reports whether err was caused by the mail server being unreachable (domain.ErrOffline
// wrapped anywhere in the chain), as opposed to the server rejecting a well-formed request.
func isOffline(err error) bool {
	return err != nil && errors.Is(err, domain.ErrOffline)
}

// friendlyMailError converts an internal mail error into one fit to show the user: a connectivity
// failure becomes the plain offline message and a mailbox with IMAP switched off becomes the message
// naming that setting, while every other error is returned unchanged so a genuine fault still surfaces
// its detail. A nil error stays nil.
func friendlyMailError(err error) error {
	if isOffline(err) {
		return errOffline
	}
	if err != nil && errors.Is(err, domain.ErrIMAPDisabled) {
		return errIMAPDisabled
	}
	return err
}
