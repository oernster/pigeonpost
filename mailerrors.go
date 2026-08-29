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
// user meets.
//
// It names BOTH headings the section goes by. Microsoft's documentation says "Forwarding and IMAP" while
// the current web interface calls it "Sync email"; a message that names only one sends half its readers
// hunting for a heading their Outlook does not have. The gear is named too, since the interface labels
// it with nothing.
//
// It also names the two states in which the switch cannot be used, because on a new mailbox neither is
// obvious and each looks like the instructions being wrong. An unverified account is shown a Sign in
// button in place of the switches, so the account has to be verified before the setting exists to turn
// on. Microsoft also holds IMAP back on a freshly created mailbox, so the switch can refuse to stay on
// for a day or so afterwards.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errIMAPDisabled = errors.New(
	"Signed in successfully; IMAP is switched off for this mailbox, which is how Microsoft ships a " +
		"new Outlook.com or Hotmail account. Turn it on at outlook.com: open Settings (the gear, top " +
		"right), then Mail, then either \"Sync email\" or \"Forwarding and IMAP\" depending on which " +
		"Outlook you have. Switch on \"Let devices and apps use IMAP\", save, then add the account " +
		"again. If that page offers a Sign in button instead of the switches, the account is not " +
		"verified yet: add and confirm a phone number under your Microsoft account security info " +
		"first. On a mailbox created in the last day or two the switch can also refuse to stay on " +
		"until Microsoft has finished preparing it.")

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
