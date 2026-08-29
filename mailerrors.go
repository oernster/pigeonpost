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

// errIMAPDisabled is the message shown when the server accepts the sign-in and then refuses the session,
// which is what a mailbox with IMAP switched off does. It names the setting and the steps, because the
// person reading it has just signed in successfully and has no reason to suspect a switch they have
// never seen. Microsoft ships personal accounts this way, so this is the first thing a new Outlook.com
// or Hotmail user meets.
//
// It says the mailbox is refusing IMAP rather than that the switch is off, because the switch being off
// is an inference: what was actually observed is a refusal, matched on the server's own words. Stating
// the observation keeps the message true even where the cause turns out to be another one.
//
// It names BOTH headings the section goes by. Microsoft's documentation says "Forwarding and IMAP" while
// the current web interface calls it "Sync email"; a message that names only one sends half its readers
// hunting for a heading their Outlook does not have. The gear is named too, since the interface labels
// it with nothing.
//
// It carries the revert warning and nothing else beyond the route, because a mailbox only a day or two
// old accepts the switch, saves it, then quietly puts it back: Microsoft holds IMAP down on new accounts
// while they build reputation; the only reported cure is time. Told merely to turn the switch on,
// the reader does exactly that, watches it save, then meets this message again with nothing left to try.
//
// It is deliberately SHORT. An earlier draft spelled out every obstacle and ran to ninety words, which
// nobody reads: an error nobody finishes is worth less than a shorter one they act on. The remaining
// detail lives in the README and behind the wizard's help link, not in a red box.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errIMAPDisabled = errors.New(
	"Microsoft is refusing IMAP for this mailbox. Turn it on at outlook.com under Settings, Mail, " +
		"then \"Sync email\" or \"Forwarding and IMAP\". On a new account it often reverts after " +
		"saving, so check it stayed on.")

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
